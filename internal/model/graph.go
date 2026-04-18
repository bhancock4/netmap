package model

import (
	"fmt"
	"sync"
	"time"
)

// NodeType represents what kind of network entity a node is.
type NodeType int

const (
	NodeTypeHost NodeType = iota
	NodeTypeIP
	NodeTypeRouter
)

func (n NodeType) String() string {
	switch n {
	case NodeTypeHost:
		return "HOST"
	case NodeTypeIP:
		return "IP"
	case NodeTypeRouter:
		return "ROUTER"
	default:
		return "UNKNOWN"
	}
}

// ProbeStatus represents the result of a probe against a node.
type ProbeStatus int

const (
	ProbeStatusPending ProbeStatus = iota
	ProbeStatusRunning
	ProbeStatusSuccess
	ProbeStatusFailed
	ProbeStatusTimeout
)

func (p ProbeStatus) String() string {
	switch p {
	case ProbeStatusPending:
		return "pending"
	case ProbeStatusRunning:
		return "running"
	case ProbeStatusSuccess:
		return "ok"
	case ProbeStatusFailed:
		return "fail"
	case ProbeStatusTimeout:
		return "timeout"
	default:
		return "unknown"
	}
}

// EdgeType represents the relationship between two nodes.
type EdgeType int

const (
	EdgeTypeDNS EdgeType = iota // hostname resolves to IP
	EdgeTypeRoute               // traceroute hop
	EdgeTypeLink                // HTTP link to another host
	EdgeTypeCert                // TLS certificate SAN
)

func (e EdgeType) String() string {
	switch e {
	case EdgeTypeDNS:
		return "DNS"
	case EdgeTypeRoute:
		return "ROUTE"
	case EdgeTypeLink:
		return "LINK"
	case EdgeTypeCert:
		return "CERT"
	default:
		return "UNKNOWN"
	}
}

// EventType constants for scanner events.
const (
	EventProbeStart = "probe_start"
	EventProbeDone  = "probe_done"
	EventNodeAdded  = "node_added"
	EventScanDone   = "scan_done"
	EventDeepStart  = "deep_start"
	EventDeepDone   = "deep_done"
)

// ProbeResult holds the result of a single network probe.
type ProbeResult struct {
	Type    string
	Status  ProbeStatus
	Latency time.Duration
	Data    map[string]string
	Error   string
}

// Node represents a network entity in the graph.
type Node struct {
	ID          string
	Label       string
	Type        NodeType
	Address     string
	Depth       int
	Probes      []ProbeResult
	Children    []string // IDs of child nodes
	Parent      string   // ID of parent node
	DeepScanned bool     // whether a deep scan has been run
}

// StatusSummary returns the overall status of this node based on its probes.
// IMPORTANT: must be called while holding graph read lock.
func (n *Node) StatusSummary() ProbeStatus {
	if len(n.Probes) == 0 {
		return ProbeStatusPending
	}
	hasRunning := false
	hasFailed := false
	for _, p := range n.Probes {
		switch p.Status {
		case ProbeStatusRunning:
			hasRunning = true
		case ProbeStatusFailed, ProbeStatusTimeout:
			hasFailed = true
		}
	}
	if hasRunning {
		return ProbeStatusRunning
	}
	if hasFailed {
		return ProbeStatusFailed
	}
	return ProbeStatusSuccess
}

// Edge represents a relationship between two nodes.
type Edge struct {
	From  string
	To    string
	Type  EdgeType
	Label string
}

// Graph is the main data structure holding the network map.
type Graph struct {
	mu    sync.RWMutex
	Nodes map[string]*Node
	Edges []Edge
	Root  string
}

// NewGraph creates a new empty graph.
func NewGraph() *Graph {
	return &Graph{
		Nodes: make(map[string]*Node),
	}
}

// AddNode adds a node to the graph. Returns false if it already exists.
func (g *Graph) AddNode(node *Node) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, exists := g.Nodes[node.ID]; exists {
		return false
	}
	g.Nodes[node.ID] = node
	if g.Root == "" {
		g.Root = node.ID
	}
	if node.Parent != "" {
		if parent, ok := g.Nodes[node.Parent]; ok {
			parent.Children = append(parent.Children, node.ID)
		}
	}
	return true
}

// AddEdge adds an edge to the graph.
func (g *Graph) AddEdge(edge Edge) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.Edges = append(g.Edges, edge)
}

// GetNode returns a snapshot of a node by ID. The returned node is a copy
// safe to read without holding a lock.
func (g *Graph) GetNode(id string) (*Node, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	n, ok := g.Nodes[id]
	if !ok {
		return nil, false
	}
	// Return a shallow copy to avoid races on slice reads
	cp := *n
	cp.Probes = make([]ProbeResult, len(n.Probes))
	copy(cp.Probes, n.Probes)
	cp.Children = make([]string, len(n.Children))
	copy(cp.Children, n.Children)
	return &cp, true
}

// SetDeepScanned marks a node as deep scanned under the graph lock.
func (g *Graph) SetDeepScanned(nodeID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if n, ok := g.Nodes[nodeID]; ok {
		n.DeepScanned = true
	}
}

// AddProbe adds a probe result to a node.
func (g *Graph) AddProbe(nodeID string, probe ProbeResult) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if n, ok := g.Nodes[nodeID]; ok {
		n.Probes = append(n.Probes, probe)
	}
}

// UpdateProbe updates the last probe of a given type on a node.
func (g *Graph) UpdateProbe(nodeID string, probeType string, probe ProbeResult) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if n, ok := g.Nodes[nodeID]; ok {
		for i := len(n.Probes) - 1; i >= 0; i-- {
			if n.Probes[i].Type == probeType {
				n.Probes[i] = probe
				return
			}
		}
		n.Probes = append(n.Probes, probe)
	}
}

// NodeCount returns the number of nodes.
func (g *Graph) NodeCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.Nodes)
}

// HasNode checks if a node exists by ID.
func (g *Graph) HasNode(id string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	_, ok := g.Nodes[id]
	return ok
}

// Snapshot returns a consistent snapshot of all nodes and edges for export.
func (g *Graph) Snapshot() (map[string]*Node, []Edge) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	nodes := make(map[string]*Node, len(g.Nodes))
	for id, n := range g.Nodes {
		cp := *n
		cp.Probes = make([]ProbeResult, len(n.Probes))
		copy(cp.Probes, n.Probes)
		cp.Children = make([]string, len(n.Children))
		copy(cp.Children, n.Children)
		nodes[id] = &cp
	}
	edges := make([]Edge, len(g.Edges))
	copy(edges, g.Edges)
	return nodes, edges
}

// NodeID generates a deterministic ID for a node.
func NodeID(nodeType NodeType, address string) string {
	return fmt.Sprintf("%s:%s", nodeType, address)
}
