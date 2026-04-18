package scanner

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/bhancock4/netmap/internal/model"
)

// TLSProbe inspects the TLS certificate of a target host.
func TLSProbe(ctx context.Context, graph *model.Graph, nodeID string, host string) {
	probe := model.ProbeResult{
		Type:   "tls",
		Status: model.ProbeStatusRunning,
		Data:   make(map[string]string),
	}
	graph.UpdateProbe(nodeID, "tls", probe)

	dialer := &net.Dialer{Timeout: 5 * time.Second}
	start := time.Now()

	conn, err := tls.DialWithDialer(dialer, "tcp", host+":443", &tls.Config{
		InsecureSkipVerify: false,
	})
	probe.Latency = time.Since(start)

	if err != nil {
		// Try with insecure to still get cert info
		conn, err = tls.DialWithDialer(dialer, "tcp", host+":443", &tls.Config{
			InsecureSkipVerify: true,
		})
		if err != nil {
			probe.Status = model.ProbeStatusFailed
			probe.Error = err.Error()
			graph.UpdateProbe(nodeID, "tls", probe)
			return
		}
		probe.Data["verified"] = "false"
	} else {
		probe.Data["verified"] = "true"
	}
	defer conn.Close()

	state := conn.ConnectionState()
	probe.Data["tls_version"] = tlsVersionString(state.Version)
	probe.Data["cipher_suite"] = tls.CipherSuiteName(state.CipherSuite)

	if len(state.PeerCertificates) > 0 {
		cert := state.PeerCertificates[0]
		probe.Data["subject"] = cert.Subject.CommonName
		probe.Data["issuer"] = cert.Issuer.CommonName
		probe.Data["not_before"] = cert.NotBefore.Format("2006-01-02")
		probe.Data["not_after"] = cert.NotAfter.Format("2006-01-02")

		if time.Now().After(cert.NotAfter) {
			probe.Data["expired"] = "true"
		}

		// SANs — these can reveal related domains
		if len(cert.DNSNames) > 0 {
			probe.Data["sans"] = strings.Join(cert.DNSNames, ", ")

			// Add SAN domains as linked nodes (up to breadth limit)
			for i, san := range cert.DNSNames {
				if i >= 5 {
					break
				}
				san = strings.TrimPrefix(san, "*.")
				if san == host {
					continue
				}
				sanNodeID := model.NodeID(model.NodeTypeHost, san)
				if graph.HasNode(sanNodeID) {
					continue
				}
				sanNode := &model.Node{
					ID:      sanNodeID,
					Label:   san,
					Type:    model.NodeTypeHost,
					Address: san,
					Depth:   2,
					Parent:  nodeID,
				}
				graph.AddNode(sanNode)
				graph.AddEdge(model.Edge{
					From:  nodeID,
					To:    sanNodeID,
					Type:  model.EdgeTypeCert,
					Label: "SAN",
				})
			}
		}
	}

	probe.Status = model.ProbeStatusSuccess
	graph.UpdateProbe(nodeID, "tls", probe)
}

func tlsVersionString(v uint16) string {
	switch v {
	case tls.VersionTLS10:
		return "TLS 1.0"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS13:
		return "TLS 1.3"
	default:
		return fmt.Sprintf("0x%04x", v)
	}
}
