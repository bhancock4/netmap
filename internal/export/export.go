package export

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/bhancock4/netmap/internal/model"
)

// Format represents an export format.
type Format string

const (
	FormatYAML Format = "yaml"
	FormatJSON Format = "json"
)

// Report is the serializable scan result.
type Report struct {
	Target    string       `json:"target" yaml:"target"`
	ScannedAt string       `json:"scanned_at" yaml:"scanned_at"`
	Duration  string       `json:"duration" yaml:"duration"`
	NodeCount int          `json:"node_count" yaml:"node_count"`
	EdgeCount int          `json:"edge_count" yaml:"edge_count"`
	Nodes     []ReportNode `json:"nodes" yaml:"nodes"`
	Edges     []ReportEdge `json:"edges" yaml:"edges"`
}

// ReportNode is a serializable node.
type ReportNode struct {
	ID      string            `json:"id" yaml:"id"`
	Label   string            `json:"label" yaml:"label"`
	Type    string            `json:"type" yaml:"type"`
	Address string            `json:"address" yaml:"address"`
	Depth   int               `json:"depth" yaml:"depth"`
	Probes  []ReportProbe     `json:"probes,omitempty" yaml:"probes,omitempty"`
}

// ReportProbe is a serializable probe result.
type ReportProbe struct {
	Type    string            `json:"type" yaml:"type"`
	Status  string            `json:"status" yaml:"status"`
	Latency string            `json:"latency,omitempty" yaml:"latency,omitempty"`
	Error   string            `json:"error,omitempty" yaml:"error,omitempty"`
	Data    map[string]string `json:"data,omitempty" yaml:"data,omitempty"`
}

// ReportEdge is a serializable edge.
type ReportEdge struct {
	From  string `json:"from" yaml:"from"`
	To    string `json:"to" yaml:"to"`
	Type  string `json:"type" yaml:"type"`
	Label string `json:"label,omitempty" yaml:"label,omitempty"`
}

// BuildReport creates a Report from a graph using a consistent snapshot.
func BuildReport(graph *model.Graph, target string, duration time.Duration) Report {
	nodes, edges := graph.Snapshot()

	report := Report{
		Target:    target,
		ScannedAt: time.Now().UTC().Format(time.RFC3339),
		Duration:  duration.Round(time.Millisecond).String(),
		NodeCount: len(nodes),
		EdgeCount: len(edges),
	}

	for _, node := range nodes {
		rn := ReportNode{
			ID:      node.ID,
			Label:   node.Label,
			Type:    node.Type.String(),
			Address: node.Address,
			Depth:   node.Depth,
		}
		for _, probe := range node.Probes {
			rp := ReportProbe{
				Type:   probe.Type,
				Status: probe.Status.String(),
				Data:   probe.Data,
				Error:  probe.Error,
			}
			if probe.Latency > 0 {
				rp.Latency = probe.Latency.Round(time.Millisecond).String()
			}
			rn.Probes = append(rn.Probes, rp)
		}
		report.Nodes = append(report.Nodes, rn)
	}
	sort.Slice(report.Nodes, func(i, j int) bool {
		if report.Nodes[i].Depth != report.Nodes[j].Depth {
			return report.Nodes[i].Depth < report.Nodes[j].Depth
		}
		return report.Nodes[i].Label < report.Nodes[j].Label
	})

	for _, edge := range edges {
		report.Edges = append(report.Edges, ReportEdge{
			From:  edge.From,
			To:    edge.To,
			Type:  edge.Type.String(),
			Label: edge.Label,
		})
	}

	return report
}

// Marshal serializes a report to the given format.
func Marshal(report Report, format Format) ([]byte, error) {
	switch format {
	case FormatJSON:
		return json.MarshalIndent(report, "", "  ")
	case FormatYAML:
		return marshalYAML(report), nil
	default:
		return nil, fmt.Errorf("unknown format: %s", format)
	}
}

// WriteFile writes a report to a file.
func WriteFile(report Report, path string, format Format) error {
	data, err := Marshal(report, format)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// marshalYAML produces clean YAML without a dependency.
func marshalYAML(r Report) []byte {
	var b strings.Builder

	b.WriteString("# netmap scan results\n")
	b.WriteString(fmt.Sprintf("target: %s\n", r.Target))
	b.WriteString(fmt.Sprintf("scanned_at: %s\n", r.ScannedAt))
	b.WriteString(fmt.Sprintf("duration: %s\n", r.Duration))
	b.WriteString(fmt.Sprintf("node_count: %d\n", r.NodeCount))
	b.WriteString(fmt.Sprintf("edge_count: %d\n", r.EdgeCount))

	b.WriteString("\nnodes:\n")
	for _, node := range r.Nodes {
		b.WriteString(fmt.Sprintf("  - id: %s\n", yamlStr(node.ID)))
		b.WriteString(fmt.Sprintf("    label: %s\n", yamlStr(node.Label)))
		b.WriteString(fmt.Sprintf("    type: %s\n", node.Type))
		b.WriteString(fmt.Sprintf("    address: %s\n", yamlStr(node.Address)))
		b.WriteString(fmt.Sprintf("    depth: %d\n", node.Depth))

		if len(node.Probes) > 0 {
			b.WriteString("    probes:\n")
			for _, probe := range node.Probes {
				b.WriteString(fmt.Sprintf("      - type: %s\n", probe.Type))
				b.WriteString(fmt.Sprintf("        status: %s\n", probe.Status))
				if probe.Latency != "" {
					b.WriteString(fmt.Sprintf("        latency: %s\n", probe.Latency))
				}
				if probe.Error != "" {
					b.WriteString(fmt.Sprintf("        error: %s\n", yamlStr(probe.Error)))
				}
				if len(probe.Data) > 0 {
					b.WriteString("        data:\n")
					keys := sortedKeys(probe.Data)
					for _, k := range keys {
						b.WriteString(fmt.Sprintf("          %s: %s\n", k, yamlStr(probe.Data[k])))
					}
				}
			}
		}
	}

	if len(r.Edges) > 0 {
		b.WriteString("\nedges:\n")
		for _, edge := range r.Edges {
			b.WriteString(fmt.Sprintf("  - from: %s\n", yamlStr(edge.From)))
			b.WriteString(fmt.Sprintf("    to: %s\n", yamlStr(edge.To)))
			b.WriteString(fmt.Sprintf("    type: %s\n", edge.Type))
			if edge.Label != "" {
				b.WriteString(fmt.Sprintf("    label: %s\n", yamlStr(edge.Label)))
			}
		}
	}

	return []byte(b.String())
}

func yamlStr(s string) string {
	// Quote strings that contain special YAML characters
	if strings.ContainsAny(s, ":{}[]#&*!|>'\",\n") || s == "" {
		escaped := strings.ReplaceAll(s, `"`, `\"`)
		return `"` + escaped + `"`
	}
	return s
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
