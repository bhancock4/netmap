package scanner

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/bhancock4/netmap/internal/model"
)

// DNSProbe performs DNS lookups for a target hostname.
func DNSProbe(ctx context.Context, graph *model.Graph, nodeID string, target string) {
	probe := model.ProbeResult{
		Type:   "dns",
		Status: model.ProbeStatusRunning,
		Data:   make(map[string]string),
	}
	graph.UpdateProbe(nodeID, "dns", probe)

	resolver := &net.Resolver{}
	start := time.Now()

	// A records
	ips, err := resolver.LookupHost(ctx, target)
	probe.Latency = time.Since(start)

	if err != nil {
		probe.Status = model.ProbeStatusFailed
		probe.Error = err.Error()
		graph.UpdateProbe(nodeID, "dns", probe)
		return
	}

	probe.Status = model.ProbeStatusSuccess
	probe.Data["a_records"] = strings.Join(ips, ", ")

	// Add IP nodes to the graph
	for i, ip := range ips {
		if i >= 5 { // cap at 5 IPs
			break
		}
		ipNodeID := model.NodeID(model.NodeTypeIP, ip)
		ipNode := &model.Node{
			ID:      ipNodeID,
			Label:   ip,
			Type:    model.NodeTypeIP,
			Address: ip,
			Depth:   1,
			Parent:  nodeID,
		}
		graph.AddNode(ipNode)
		graph.AddEdge(model.Edge{
			From:  nodeID,
			To:    ipNodeID,
			Type:  model.EdgeTypeDNS,
			Label: "A",
		})
	}

	// MX records
	mxs, err := resolver.LookupMX(ctx, target)
	if err == nil && len(mxs) > 0 {
		mxList := make([]string, 0, len(mxs))
		for _, mx := range mxs {
			mxList = append(mxList, fmt.Sprintf("%s (pri %d)", mx.Host, mx.Pref))
		}
		probe.Data["mx_records"] = strings.Join(mxList, ", ")
	}

	// NS records
	nss, err := resolver.LookupNS(ctx, target)
	if err == nil && len(nss) > 0 {
		nsList := make([]string, 0, len(nss))
		for _, ns := range nss {
			nsList = append(nsList, ns.Host)
		}
		probe.Data["ns_records"] = strings.Join(nsList, ", ")
	}

	// TXT records
	txts, err := resolver.LookupTXT(ctx, target)
	if err == nil && len(txts) > 0 {
		probe.Data["txt_records"] = strings.Join(txts, "; ")
	}

	// CNAME
	cname, err := resolver.LookupCNAME(ctx, target)
	if err == nil && cname != "" && strings.TrimSuffix(cname, ".") != target {
		probe.Data["cname"] = cname
	}

	graph.UpdateProbe(nodeID, "dns", probe)
}
