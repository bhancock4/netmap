package scanner

import (
	"context"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/bhancock4/netmap/internal/model"
)

var pingTimeRegex = regexp.MustCompile(`time[=<]\s*([\d.]+)\s*ms`)
var pingStatsRegex = regexp.MustCompile(`(\d+) packets transmitted, (\d+).*received`)
var pingAvgRegex = regexp.MustCompile(`min/avg/max.*=\s*([\d.]+)/([\d.]+)/([\d.]+)`)

// PingProbe pings a target address using the system ping command.
func PingProbe(ctx context.Context, graph *model.Graph, nodeID string, address string) {
	probe := model.ProbeResult{
		Type:   "ping",
		Status: model.ProbeStatusRunning,
		Data:   make(map[string]string),
	}
	graph.UpdateProbe(nodeID, "ping", probe)

	start := time.Now()
	cmd := exec.CommandContext(ctx, "ping", "-c", "3", "-W", "2", address)
	output, err := cmd.CombinedOutput()
	probe.Latency = time.Since(start)

	result := string(output)

	if err != nil {
		// Check if we got partial results
		if !strings.Contains(result, "packets transmitted") {
			probe.Status = model.ProbeStatusFailed
			probe.Error = "host unreachable"
			graph.UpdateProbe(nodeID, "ping", probe)
			return
		}
	}

	// Parse stats
	if matches := pingStatsRegex.FindStringSubmatch(result); len(matches) >= 3 {
		probe.Data["transmitted"] = matches[1]
		probe.Data["received"] = matches[2]
		received, _ := strconv.Atoi(matches[2])
		if received == 0 {
			probe.Status = model.ProbeStatusTimeout
			probe.Error = "0 packets received"
			graph.UpdateProbe(nodeID, "ping", probe)
			return
		}
	}

	// Parse avg latency
	if matches := pingAvgRegex.FindStringSubmatch(result); len(matches) >= 4 {
		probe.Data["min_ms"] = matches[1]
		probe.Data["avg_ms"] = matches[2]
		probe.Data["max_ms"] = matches[3]
		if avg, err := strconv.ParseFloat(matches[2], 64); err == nil {
			probe.Latency = time.Duration(avg * float64(time.Millisecond))
		}
	}

	// Parse individual ping times
	times := pingTimeRegex.FindAllStringSubmatch(result, -1)
	if len(times) > 0 {
		timeStrs := make([]string, 0, len(times))
		for _, t := range times {
			timeStrs = append(timeStrs, t[1]+"ms")
		}
		probe.Data["replies"] = strings.Join(timeStrs, ", ")
	}

	probe.Status = model.ProbeStatusSuccess
	graph.UpdateProbe(nodeID, "ping", probe)
}
