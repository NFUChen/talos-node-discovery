# Talos Probe - Usage Guide

## Overview

This tool provides functionality to scan for Talos hosts, fetch cluster member information, and automatically join new worker nodes to a Talos cluster.

## Features

1. **CIDR Scanning**: Scan a network range for Talos hosts (port 50000)
2. **Member Information**: Fetch and parse Talos cluster member details using `talosctl`
3. **Auto-Join Workers**: Automatically detect and join new worker nodes to the cluster

## Usage

### Running the Program

```bash
# Scan a CIDR range, fetch members, and join new workers
go run main.go <cidr> <talosconfig-path> <worker-config-path>

# Example:
go run main.go 10.3.5.1/24 ./talosconfig ./worker.yaml
```

### What the Program Does

1. **Scans the CIDR range** for hosts listening on port 50000 (Talos API port)
2. **Fetches existing cluster members** using `talosctl get members`
3. **Identifies new nodes** by comparing scanned hosts with existing members
4. **Automatically joins new worker nodes** using `talosctl apply-config --insecure`

### Using the Talos Package

The `talos` package provides a `GetMembers` function to fetch cluster member information:

```go
package main

import (
    "context"
    "fmt"
    "log"
    "talos-probe/talos"
    "time"
)

func main() {
    ctx := context.Background()
    talosConfigPath := "./talosconfig"
    timeout := 10 * time.Second

    members, err := talos.GetMembers(ctx, talosConfigPath, timeout)
    if err != nil {
        log.Fatalf("Failed to get members: %v", err)
    }

    for _, member := range members {
        log.Printf("Member: %s (%s)\n", member.Hostname, member.MachineType)
        log.Printf("  Addresses: %v\n", member.Addresses)
    }
}
```

## Member Structure

The `Member` struct contains the following fields:

```go
type Member struct {
    Node        string   // Node IP
    Namespace   string   // Kubernetes namespace
    Type        string   // Resource type
    ID          string   // Member ID
    Version     string   // Version number
    Hostname    string   // Hostname
    MachineType string   // "worker" or "controlplane"
    OS          string   // Operating system info
    Addresses   []string // IP addresses (IPv4 and IPv6)
    InternalIP  string   // First IPv4 address (extracted from Addresses)
}
```

### Helper Methods

```go
// Check if member is a control plane node
member.IsControlPlane() bool

// Check if member is a worker node
member.IsWorker() bool
```

## Example Output

```
cidr: 10.3.5.1/24
talosConfigPath: ./talosconfig
workerConfigPath: ./worker.yaml
Total IPs to scan: 254
dialing talos hosts...
connected to address 10.3.5.10 successfully
connected to address 10.3.5.20 successfully
connected to address 10.3.5.30 successfully
talos hosts: [10.3.5.10 10.3.5.20 10.3.5.30]

=== Fetching Talos Cluster Members ===

Found 2 cluster members:

Member 1:
  Node:         43.213.7.87
  ID:           ip-10-1-17-105
  Hostname:     ip-10-1-17-105.ap-east-2.compute.internal
  Machine Type: worker
  OS:           Talos (v1.11.3)
  Internal IP:  10.3.5.10
  All Addresses: [10.3.5.10 fdca:82ba:83b5:8302:8ba:87ff:fe69:f43a]

Member 2:
  Node:         43.213.7.87
  ID:           ip-10-1-26-182
  Hostname:     ip-10-1-26-182.ap-east-2.compute.internal
  Machine Type: controlplane
  OS:           Talos (v1.11.3)
  Internal IP:  10.3.5.20
  All Addresses: [10.3.5.20 fdca:82ba:83b5:8302:80c:2fff:fe58:7606]

new member not joined ips[1]: [10.3.5.30]
joining worker: 10.3.5.30
```

## How It Works

### 1. Network Scanning (probe package)
- **IpsFromCIDR**: Generates all IPs from a CIDR range (excludes network/broadcast for IPv4)
- **DialAddress**: Tests TCP connectivity to port 50000 with a 3-second timeout
- Concurrent scanning using goroutines for speed

### 2. Member Discovery (talos package)
- **GetMembers**: Executes `talosctl get members -o json` with configurable timeout
- **JSON Parsing**: Uses a JSON decoder to parse multiple JSON objects from the output
- **InternalIP Extraction**: Automatically extracts the first IPv4 address from the addresses list
- **Environment Setup**: Inherits parent process environment and adds `TALOSCONFIG` path

### 3. New Node Detection (main.go)
- Compares scanned Talos hosts with existing cluster members
- Matches by comparing InternalIP field
- Filters out IPv6 addresses and duplicates
- Identifies nodes that need to be joined

### 4. Worker Join (talos package)
- **WorkerJoin**: Executes `talosctl apply-config --insecure --nodes <ip> --file <worker-config>`
- Uses `--insecure` flag for initial bootstrap (no existing trust)
- Applies worker configuration to new nodes

## Error Handling

The tool handles several error cases:
- **Network errors**: Failed TCP connections, timeouts
- **Command timeout**: Configurable timeout for `talosctl` commands
- **Invalid talosconfig**: Missing or invalid configuration file
- **JSON parsing errors**: Malformed output from `talosctl`
- **Command execution failures**: `talosctl` not found or execution errors

## Advanced Usage

### Using as a Library

#### Scan Network for Talos Hosts

```go
import "talos-probe/probe"

// Get all IPs from CIDR
ips, err := probe.IpsFromCIDR("10.3.5.0/24")

// Test connectivity
for _, ip := range ips {
    err := probe.DialAddress(fmt.Sprintf("%s:50000", ip), 3*time.Second)
    if err == nil {
        log.Printf("Found Talos host: %s\n", ip)
    }
}
```

#### Filter by Machine Type

```go
members, _ := talos.GetMembers(ctx, "./talosconfig", 10*time.Second)

// Get only workers using helper method
var workers []talos.Member
for _, m := range members {
    if m.IsWorker() {
        workers = append(workers, m)
    }
}

// Get only control plane nodes using helper method
var controlPlanes []talos.Member
for _, m := range members {
    if m.IsControlPlane() {
        controlPlanes = append(controlPlanes, m)
    }
}
```

#### Extract IPv4 Addresses Only

```go
members, _ := talos.GetMembers(ctx, "./talosconfig", 10*time.Second)

for _, member := range members {
    // Use the InternalIP field (first IPv4)
    log.Printf("Internal IP: %s\n", member.InternalIP)
    
    // Or iterate through all addresses
    for _, addr := range member.Addresses {
        ip := net.ParseIP(addr)
        if ip != nil && ip.To4() != nil {
            log.Printf("IPv4: %s\n", addr)
        }
    }
}
```

#### Join a Worker Node Manually

```go
import "talos-probe/talos"

err := talos.WorkerJoin(
    context.Background(),
    "10.3.5.30",           // Node IP
    "./worker.yaml",        // Worker config file
    "./talosconfig",        // Talos config file
    10*time.Second,         // Timeout
)
if err != nil {
    log.Fatalf("Failed to join worker: %v", err)
}
```

## Requirements

- Go 1.24.7 or later
- `talosctl` command-line tool installed and in PATH
- Valid talosconfig file with cluster access
- Worker configuration YAML file (for joining new nodes)
- Network access to Talos cluster (port 50000)

## Dependencies

```go
require (
    github.com/thedevsaddam/unpack v1.0.0
    github.com/siderolabs/talos/pkg/machinery v1.11.5
    golang.org/x/sync v0.17.0
    google.golang.org/grpc v1.76.0
)
```

## Package Structure

```
talos-probe/
├── main.go              # Main program with CIDR scanning and auto-join logic
├── probe/
│   └── probe.go        # Network scanning utilities (CIDR parsing, TCP dial)
├── talos/
│   └── join.go         # Talos cluster operations (GetMembers, WorkerJoin)
├── talosconfig         # Talos cluster configuration
├── worker.yaml         # Worker node configuration template
└── USAGE.md           # This file
```

## Security Notes

- The `--insecure` flag is used when joining new workers (no existing trust)
- Ensure your talosconfig has appropriate permissions
- The tool scans networks concurrently - be mindful of network policies
- Worker configuration should be properly secured and not committed to version control

