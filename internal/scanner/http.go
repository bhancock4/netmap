package scanner

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/bhancock4/netmap/internal/model"
)

var linkRegex = regexp.MustCompile(`(?i)href=["'](https?://[^"']+)["']`)

// HTTPProbe fetches HTTP headers and discovers linked hosts.
func HTTPProbe(ctx context.Context, graph *model.Graph, nodeID string, host string, breadth int) {
	probe := model.ProbeResult{
		Type:   "http",
		Status: model.ProbeStatusRunning,
		Data:   make(map[string]string),
	}
	graph.UpdateProbe(nodeID, "http", probe)

	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	url := "https://" + host
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		probe.Status = model.ProbeStatusFailed
		probe.Error = err.Error()
		graph.UpdateProbe(nodeID, "http", probe)
		return
	}
	req.Header.Set("User-Agent", "netmap/1.0 (network mapping tool)")

	start := time.Now()
	resp, err := client.Do(req)
	probe.Latency = time.Since(start)

	if err != nil {
		// Try HTTP fallback
		url = "http://" + host
		req, _ = http.NewRequestWithContext(ctx, "GET", url, nil)
		req.Header.Set("User-Agent", "netmap/1.0 (network mapping tool)")
		start = time.Now()
		resp, err = client.Do(req)
		probe.Latency = time.Since(start)
		if err != nil {
			probe.Status = model.ProbeStatusFailed
			probe.Error = err.Error()
			graph.UpdateProbe(nodeID, "http", probe)
			return
		}
	}
	defer resp.Body.Close()

	probe.Data["status"] = fmt.Sprintf("%d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	probe.Data["url"] = resp.Request.URL.String()

	// Interesting headers
	for _, h := range []string{"Server", "X-Powered-By", "Content-Type", "X-Frame-Options", "Strict-Transport-Security"} {
		if v := resp.Header.Get(h); v != "" {
			probe.Data[strings.ToLower(h)] = v
		}
	}

	// Read body (limited) to find links to other hosts
	body, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024)) // 256KB max
	if err == nil {
		links := linkRegex.FindAllStringSubmatch(string(body), -1)
		seen := make(map[string]bool)
		linkedHosts := 0

		for _, match := range links {
			if linkedHosts >= breadth {
				break
			}
			linkURL := match[1]
			// Extract hostname from URL
			parts := strings.SplitN(strings.TrimPrefix(strings.TrimPrefix(linkURL, "https://"), "http://"), "/", 2)
			if len(parts) == 0 {
				continue
			}
			linkHost := strings.Split(parts[0], ":")[0] // strip port
			if linkHost == host || seen[linkHost] {
				continue
			}
			seen[linkHost] = true
			linkedHosts++

			linkNodeID := model.NodeID(model.NodeTypeHost, linkHost)
			if graph.HasNode(linkNodeID) {
				continue
			}
			linkNode := &model.Node{
				ID:      linkNodeID,
				Label:   linkHost,
				Type:    model.NodeTypeHost,
				Address: linkHost,
				Depth:   2,
				Parent:  nodeID,
			}
			graph.AddNode(linkNode)
			graph.AddEdge(model.Edge{
				From:  nodeID,
				To:    linkNodeID,
				Type:  model.EdgeTypeLink,
				Label: "link",
			})
		}
		probe.Data["linked_hosts"] = fmt.Sprintf("%d", linkedHosts)
	}

	probe.Status = model.ProbeStatusSuccess
	graph.UpdateProbe(nodeID, "http", probe)
}
