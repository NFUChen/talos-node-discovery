package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"slices"
	"strconv"
	"sync"
	"talos-probe/probe"
	"talos-probe/talos"
	"time"

	"github.com/thedevsaddam/unpack"
)

func dialTalosHosts(ips []string, batchSize int) ([]string, error) {
	if batchSize <= 0 && batchSize != -1 {
		log.Fatal("batch size cannot be less than or equal to 0")
	}

	if batchSize == -1 {
		batchSize = 100
	}

	talosHosts := make([]string, 0)
	var mu sync.Mutex

	fmt.Printf("dialing %d talos hosts with batch size %d...\n", len(ips), batchSize)

	for i := 0; i < len(ips); i += batchSize {
		end := i + batchSize
		if end > len(ips) {
			end = len(ips)
		}

		batch := ips[i:end]
		wg := sync.WaitGroup{}
		wg.Add(len(batch))

		fmt.Printf("processing batch %d-%d of %d\n", i+1, end, len(ips))

		for _, ip := range batch {
			go func(ip string) {
				defer wg.Done()
				err := probe.DialAddress(fmt.Sprintf("%s:50000", ip), 3*time.Second)
				if err != nil {
					return
				}
				log.Printf("connected to address %s successfully", ip)
				mu.Lock()
				talosHosts = append(talosHosts, ip)
				mu.Unlock()
			}(ip)
		}

		wg.Wait()
	}

	return talosHosts, nil
}

func main() {
	if len(os.Args) < 5 {
		fmt.Println("Usage: talos_scanner <cidr> <talosconfig path> <worker config path> <batch size>")
		return
	}

	var cidr, talosConfigPath, workerConfigPath, batchSize string
	unpack.Do(os.Args[1:], &cidr, &talosConfigPath, &workerConfigPath, &batchSize)
	log.Println("cidr:", cidr)
	log.Println("talosConfigPath:", talosConfigPath)
	log.Println("workerConfigPath:", workerConfigPath)

	ips, err := probe.IpsFromCIDR(cidr)
	if err != nil {
		log.Fatalf("cidr parse error: %v", err)
	}

	log.Printf("Total IPs to scan: %d\n", len(ips))

	batchSizeInt, err := strconv.Atoi(batchSize)
	if err != nil {
		log.Fatalf("batch size parse error: %v", err)
	}

	talosHostsInCidr, err := dialTalosHosts(ips, batchSizeInt)
	if err != nil {
		log.Println("faild to dial talos hosts.")
	}
	fmt.Println("talos hosts:", talosHostsInCidr)

	fmt.Println("\n=== Fetching Talos Cluster Members ===")
	members, err := talos.GetMembers(context.TODO(), talosConfigPath, 10*time.Second)
	if err != nil {
		log.Println("failed to get members:", err)
		return
	}

	log.Printf("\nFound %d cluster members:\n\n", len(members))
	for i, member := range members {
		log.Printf("Member %d:\n", i+1)
		log.Printf("  Node:         %s\n", member.Node)
		log.Printf("  ID:           %s\n", member.ID)
		log.Printf("  Hostname:     %s\n", member.Hostname)
		log.Printf("  Machine Type: %s\n", member.MachineType)
		log.Printf("  OS:           %s\n", member.OS)
		log.Printf("  Internal IP:  %s\n", member.InternalIP)
		log.Printf("  All Addresses: %v\n", member.Addresses)
		fmt.Println()
	}
	// this will be the member not joined
	var newMemberIps []string
	for _, talosHost := range talosHostsInCidr {
		// Skip if already in newMemberIps
		if slices.Contains(newMemberIps, talosHost) {
			log.Printf("talos host %s already in newMemberIps, skipping\n", talosHost)
			continue
		}

		// Validate IP format
		ip := net.ParseIP(talosHost)
		if ip == nil || ip.To4() == nil {
			continue
		}

		// Check if this host is already a member
		isExistingMember := false
		for _, member := range members {
			if member.InternalIP == talosHost {
				isExistingMember = true
				break
			}
		}

		// Only add if not an existing member
		if !isExistingMember {
			newMemberIps = append(newMemberIps, talosHost)
		}
	}
	log.Printf("new member not joined ips[%d]: %v\n", len(newMemberIps), newMemberIps)

	if len(newMemberIps) == 0 {
		log.Println("no new member to join")
		return
	}

	for _, ip := range newMemberIps {
		log.Printf("joining worker: %s\n", ip)
		err := talos.WorkerJoin(context.TODO(), ip, workerConfigPath, talosConfigPath, 10*time.Second)
		if err != nil {
			log.Println("failed to join worker:", err)
		}
	}
}
