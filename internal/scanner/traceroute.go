package scanner

import (
	"context"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/bhancock4/netmap/internal/model"
)

var traceHopRegex = regexp.MustCompile(`^\s*(\d+)\s+(.+)$`)
var traceIPRegex = regexp.MustCompile(`\(?([\d.]+)\)?\s+([\d.]+)\s*ms`)
var traceStarRegex = regexp.MustCompile(`^\s*(\d+)\s+\*\s+\*\s+\*`)

// TracerouteProbe runs traceroute against a target and adds router nodes.
func TracerouteProbe(ctx context.Context, graph *model.Graph, parentNodeID string, address string) {
	probe := model.ProbeResult{
		Type:   "traceroute",
		Status: model.ProbeStatusRunning,
		Data:   make(map[string]string),
	}
	graph.UpdateProbe(parentNodeID, "traceroute", probe)

	cmd := exec.CommandContext(ctx, "traceroute", "-m", "15", "-w", "2", "-q", "1", address)
	output, err := cmd.CombinedOutput()

	if err != nil && len(output) == 0 {
		probe.Status = model.ProbeStatusFailed
		probe.Error = err.Error()
		graph.UpdateProbe(parentNodeID, "traceroute", probe)
		return
	}

	lines := strings.Split(string(output), "\n")
	hopCount := 0
	prevNodeID := parentNodeID

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "traceroute") {
			continue
		}

		// Check for * * * (no response)
		if traceStarRegex.MatchString(line) {
			hopCount++
			continue
		}

		// Parse hop with IP and latency
		if matches := traceIPRegex.FindStringSubmatch(line); len(matches) >= 3 {
			hopCount++
			ip := matches[1]
			latency := matches[2]

			routerID := model.NodeID(model.NodeTypeRouter, ip)
			routerNode := &model.Node{
				ID:      routerID,
				Label:   ip,
				Type:    model.NodeTypeRouter,
				Address: ip,
				Depth:   hopCount,
				Parent:  parentNodeID,
			}
			graph.AddNode(routerNode)
			graph.AddEdge(model.Edge{
				From:  prevNodeID,
				To:    routerID,
				Type:  model.EdgeTypeRoute,
				Label: "hop " + latency + "ms",
			})
			prevNodeID = routerID
		}
	}

	probe.Status = model.ProbeStatusSuccess
	probe.Data["hops"] = strconv.Itoa(hopCount)
	graph.UpdateProbe(parentNodeID, "traceroute", probe)
}
