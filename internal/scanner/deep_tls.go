package scanner

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/bhancock4/netmap/internal/model"
)

// TLSDeepProbe performs a detailed TLS analysis.
func TLSDeepProbe(ctx context.Context, graph *model.Graph, nodeID string, host string) {
	probe := model.ProbeResult{
		Type:   "tls_deep",
		Status: model.ProbeStatusRunning,
		Data:   make(map[string]string),
	}
	graph.UpdateProbe(nodeID, "tls_deep", probe)

	start := time.Now()

	// Test each TLS version
	versions := []struct {
		version uint16
		name    string
	}{
		{tls.VersionTLS10, "TLS 1.0"},
		{tls.VersionTLS11, "TLS 1.1"},
		{tls.VersionTLS12, "TLS 1.2"},
		{tls.VersionTLS13, "TLS 1.3"},
	}

	supported := []string{}
	deprecated := []string{}

	for _, v := range versions {
		dialer := &net.Dialer{Timeout: 3 * time.Second}
		conn, err := tls.DialWithDialer(dialer, "tcp", host+":443", &tls.Config{
			InsecureSkipVerify: true,
			MinVersion:         v.version,
			MaxVersion:         v.version,
		})
		if err == nil {
			conn.Close()
			supported = append(supported, v.name)
			if v.version <= tls.VersionTLS11 {
				deprecated = append(deprecated, v.name)
			}
		}
	}

	probe.Data["supported_versions"] = strings.Join(supported, ", ")
	if len(deprecated) > 0 {
		probe.Data["deprecated_versions"] = strings.Join(deprecated, ", ")
	}

	// Get full cert chain
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", host+":443", &tls.Config{
		InsecureSkipVerify: true,
	})
	if err != nil {
		probe.Status = model.ProbeStatusFailed
		probe.Error = err.Error()
		probe.Latency = time.Since(start)
		graph.UpdateProbe(nodeID, "tls_deep", probe)
		return
	}
	defer conn.Close()

	state := conn.ConnectionState()

	// Cipher suite info
	probe.Data["negotiated_cipher"] = tls.CipherSuiteName(state.CipherSuite)

	// Check if cipher is considered weak
	for _, weak := range tls.InsecureCipherSuites() {
		if weak.ID == state.CipherSuite {
			probe.Data["cipher_warning"] = "INSECURE cipher suite"
			break
		}
	}

	// Certificate chain
	if len(state.PeerCertificates) > 0 {
		chain := []string{}
		for i, cert := range state.PeerCertificates {
			entry := fmt.Sprintf("[%d] %s", i, cert.Subject.CommonName)
			if cert.IsCA {
				entry += " (CA)"
			}
			chain = append(chain, entry)
		}
		probe.Data["cert_chain"] = strings.Join(chain, " → ")
		probe.Data["chain_length"] = fmt.Sprintf("%d", len(state.PeerCertificates))

		// Leaf cert details
		leaf := state.PeerCertificates[0]
		probe.Data["signature_algo"] = leaf.SignatureAlgorithm.String()
		probe.Data["public_key_algo"] = leaf.PublicKeyAlgorithm.String()

		// Key size
		switch pub := leaf.PublicKey.(type) {
		case interface{ Size() int }:
			probe.Data["key_size"] = fmt.Sprintf("%d bits", pub.Size()*8)
		}

		// Check key usage
		usages := []string{}
		if leaf.KeyUsage&x509.KeyUsageDigitalSignature != 0 {
			usages = append(usages, "DigitalSignature")
		}
		if leaf.KeyUsage&x509.KeyUsageKeyEncipherment != 0 {
			usages = append(usages, "KeyEncipherment")
		}
		if len(usages) > 0 {
			probe.Data["key_usage"] = strings.Join(usages, ", ")
		}

		// Extended key usage
		extUsages := []string{}
		for _, u := range leaf.ExtKeyUsage {
			switch u {
			case x509.ExtKeyUsageServerAuth:
				extUsages = append(extUsages, "ServerAuth")
			case x509.ExtKeyUsageClientAuth:
				extUsages = append(extUsages, "ClientAuth")
			}
		}
		if len(extUsages) > 0 {
			probe.Data["ext_key_usage"] = strings.Join(extUsages, ", ")
		}

		// Days until expiry
		daysLeft := int(time.Until(leaf.NotAfter).Hours() / 24)
		probe.Data["days_until_expiry"] = fmt.Sprintf("%d", daysLeft)
		if daysLeft < 30 {
			probe.Data["expiry_warning"] = "Certificate expires soon!"
		}
		if daysLeft < 0 {
			probe.Data["expiry_warning"] = "Certificate EXPIRED!"
		}
	}

	probe.Latency = time.Since(start)
	probe.Status = model.ProbeStatusSuccess
	graph.UpdateProbe(nodeID, "tls_deep", probe)
}
