package export

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// LoadReport loads a report from a YAML or JSON file.
func LoadReport(path string) (Report, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Report{}, fmt.Errorf("reading %s: %w", path, err)
	}

	// Detect format by trying JSON first, then YAML
	var report Report
	if err := json.Unmarshal(data, &report); err == nil && report.Target != "" {
		return report, nil
	}

	// Parse YAML manually (matching our output format)
	report, err = parseYAML(data)
	if err != nil {
		return Report{}, fmt.Errorf("parsing %s: %w", path, err)
	}
	return report, nil
}

// parseYAML is a minimal YAML parser that handles our specific output format.
func parseYAML(data []byte) (Report, error) {
	var r Report
	lines := strings.Split(string(data), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		val = strings.Trim(val, `"`)

		switch key {
		case "target":
			r.Target = val
		case "scanned_at":
			r.ScannedAt = val
		case "duration":
			r.Duration = val
		case "node_count":
			fmt.Sscanf(val, "%d", &r.NodeCount)
		case "edge_count":
			fmt.Sscanf(val, "%d", &r.EdgeCount)
		}
	}

	// Parse nodes
	r.Nodes = parseYAMLNodes(lines)
	r.Edges = parseYAMLEdges(lines)

	if r.Target == "" {
		return r, fmt.Errorf("no target found in YAML")
	}
	return r, nil
}

func parseYAMLNodes(lines []string) []ReportNode {
	var nodes []ReportNode
	inNodes := false
	inEdges := false
	var current *ReportNode
	var currentProbe *ReportProbe
	inProbeData := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == "nodes:" {
			inNodes = true
			inEdges = false
			continue
		}
		if trimmed == "edges:" {
			inNodes = false
			inEdges = true
			if current != nil {
				if currentProbe != nil {
					current.Probes = append(current.Probes, *currentProbe)
					currentProbe = nil
				}
				nodes = append(nodes, *current)
				current = nil
			}
			continue
		}
		if inEdges || !inNodes {
			continue
		}

		// New node
		if strings.HasPrefix(trimmed, "- id:") {
			if current != nil {
				if currentProbe != nil {
					current.Probes = append(current.Probes, *currentProbe)
					currentProbe = nil
				}
				nodes = append(nodes, *current)
			}
			current = &ReportNode{}
			inProbeData = false
			val := strings.TrimSpace(strings.TrimPrefix(trimmed, "- id:"))
			current.ID = strings.Trim(val, `"`)
			continue
		}

		if current == nil {
			continue
		}

		// Node fields
		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(strings.TrimPrefix(parts[0], "- "))
		val := strings.TrimSpace(parts[1])
		val = strings.Trim(val, `"`)

		if key == "probes" {
			continue
		}
		if key == "data" {
			inProbeData = true
			continue
		}

		if strings.HasPrefix(trimmed, "      - type:") {
			// New probe
			if currentProbe != nil {
				current.Probes = append(current.Probes, *currentProbe)
			}
			currentProbe = &ReportProbe{Type: val}
			inProbeData = false
			continue
		}

		if currentProbe != nil && inProbeData {
			// Probe data key-value
			currentProbe.Data[key] = val
			continue
		}

		if currentProbe != nil {
			switch key {
			case "status":
				currentProbe.Status = val
			case "latency":
				currentProbe.Latency = val
			case "error":
				currentProbe.Error = val
			case "type":
				// already set
			case "data":
				inProbeData = true
				if currentProbe.Data == nil {
					currentProbe.Data = make(map[string]string)
				}
			}
			continue
		}

		switch key {
		case "label":
			current.Label = val
		case "type":
			current.Type = val
		case "address":
			current.Address = val
		case "depth":
			fmt.Sscanf(val, "%d", &current.Depth)
		}
	}

	if current != nil {
		if currentProbe != nil {
			current.Probes = append(current.Probes, *currentProbe)
		}
		nodes = append(nodes, *current)
	}

	return nodes
}

func parseYAMLEdges(lines []string) []ReportEdge {
	var edges []ReportEdge
	inEdges := false
	var current *ReportEdge

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == "edges:" {
			inEdges = true
			continue
		}
		if !inEdges {
			continue
		}
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if strings.HasPrefix(trimmed, "- from:") {
			if current != nil {
				edges = append(edges, *current)
			}
			current = &ReportEdge{}
			val := strings.TrimSpace(strings.TrimPrefix(trimmed, "- from:"))
			current.From = strings.Trim(val, `"`)
			continue
		}

		if current == nil {
			continue
		}

		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		val = strings.Trim(val, `"`)

		switch key {
		case "to":
			current.To = val
		case "type":
			current.Type = val
		case "label":
			current.Label = val
		}
	}

	if current != nil {
		edges = append(edges, *current)
	}
	return edges
}
