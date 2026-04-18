package export

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/bhancock4/netmap/internal/model"
)

func newTestGraph() *model.Graph {
	g := model.NewGraph()
	g.AddNode(&model.Node{
		ID:      "HOST:example.com",
		Label:   "example.com",
		Type:    model.NodeTypeHost,
		Address: "example.com",
		Depth:   0,
		Probes: []model.ProbeResult{
			{
				Type:    "dns",
				Status:  model.ProbeStatusSuccess,
				Latency: 15 * time.Millisecond,
				Data:    map[string]string{"a_records": "1.2.3.4"},
			},
		},
	})
	g.AddNode(&model.Node{
		ID:      "IP:1.2.3.4",
		Label:   "1.2.3.4",
		Type:    model.NodeTypeIP,
		Address: "1.2.3.4",
		Depth:   1,
		Parent:  "HOST:example.com",
	})
	g.AddEdge(model.Edge{
		From:  "HOST:example.com",
		To:    "IP:1.2.3.4",
		Type:  model.EdgeTypeDNS,
		Label: "A",
	})
	return g
}

func TestBuildReport(t *testing.T) {
	g := newTestGraph()
	report := BuildReport(g, "example.com", 2*time.Second)

	if report.Target != "example.com" {
		t.Errorf("expected target example.com, got %s", report.Target)
	}
	if report.NodeCount != 2 {
		t.Errorf("expected 2 nodes, got %d", report.NodeCount)
	}
	if report.EdgeCount != 1 {
		t.Errorf("expected 1 edge, got %d", report.EdgeCount)
	}
	if report.Duration != "2s" {
		t.Errorf("expected duration 2s, got %s", report.Duration)
	}
	if len(report.Nodes) != 2 {
		t.Fatalf("expected 2 report nodes, got %d", len(report.Nodes))
	}
	// Nodes should be sorted by depth
	if report.Nodes[0].Depth != 0 {
		t.Errorf("expected first node at depth 0, got %d", report.Nodes[0].Depth)
	}
}

func TestMarshalJSON(t *testing.T) {
	g := newTestGraph()
	report := BuildReport(g, "example.com", 1*time.Second)

	data, err := Marshal(report, FormatJSON)
	if err != nil {
		t.Fatalf("Marshal JSON failed: %v", err)
	}

	// Should be valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if parsed["target"] != "example.com" {
		t.Errorf("JSON target mismatch")
	}
}

func TestMarshalYAML(t *testing.T) {
	g := newTestGraph()
	report := BuildReport(g, "example.com", 1*time.Second)

	data, err := Marshal(report, FormatYAML)
	if err != nil {
		t.Fatalf("Marshal YAML failed: %v", err)
	}

	yaml := string(data)
	if !strings.Contains(yaml, "target: example.com") {
		t.Error("YAML missing target field")
	}
	if !strings.Contains(yaml, "node_count: 2") {
		t.Error("YAML missing node_count")
	}
	if !strings.Contains(yaml, "edge_count: 1") {
		t.Error("YAML missing edge_count")
	}
	if !strings.Contains(yaml, "a_records") {
		t.Error("YAML missing probe data")
	}
}

func TestMarshalUnknownFormat(t *testing.T) {
	_, err := Marshal(Report{}, "xml")
	if err == nil {
		t.Error("expected error for unknown format")
	}
}

func TestYamlStrEscaping(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple", "simple"},
		{"has: colon", `"has: colon"`},
		{"has \"quotes\"", `"has \"quotes\""`},
		{"", `""`},
		{"has #comment", `"has #comment"`},
	}
	for _, tt := range tests {
		got := yamlStr(tt.input)
		if got != tt.want {
			t.Errorf("yamlStr(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
