package export

import (
	"fmt"
	"strings"
)

// DiffResult holds the comparison between two scan reports.
type DiffResult struct {
	Target     string
	OldScan    string // timestamp
	NewScan    string // timestamp
	AddedNodes []ReportNode
	RemovedNodes []ReportNode
	ChangedNodes []NodeDiff
	AddedEdges   []ReportEdge
	RemovedEdges []ReportEdge
	Summary      DiffSummary
}

// DiffSummary provides high-level stats.
type DiffSummary struct {
	NodesAdded   int
	NodesRemoved int
	NodesChanged int
	EdgesAdded   int
	EdgesRemoved int
	PortsOpened  []string // "host:port"
	PortsClosed  []string
	CertsChanged []string // hosts where TLS certs changed
}

// NodeDiff describes changes to a specific node between scans.
type NodeDiff struct {
	ID      string
	Label   string
	Changes []Change
}

// Change is a single field-level change.
type Change struct {
	Probe    string // probe type (dns, tls, ports, etc.)
	Field    string
	OldValue string
	NewValue string
}

// Diff compares two reports and returns the differences.
func Diff(old, new Report) DiffResult {
	result := DiffResult{
		Target:  new.Target,
		OldScan: old.ScannedAt,
		NewScan: new.ScannedAt,
	}

	oldNodes := make(map[string]ReportNode)
	for _, n := range old.Nodes {
		oldNodes[n.ID] = n
	}
	newNodes := make(map[string]ReportNode)
	for _, n := range new.Nodes {
		newNodes[n.ID] = n
	}

	// Find added and removed nodes
	for id, n := range newNodes {
		if _, exists := oldNodes[id]; !exists {
			result.AddedNodes = append(result.AddedNodes, n)
		}
	}
	for id, n := range oldNodes {
		if _, exists := newNodes[id]; !exists {
			result.RemovedNodes = append(result.RemovedNodes, n)
		}
	}

	// Find changed nodes
	for id, newNode := range newNodes {
		oldNode, exists := oldNodes[id]
		if !exists {
			continue
		}
		changes := diffNode(oldNode, newNode)
		if len(changes) > 0 {
			result.ChangedNodes = append(result.ChangedNodes, NodeDiff{
				ID:      id,
				Label:   newNode.Label,
				Changes: changes,
			})
		}
	}

	// Diff edges
	oldEdgeSet := make(map[string]bool)
	for _, e := range old.Edges {
		oldEdgeSet[edgeKey(e)] = true
	}
	newEdgeSet := make(map[string]bool)
	for _, e := range new.Edges {
		newEdgeSet[edgeKey(e)] = true
	}
	for _, e := range new.Edges {
		if !oldEdgeSet[edgeKey(e)] {
			result.AddedEdges = append(result.AddedEdges, e)
		}
	}
	for _, e := range old.Edges {
		if !newEdgeSet[edgeKey(e)] {
			result.RemovedEdges = append(result.RemovedEdges, e)
		}
	}

	// Build summary
	result.Summary = buildSummary(result)
	return result
}

func diffNode(old, new ReportNode) []Change {
	var changes []Change

	oldProbes := make(map[string]ReportProbe)
	for _, p := range old.Probes {
		oldProbes[p.Type] = p
	}

	for _, newProbe := range new.Probes {
		oldProbe, exists := oldProbes[newProbe.Type]
		if !exists {
			changes = append(changes, Change{
				Probe:    newProbe.Type,
				Field:    "probe",
				NewValue: "added",
			})
			continue
		}

		// Compare status
		if oldProbe.Status != newProbe.Status {
			changes = append(changes, Change{
				Probe:    newProbe.Type,
				Field:    "status",
				OldValue: oldProbe.Status,
				NewValue: newProbe.Status,
			})
		}

		// Compare latency
		if oldProbe.Latency != newProbe.Latency {
			changes = append(changes, Change{
				Probe:    newProbe.Type,
				Field:    "latency",
				OldValue: oldProbe.Latency,
				NewValue: newProbe.Latency,
			})
		}

		// Compare data fields
		for k, newVal := range newProbe.Data {
			oldVal := oldProbe.Data[k]
			if oldVal != newVal {
				changes = append(changes, Change{
					Probe:    newProbe.Type,
					Field:    k,
					OldValue: oldVal,
					NewValue: newVal,
				})
			}
		}
		// Check for removed data fields
		for k, oldVal := range oldProbe.Data {
			if _, exists := newProbe.Data[k]; !exists {
				changes = append(changes, Change{
					Probe:    newProbe.Type,
					Field:    k,
					OldValue: oldVal,
					NewValue: "(removed)",
				})
			}
		}
	}

	// Check for removed probes
	newProbes := make(map[string]bool)
	for _, p := range new.Probes {
		newProbes[p.Type] = true
	}
	for _, oldProbe := range old.Probes {
		if !newProbes[oldProbe.Type] {
			changes = append(changes, Change{
				Probe:    oldProbe.Type,
				Field:    "probe",
				OldValue: "present",
				NewValue: "(removed)",
			})
		}
	}

	return changes
}

func edgeKey(e ReportEdge) string {
	return fmt.Sprintf("%s->%s:%s", e.From, e.To, e.Type)
}

func buildSummary(d DiffResult) DiffSummary {
	s := DiffSummary{
		NodesAdded:   len(d.AddedNodes),
		NodesRemoved: len(d.RemovedNodes),
		NodesChanged: len(d.ChangedNodes),
		EdgesAdded:   len(d.AddedEdges),
		EdgesRemoved: len(d.RemovedEdges),
	}

	for _, nd := range d.ChangedNodes {
		for _, c := range nd.Changes {
			if c.Probe == "ports" && c.Field == "open" {
				// Parse port changes
				oldPorts := parsePortList(c.OldValue)
				newPorts := parsePortList(c.NewValue)
				for p := range newPorts {
					if !oldPorts[p] {
						s.PortsOpened = append(s.PortsOpened, nd.Label+":"+p)
					}
				}
				for p := range oldPorts {
					if !newPorts[p] {
						s.PortsClosed = append(s.PortsClosed, nd.Label+":"+p)
					}
				}
			}
			if c.Probe == "tls" && (c.Field == "subject" || c.Field == "not_after" || c.Field == "issuer") {
				s.CertsChanged = append(s.CertsChanged, nd.Label)
			}
		}
	}

	return s
}

func parsePortList(s string) map[string]bool {
	ports := make(map[string]bool)
	if s == "" || s == "none" {
		return ports
	}
	for _, entry := range strings.Split(s, ", ") {
		entry = strings.TrimSpace(entry)
		if entry != "" {
			ports[entry] = true
		}
	}
	return ports
}
