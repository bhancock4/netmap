package scanner

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"os/exec"
	"sync"
	"time"

	"github.com/bhancock4/netmap/internal/model"
)

// ExpandCIDR returns all host IPs in a CIDR range.
// Excludes network and broadcast addresses.
func ExpandCIDR(cidr string) ([]net.IP, error) {
	ip, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}

	var ips []net.IP
	maskSize, bits := ipNet.Mask.Size()
	if bits != 32 {
		return nil, fmt.Errorf("only IPv4 CIDR is supported")
	}

	// Limit to /16 max (65534 hosts)
	if maskSize < 16 {
		return nil, fmt.Errorf("CIDR range too large (min /16)")
	}

	numHosts := 1 << (bits - maskSize)

	startIP := ip.Mask(ipNet.Mask).To4()
	start := binary.BigEndian.Uint32(startIP)

	for i := 1; i < numHosts-1; i++ { // skip network and broadcast
		next := start + uint32(i)
		ip := make(net.IP, 4)
		binary.BigEndian.PutUint32(ip, next)
		ips = append(ips, ip)
	}

	return ips, nil
}

// SubnetScan performs a ping sweep on a CIDR range and adds live hosts.
func SubnetScan(ctx context.Context, s *Scanner, cidr string) error {
	ips, err := ExpandCIDR(cidr)
	if err != nil {
		return err
	}

	// Create a root node for the subnet
	rootID := model.NodeID(model.NodeTypeRouter, cidr)
	rootNode := &model.Node{
		ID:      rootID,
		Label:   cidr,
		Type:    model.NodeTypeRouter,
		Address: cidr,
		Depth:   0,
	}
	s.Graph.AddNode(rootNode)
	s.emit(model.EventNodeAdded, rootID, "Subnet: "+cidr)

	// Ping sweep with concurrency limit
	var wg sync.WaitGroup
	sem := make(chan struct{}, 50) // 50 concurrent pings

	alive := 0
	var mu sync.Mutex

	for _, ip := range ips {
		if ctx.Err() != nil {
			break
		}

		wg.Add(1)
		go func(addr string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if isAlive(ctx, addr) {
				nodeID := model.NodeID(model.NodeTypeIP, addr)
				node := &model.Node{
					ID:      nodeID,
					Label:   addr,
					Type:    model.NodeTypeIP,
					Address: addr,
					Depth:   1,
					Parent:  rootID,
				}
				s.Graph.AddNode(node)
				s.Graph.AddEdge(model.Edge{
					From:  rootID,
					To:    nodeID,
					Type:  model.EdgeTypeRoute,
					Label: "subnet",
				})
				s.emit(model.EventNodeAdded, nodeID, "Host alive: "+addr)

				// Try reverse DNS
				names, err := net.LookupAddr(addr)
				if err == nil && len(names) > 0 {
					s.Graph.UpdateProbe(nodeID, "dns", model.ProbeResult{
						Type:   "dns",
						Status: model.ProbeStatusSuccess,
						Data:   map[string]string{"reverse_dns": names[0]},
					})
					// Update label to hostname
					if n, ok := s.Graph.GetNode(nodeID); ok {
						_ = n // can't mutate copy, but probe data is stored
					}
				}

				mu.Lock()
				alive++
				mu.Unlock()
			}
		}(ip.String())
	}

	wg.Wait()

	s.Graph.UpdateProbe(rootID, "sweep", model.ProbeResult{
		Type:   "sweep",
		Status: model.ProbeStatusSuccess,
		Data: map[string]string{
			"total_ips": fmt.Sprintf("%d", len(ips)),
			"alive":     fmt.Sprintf("%d", alive),
		},
	})

	return nil
}

// isAlive does a quick TCP connect or ping check.
func isAlive(ctx context.Context, addr string) bool {
	// Try TCP connect on common ports first (faster than ICMP which needs privileges)
	ports := []string{"80", "443", "22", "445"}
	for _, port := range ports {
		d := net.Dialer{Timeout: 1 * time.Second}
		conn, err := d.DialContext(ctx, "tcp", addr+":"+port)
		if err == nil {
			conn.Close()
			return true
		}
	}

	// Fallback: try ICMP ping via system command
	ctx2, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	result := make(chan bool, 1)
	go func() {
		// Use a single ping with short timeout
		cmd := exec.CommandContext(ctx2, "ping", "-c", "1", "-W", "1", addr)
		err := cmd.Run()
		result <- err == nil
	}()

	select {
	case alive := <-result:
		return alive
	case <-ctx2.Done():
		return false
	}
}
