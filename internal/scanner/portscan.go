package scanner

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bhancock4/netmap/internal/model"
)

// CommonPorts are the top 25 well-known ports to check in a deep scan.
var CommonPorts = []struct {
	Port    int
	Service string
}{
	{21, "FTP"},
	{22, "SSH"},
	{23, "Telnet"},
	{25, "SMTP"},
	{53, "DNS"},
	{80, "HTTP"},
	{110, "POP3"},
	{143, "IMAP"},
	{443, "HTTPS"},
	{465, "SMTPS"},
	{587, "Submission"},
	{993, "IMAPS"},
	{995, "POP3S"},
	{3306, "MySQL"},
	{3389, "RDP"},
	{5432, "PostgreSQL"},
	{5900, "VNC"},
	{6379, "Redis"},
	{8080, "HTTP-Alt"},
	{8443, "HTTPS-Alt"},
	{8888, "HTTP-Alt2"},
	{9090, "Prometheus"},
	{9200, "Elasticsearch"},
	{27017, "MongoDB"},
	{11211, "Memcached"},
}

// PortScanProbe checks common ports via TCP connect.
func PortScanProbe(ctx context.Context, graph *model.Graph, nodeID string, address string) {
	probe := model.ProbeResult{
		Type:   "ports",
		Status: model.ProbeStatusRunning,
		Data:   make(map[string]string),
	}
	graph.UpdateProbe(nodeID, "ports", probe)

	var mu sync.Mutex
	var wg sync.WaitGroup
	openPorts := []string{}
	closedCount := 0

	start := time.Now()

	for _, p := range CommonPorts {
		wg.Add(1)
		go func(port int, service string) {
			defer wg.Done()

			addr := fmt.Sprintf("%s:%d", address, port)
			dialer := &net.Dialer{Timeout: 3 * time.Second}

			conn, err := dialer.DialContext(ctx, "tcp", addr)
			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				closedCount++
				return
			}
			conn.Close()

			entry := fmt.Sprintf("%d/%s", port, service)
			openPorts = append(openPorts, entry)
		}(p.Port, p.Service)
	}

	wg.Wait()
	probe.Latency = time.Since(start)

	sort.Strings(openPorts)
	probe.Data["open"] = strings.Join(openPorts, ", ")
	probe.Data["open_count"] = fmt.Sprintf("%d", len(openPorts))
	probe.Data["closed_count"] = fmt.Sprintf("%d", closedCount)
	probe.Data["scanned"] = fmt.Sprintf("%d", len(CommonPorts))

	if len(openPorts) == 0 {
		probe.Data["open"] = "none"
	}

	probe.Status = model.ProbeStatusSuccess
	graph.UpdateProbe(nodeID, "ports", probe)
}

// BannerGrabProbe connects to open ports and reads service banners.
func BannerGrabProbe(ctx context.Context, graph *model.Graph, nodeID string, address string) {
	// First check which ports are open from the port scan
	node, ok := graph.GetNode(nodeID)
	if !ok {
		return
	}

	probe := model.ProbeResult{
		Type:   "banners",
		Status: model.ProbeStatusRunning,
		Data:   make(map[string]string),
	}
	graph.UpdateProbe(nodeID, "banners", probe)

	// Find open ports from the ports probe
	var openPorts []int
	for _, p := range node.Probes {
		if p.Type == "ports" && p.Data["open"] != "none" {
			for _, entry := range strings.Split(p.Data["open"], ", ") {
				var port int
				fmt.Sscanf(entry, "%d/", &port)
				if port > 0 {
					openPorts = append(openPorts, port)
				}
			}
		}
	}

	if len(openPorts) == 0 {
		probe.Status = model.ProbeStatusSuccess
		probe.Data["info"] = "no open ports to grab banners from"
		graph.UpdateProbe(nodeID, "banners", probe)
		return
	}

	start := time.Now()
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, port := range openPorts {
		wg.Add(1)
		go func(p int) {
			defer wg.Done()

			addr := fmt.Sprintf("%s:%d", address, p)
			dialer := &net.Dialer{Timeout: 3 * time.Second}

			conn, err := dialer.DialContext(ctx, "tcp", addr)
			if err != nil {
				return
			}
			defer conn.Close()

			// Set read deadline
			conn.SetReadDeadline(time.Now().Add(3 * time.Second))

			buf := make([]byte, 512)
			n, err := conn.Read(buf)
			if err != nil || n == 0 {
				return
			}

			banner := strings.TrimSpace(string(buf[:n]))
			// Clean up non-printable characters
			cleaned := strings.Map(func(r rune) rune {
				if r < 32 && r != '\n' && r != '\r' && r != '\t' {
					return '.'
				}
				return r
			}, banner)

			// Take first line only
			if idx := strings.IndexAny(cleaned, "\r\n"); idx > 0 {
				cleaned = cleaned[:idx]
			}

			if cleaned != "" {
				mu.Lock()
				probe.Data[fmt.Sprintf("port_%d", p)] = cleaned
				mu.Unlock()
			}
		}(port)
	}

	wg.Wait()
	probe.Latency = time.Since(start)
	probe.Status = model.ProbeStatusSuccess
	graph.UpdateProbe(nodeID, "banners", probe)
}
