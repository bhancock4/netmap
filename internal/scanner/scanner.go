package scanner

import (
	"context"
	"net"
	"strings"
	"sync"

	"github.com/bhancock4/netmap/internal/model"
)

// MaxConcurrency limits the number of concurrent probe goroutines.
const MaxConcurrency = 20

// Event represents a discovery event sent to the UI.
type Event struct {
	Type    string
	NodeID  string
	Message string
}

// Scanner orchestrates network discovery.
type Scanner struct {
	Config model.Config
	Graph  *model.Graph
	Events chan Event
	sem    chan struct{} // concurrency semaphore
}

// New creates a new scanner.
func New(cfg model.Config) *Scanner {
	return &Scanner{
		Config: cfg,
		Graph:  model.NewGraph(),
		Events: make(chan Event, 256),
		sem:    make(chan struct{}, MaxConcurrency),
	}
}

// Run starts the scan. It blocks until complete.
func (s *Scanner) Run(ctx context.Context) {
	target := s.Config.Target

	// Check for CIDR notation (subnet scan)
	if strings.Contains(target, "/") {
		_, _, err := net.ParseCIDR(target)
		if err == nil {
			s.Config.Target = target
			SubnetScan(ctx, s, target)
			s.emit(model.EventScanDone, "", "Subnet scan complete")
			return
		}
	}

	isIP := net.ParseIP(target) != nil

	target = strings.TrimPrefix(target, "https://")
	target = strings.TrimPrefix(target, "http://")
	target = strings.Split(target, "/")[0]
	target = strings.Split(target, ":")[0]
	s.Config.Target = target

	rootType := model.NodeTypeHost
	if isIP {
		rootType = model.NodeTypeIP
	}
	rootID := model.NodeID(rootType, target)
	rootNode := &model.Node{
		ID:      rootID,
		Label:   target,
		Type:    rootType,
		Address: target,
		Depth:   0,
	}
	s.Graph.AddNode(rootNode)
	s.emit(model.EventNodeAdded, rootID, "Root target: "+target)

	s.probeNode(ctx, rootID, target, isIP, 0)

	s.emit(model.EventScanDone, rootID, "Scan complete")
}

// DeepScan runs an in-depth scan on a specific node.
func (s *Scanner) DeepScan(ctx context.Context, nodeID string) {
	node, ok := s.Graph.GetNode(nodeID)
	if !ok {
		return
	}

	address := node.Address
	isIP := node.Type == model.NodeTypeIP || node.Type == model.NodeTypeRouter

	s.Graph.SetDeepScanned(nodeID)

	s.emit(model.EventDeepStart, nodeID, "Deep scanning "+address)

	var wg sync.WaitGroup

	// Port scan then banner grab (sequential — banners need port results)
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.acquire(ctx)
		defer s.release()
		s.emit(model.EventProbeStart, nodeID, "Port scan")
		PortScanProbe(ctx, s.Graph, nodeID, address)
		s.emit(model.EventProbeDone, nodeID, "Port scan complete")

		s.emit(model.EventProbeStart, nodeID, "Banner grab")
		BannerGrabProbe(ctx, s.Graph, nodeID, address)
		s.emit(model.EventProbeDone, nodeID, "Banner grab complete")
	}()

	if !isIP {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.acquire(ctx)
			defer s.release()
			s.emit(model.EventProbeStart, nodeID, "TLS deep analysis")
			TLSDeepProbe(ctx, s.Graph, nodeID, address)
			s.emit(model.EventProbeDone, nodeID, "TLS deep complete")
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			s.acquire(ctx)
			defer s.release()
			s.emit(model.EventProbeStart, nodeID, "DNS deep analysis")
			DNSDeepProbe(ctx, s.Graph, nodeID, address)
			s.emit(model.EventProbeDone, nodeID, "DNS deep complete")
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			s.acquire(ctx)
			defer s.release()
			s.emit(model.EventProbeStart, nodeID, "HTTP security audit")
			HTTPDeepProbe(ctx, s.Graph, nodeID, address)
			s.emit(model.EventProbeDone, nodeID, "HTTP security audit complete")
		}()
	}

	wg.Wait()
	s.emit(model.EventDeepDone, nodeID, "Deep scan complete for "+address)
}

func (s *Scanner) probeNode(ctx context.Context, nodeID string, target string, isIP bool, depth int) {
	// Check context before starting
	if ctx.Err() != nil {
		return
	}

	var wg sync.WaitGroup

	if !isIP {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.acquire(ctx)
			defer s.release()
			s.emit(model.EventProbeStart, nodeID, "DNS lookup")
			DNSProbe(ctx, s.Graph, nodeID, target)
			s.emit(model.EventProbeDone, nodeID, "DNS complete")
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			s.acquire(ctx)
			defer s.release()
			s.emit(model.EventProbeStart, nodeID, "WHOIS lookup")
			WhoisProbe(ctx, s.Graph, nodeID, target)
			s.emit(model.EventProbeDone, nodeID, "WHOIS complete")
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			s.acquire(ctx)
			defer s.release()
			s.emit(model.EventProbeStart, nodeID, "TLS inspection")
			TLSProbe(ctx, s.Graph, nodeID, target)
			s.emit(model.EventProbeDone, nodeID, "TLS complete")
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			s.acquire(ctx)
			defer s.release()
			s.emit(model.EventProbeStart, nodeID, "HTTP inspection")
			HTTPProbe(ctx, s.Graph, nodeID, target, s.Config.Breadth)
			s.emit(model.EventProbeDone, nodeID, "HTTP complete")
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		s.acquire(ctx)
		defer s.release()
		s.emit(model.EventProbeStart, nodeID, "Ping")
		PingProbe(ctx, s.Graph, nodeID, target)
		s.emit(model.EventProbeDone, nodeID, "Ping complete")
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		s.acquire(ctx)
		defer s.release()
		s.emit(model.EventProbeStart, nodeID, "Traceroute")
		TracerouteProbe(ctx, s.Graph, nodeID, target)
		s.emit(model.EventProbeDone, nodeID, "Traceroute complete")
	}()

	wg.Wait()

	// Recurse into discovered child nodes if depth allows
	if depth < s.Config.Depth-1 {
		node, ok := s.Graph.GetNode(nodeID)
		if !ok {
			return
		}
		children := node.Children // already a copy from GetNode

		if len(children) > s.Config.Breadth {
			children = children[:s.Config.Breadth]
		}

		for _, childID := range children {
			if ctx.Err() != nil {
				return
			}
			child, ok := s.Graph.GetNode(childID)
			if !ok {
				continue
			}
			childIsIP := child.Type == model.NodeTypeIP || child.Type == model.NodeTypeRouter
			s.probeNode(ctx, childID, child.Address, childIsIP, depth+1)
		}
	}
}

// acquire blocks until a semaphore slot is available or context is cancelled.
func (s *Scanner) acquire(ctx context.Context) {
	select {
	case s.sem <- struct{}{}:
	case <-ctx.Done():
	}
}

// release frees a semaphore slot.
func (s *Scanner) release() {
	select {
	case <-s.sem:
	default:
	}
}

func (s *Scanner) emit(eventType, nodeID, message string) {
	// Recover from sending on a closed channel (can happen when
	// subnet sweep or deep scan reuses a scanner whose Run() already
	// closed the channel).
	defer func() { recover() }()

	select {
	case s.Events <- Event{Type: eventType, NodeID: nodeID, Message: message}:
	default:
	}
}
