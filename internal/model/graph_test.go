package model

import (
	"sync"
	"testing"
)

func TestNewGraph(t *testing.T) {
	g := NewGraph()
	if g.Nodes == nil {
		t.Fatal("expected non-nil Nodes map")
	}
	if g.NodeCount() != 0 {
		t.Fatalf("expected 0 nodes, got %d", g.NodeCount())
	}
	if g.Root != "" {
		t.Fatalf("expected empty root, got %q", g.Root)
	}
}

func TestAddNode(t *testing.T) {
	g := NewGraph()

	node := &Node{ID: "host:example.com", Label: "example.com", Type: NodeTypeHost, Address: "example.com"}
	if !g.AddNode(node) {
		t.Fatal("expected AddNode to return true for new node")
	}
	if g.Root != "host:example.com" {
		t.Fatalf("expected root to be set, got %q", g.Root)
	}
	if g.NodeCount() != 1 {
		t.Fatalf("expected 1 node, got %d", g.NodeCount())
	}

	// Adding same node again should return false
	if g.AddNode(node) {
		t.Fatal("expected AddNode to return false for duplicate")
	}
	if g.NodeCount() != 1 {
		t.Fatalf("expected still 1 node, got %d", g.NodeCount())
	}
}

func TestAddNodeParentWiring(t *testing.T) {
	g := NewGraph()

	parent := &Node{ID: "host:parent", Label: "parent"}
	child := &Node{ID: "ip:1.2.3.4", Label: "1.2.3.4", Parent: "host:parent"}

	g.AddNode(parent)
	g.AddNode(child)

	p, ok := g.GetNode("host:parent")
	if !ok {
		t.Fatal("parent node not found")
	}
	if len(p.Children) != 1 || p.Children[0] != "ip:1.2.3.4" {
		t.Fatalf("expected parent to have child, got %v", p.Children)
	}
}

func TestGetNodeReturnsACopy(t *testing.T) {
	g := NewGraph()
	g.AddNode(&Node{ID: "n1", Label: "original", Probes: []ProbeResult{{Type: "test"}}})

	n1, _ := g.GetNode("n1")
	n1.Label = "modified"
	n1.Probes = append(n1.Probes, ProbeResult{Type: "extra"})

	n2, _ := g.GetNode("n1")
	if n2.Label == "modified" {
		t.Fatal("GetNode should return a copy — label mutation leaked")
	}
	if len(n2.Probes) != 1 {
		t.Fatalf("GetNode should return a copy — probe mutation leaked, got %d probes", len(n2.Probes))
	}
}

func TestGetNodeNotFound(t *testing.T) {
	g := NewGraph()
	_, ok := g.GetNode("nonexistent")
	if ok {
		t.Fatal("expected GetNode to return false for missing node")
	}
}

func TestHasNode(t *testing.T) {
	g := NewGraph()
	g.AddNode(&Node{ID: "n1"})

	if !g.HasNode("n1") {
		t.Fatal("expected HasNode to return true")
	}
	if g.HasNode("n2") {
		t.Fatal("expected HasNode to return false for missing node")
	}
}

func TestAddEdge(t *testing.T) {
	g := NewGraph()
	g.AddEdge(Edge{From: "a", To: "b", Type: EdgeTypeDNS, Label: "A"})
	g.AddEdge(Edge{From: "b", To: "c", Type: EdgeTypeRoute})

	_, edges := g.Snapshot()
	if len(edges) != 2 {
		t.Fatalf("expected 2 edges, got %d", len(edges))
	}
}

func TestUpdateProbe(t *testing.T) {
	g := NewGraph()
	g.AddNode(&Node{ID: "n1"})

	// First update creates the probe
	g.UpdateProbe("n1", "dns", ProbeResult{Type: "dns", Status: ProbeStatusRunning})
	n, _ := g.GetNode("n1")
	if len(n.Probes) != 1 {
		t.Fatalf("expected 1 probe, got %d", len(n.Probes))
	}
	if n.Probes[0].Status != ProbeStatusRunning {
		t.Fatalf("expected running, got %s", n.Probes[0].Status)
	}

	// Second update replaces the probe
	g.UpdateProbe("n1", "dns", ProbeResult{Type: "dns", Status: ProbeStatusSuccess})
	n, _ = g.GetNode("n1")
	if len(n.Probes) != 1 {
		t.Fatalf("expected still 1 probe, got %d", len(n.Probes))
	}
	if n.Probes[0].Status != ProbeStatusSuccess {
		t.Fatalf("expected success, got %s", n.Probes[0].Status)
	}
}

func TestSetDeepScanned(t *testing.T) {
	g := NewGraph()
	g.AddNode(&Node{ID: "n1"})

	g.SetDeepScanned("n1")

	n, _ := g.GetNode("n1")
	if !n.DeepScanned {
		t.Fatal("expected DeepScanned to be true")
	}
}

func TestSnapshot(t *testing.T) {
	g := NewGraph()
	g.AddNode(&Node{ID: "n1", Label: "a"})
	g.AddNode(&Node{ID: "n2", Label: "b"})
	g.AddEdge(Edge{From: "n1", To: "n2"})

	nodes, edges := g.Snapshot()
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes in snapshot, got %d", len(nodes))
	}
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge in snapshot, got %d", len(edges))
	}

	// Mutating snapshot should not affect original
	nodes["n1"].Label = "mutated"
	n, _ := g.GetNode("n1")
	if n.Label == "mutated" {
		t.Fatal("snapshot mutation leaked to original graph")
	}
}

func TestNodeID(t *testing.T) {
	id := NodeID(NodeTypeHost, "example.com")
	if id != "HOST:example.com" {
		t.Fatalf("unexpected NodeID: %s", id)
	}
}

func TestStatusSummary(t *testing.T) {
	tests := []struct {
		name   string
		probes []ProbeResult
		want   ProbeStatus
	}{
		{"no probes", nil, ProbeStatusPending},
		{"all success", []ProbeResult{{Status: ProbeStatusSuccess}}, ProbeStatusSuccess},
		{"one running", []ProbeResult{{Status: ProbeStatusSuccess}, {Status: ProbeStatusRunning}}, ProbeStatusRunning},
		{"one failed", []ProbeResult{{Status: ProbeStatusSuccess}, {Status: ProbeStatusFailed}}, ProbeStatusFailed},
		{"timeout", []ProbeResult{{Status: ProbeStatusTimeout}}, ProbeStatusFailed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := &Node{Probes: tt.probes}
			got := n.StatusSummary()
			if got != tt.want {
				t.Errorf("StatusSummary() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestConcurrentAccess(t *testing.T) {
	g := NewGraph()
	g.AddNode(&Node{ID: "root"})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := NodeID(NodeTypeIP, string(rune('a'+i%26)))
			g.AddNode(&Node{ID: id, Parent: "root"})
			g.UpdateProbe(id, "ping", ProbeResult{Type: "ping", Status: ProbeStatusSuccess})
			g.GetNode(id)
			g.HasNode(id)
			g.NodeCount()
		}(i)
	}
	wg.Wait()
}
