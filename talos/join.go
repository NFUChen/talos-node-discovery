package talos

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/thedevsaddam/unpack"
)

// Member represents a Talos cluster member
type Member struct {
	Node        string   `json:"node"`
	Namespace   string   `json:"namespace"`
	Type        string   `json:"type"`
	ID          string   `json:"id"`
	Version     string   `json:"version"`
	Hostname    string   `json:"hostname"`
	MachineType string   `json:"machine_type"`
	OS          string   `json:"os"`
	Addresses   []string `json:"addresses"`
	InternalIP  string   `json:"internal_ip"`
}

func (member *Member) IsControlPlane() bool {
	return member.MachineType == "controlplane"
}

func (member *Member) IsWorker() bool {
	return member.MachineType == "worker"
}

// GetMembers fetches and parses Talos cluster members using talosctl
// talosConfigPath: path to talosconfig file (e.g., "./talosconfig")
// timeout: command execution timeout
func GetMembers(ctx context.Context, talosConfigPath string, timeout time.Duration) ([]Member, error) {
	// Create context with timeout
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Build talosctl command
	cmd := exec.CommandContext(cmdCtx, "talosctl", "get", "members", "-o", "json")

	// Set environment with TALOSCONFIG - inherit parent environment first
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, fmt.Sprintf("TALOSCONFIG=%s", talosConfigPath))

	// Execute command and capture output
	output, err := cmd.CombinedOutput()
	if err != nil {
		if cmdCtx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("command timed out after %v", timeout)
		}
		return nil, fmt.Errorf("failed to execute talosctl: %w, output: %s", err, string(output))
	}

	// Parse JSON output
	members, err := parseMembers(output)
	if err != nil {
		return nil, fmt.Errorf("failed to parse members: %w", err)
	}

	return members, nil
}

// parseMembers parses the JSON output from talosctl get members
func parseMembers(data []byte) ([]Member, error) {
	type MemberResponse struct {
		Metadata struct {
			ID        string `json:"id"`
			Namespace string `json:"namespace"`
			Type      string `json:"type"`
			Version   int    `json:"version"`
		} `json:"metadata"`
		Node string `json:"node"`
		Spec struct {
			NodeID          string   `json:"nodeId"`
			Addresses       []string `json:"addresses"`
			Hostname        string   `json:"hostname"`
			MachineType     string   `json:"machineType"`
			OperatingSystem string   `json:"operatingSystem"`
		} `json:"spec"`
	}

	// The output is multiple JSON objects separated by newlines
	// We need to parse each complete JSON object
	dataStr := strings.TrimSpace(string(data))
	var members []Member

	// Use a JSON decoder to handle multiple JSON objects
	decoder := json.NewDecoder(strings.NewReader(dataStr))

	for decoder.More() {
		var result MemberResponse
		if err := decoder.Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to parse JSON object: %w", err)
		}

		member := Member{
			Node:        result.Node,
			Namespace:   result.Metadata.Namespace,
			Type:        result.Metadata.Type,
			ID:          result.Metadata.ID,
			Version:     fmt.Sprintf("%d", result.Metadata.Version),
			Hostname:    result.Spec.Hostname,
			MachineType: result.Spec.MachineType,
			OS:          result.Spec.OperatingSystem,
			Addresses:   result.Spec.Addresses,
			InternalIP:  extractInternalIP(result.Spec.Addresses),
		}

		members = append(members, member)
	}

	return members, nil
}

// extractInternalIP extracts the first IPv4 address from the addresses list
func extractInternalIP(addresses []string) string {
	for _, addr := range addresses {
		ip := net.ParseIP(addr)
		if ip != nil && ip.To4() != nil {
			return addr
		}
	}
	// If no IPv4 found, return first address or empty string
	if len(addresses) > 0 {
		return addresses[0]
	}
	return ""
}

// parseMembersTable parses the table format output from talosctl get members
func parseMembersTable(data string) ([]Member, error) {
	lines := strings.Split(strings.TrimSpace(data), "\n")
	if len(lines) < 2 {
		return nil, fmt.Errorf("invalid output: expected at least 2 lines")
	}

	// Skip header line
	var members []Member
	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		// Parse each field (space-separated, but addresses are in JSON array format)
		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}

		// Extract addresses from JSON array format
		addressesStart := strings.Index(line, "[")
		addressesEnd := strings.Index(line, "]")
		var addresses []string
		if addressesStart != -1 && addressesEnd != -1 {
			addressesStr := line[addressesStart+1 : addressesEnd]
			addressesParts := strings.Split(addressesStr, ",")
			for _, addr := range addressesParts {
				addresses = append(addresses, strings.Trim(addr, `" `))
			}
		}

		var node, namespace, _type, id, version, hostname, machineType, os string
		unpack.Do(fields, &node, &namespace, &_type, &id, &version, &hostname, &machineType, &os)

		member := Member{
			Node:        fields[0],
			Namespace:   namespace,
			Type:        _type,
			ID:          id,
			Version:     version,
			Hostname:    hostname,
			MachineType: machineType,
			OS:          os,
			Addresses:   addresses,
			InternalIP:  extractInternalIP(addresses),
		}

		members = append(members, member)
	}

	return members, nil
}

func WorkerJoin(ctx context.Context, ip string, workerConfigPath string, talosConfigPath string, timeout time.Duration) error {
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)

	defer cancel()
	cmd := exec.CommandContext(cmdCtx, "talosctl", "apply-config", "--insecure", "--nodes", ip, "--file", workerConfigPath)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, fmt.Sprintf("TALOSCONFIG=%s", talosConfigPath))

	output, err := cmd.CombinedOutput()
	if err != nil {
		if cmdCtx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("command timed out after %v", timeout)
		}
		return fmt.Errorf("failed to join worker: %w, output: %s", err, string(output))
	}

	return nil
}
