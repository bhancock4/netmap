package scanner

import (
	"context"
	"strings"
	"time"

	"github.com/bhancock4/netmap/internal/model"
	"github.com/likexian/whois"
)

// WhoisProbe performs a WHOIS lookup on a domain.
func WhoisProbe(ctx context.Context, graph *model.Graph, nodeID string, target string) {
	probe := model.ProbeResult{
		Type:   "whois",
		Status: model.ProbeStatusRunning,
		Data:   make(map[string]string),
	}
	graph.UpdateProbe(nodeID, "whois", probe)

	start := time.Now()
	result, err := whois.Whois(target)
	probe.Latency = time.Since(start)

	if err != nil {
		probe.Status = model.ProbeStatusFailed
		probe.Error = err.Error()
		graph.UpdateProbe(nodeID, "whois", probe)
		return
	}

	// Parse key fields from whois output
	for _, line := range strings.Split(result, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "%") || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(strings.ToLower(parts[0]))
		value := strings.TrimSpace(parts[1])

		switch key {
		case "registrar":
			probe.Data["registrar"] = value
		case "creation date":
			probe.Data["created"] = value
		case "registry expiry date", "expiry date":
			probe.Data["expires"] = value
		case "registrant organization", "org-name":
			probe.Data["organization"] = value
		case "registrant country", "country":
			if _, exists := probe.Data["country"]; !exists {
				probe.Data["country"] = value
			}
		case "name server":
			if existing, ok := probe.Data["nameservers"]; ok {
				probe.Data["nameservers"] = existing + ", " + value
			} else {
				probe.Data["nameservers"] = value
			}
		}
	}

	probe.Status = model.ProbeStatusSuccess
	graph.UpdateProbe(nodeID, "whois", probe)
}
