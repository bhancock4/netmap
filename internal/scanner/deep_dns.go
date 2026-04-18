package scanner

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/bhancock4/netmap/internal/model"
)

// DNSDeepProbe performs detailed DNS analysis: reverse DNS, SPF, DMARC, DNSSEC indicators.
func DNSDeepProbe(ctx context.Context, graph *model.Graph, nodeID string, target string) {
	probe := model.ProbeResult{
		Type:   "dns_deep",
		Status: model.ProbeStatusRunning,
		Data:   make(map[string]string),
	}
	graph.UpdateProbe(nodeID, "dns_deep", probe)

	resolver := &net.Resolver{}
	start := time.Now()

	// Reverse DNS on resolved IPs
	ips, err := resolver.LookupHost(ctx, target)
	if err == nil {
		reverseResults := []string{}
		for _, ip := range ips {
			names, err := resolver.LookupAddr(ctx, ip)
			if err == nil && len(names) > 0 {
				for _, name := range names {
					reverseResults = append(reverseResults, fmt.Sprintf("%s → %s", ip, name))
				}
			} else {
				reverseResults = append(reverseResults, fmt.Sprintf("%s → (no PTR)", ip))
			}
		}
		probe.Data["reverse_dns"] = strings.Join(reverseResults, "; ")
	}

	// SPF record (TXT record at root domain)
	txts, err := resolver.LookupTXT(ctx, target)
	if err == nil {
		for _, txt := range txts {
			if strings.HasPrefix(txt, "v=spf1") {
				probe.Data["spf"] = txt
				// Analyze SPF
				if strings.Contains(txt, "-all") {
					probe.Data["spf_policy"] = "strict (hard fail)"
				} else if strings.Contains(txt, "~all") {
					probe.Data["spf_policy"] = "soft fail"
				} else if strings.Contains(txt, "?all") {
					probe.Data["spf_policy"] = "neutral"
				} else if strings.Contains(txt, "+all") {
					probe.Data["spf_policy"] = "permissive (DANGEROUS)"
				}
				break
			}
		}
		if _, ok := probe.Data["spf"]; !ok {
			probe.Data["spf"] = "not configured"
		}
	}

	// DMARC record
	dmarcTxts, err := resolver.LookupTXT(ctx, "_dmarc."+target)
	if err == nil {
		for _, txt := range dmarcTxts {
			if strings.HasPrefix(txt, "v=DMARC1") {
				probe.Data["dmarc"] = txt
				// Parse policy
				for _, part := range strings.Split(txt, ";") {
					part = strings.TrimSpace(part)
					if strings.HasPrefix(part, "p=") {
						policy := strings.TrimPrefix(part, "p=")
						switch policy {
						case "reject":
							probe.Data["dmarc_policy"] = "reject (strict)"
						case "quarantine":
							probe.Data["dmarc_policy"] = "quarantine"
						case "none":
							probe.Data["dmarc_policy"] = "none (monitoring only)"
						}
					}
				}
				break
			}
		}
		if _, ok := probe.Data["dmarc"]; !ok {
			probe.Data["dmarc"] = "not configured"
		}
	}

	// DKIM selector check (common selectors)
	commonSelectors := []string{"default", "google", "selector1", "selector2", "k1", "mail", "smtp"}
	dkimFound := []string{}
	for _, sel := range commonSelectors {
		dkimTxts, err := resolver.LookupTXT(ctx, sel+"._domainkey."+target)
		if err == nil && len(dkimTxts) > 0 {
			for _, txt := range dkimTxts {
				if strings.Contains(txt, "v=DKIM1") || strings.Contains(txt, "p=") {
					dkimFound = append(dkimFound, sel)
					break
				}
			}
		}
	}
	if len(dkimFound) > 0 {
		probe.Data["dkim_selectors"] = strings.Join(dkimFound, ", ")
	} else {
		probe.Data["dkim_selectors"] = "none found (checked common selectors)"
	}

	// CAA records
	// Go's net package doesn't support CAA directly, but we can note that
	probe.Data["note"] = "CAA/DNSSEC require dig; basic DNS deep analysis complete"

	// SRV records for common services
	srvServices := []struct {
		service string
		label   string
	}{
		{"_sip._tcp", "SIP"},
		{"_xmpp-server._tcp", "XMPP"},
		{"_autodiscover._tcp", "Autodiscover"},
		{"_imaps._tcp", "IMAPS"},
	}

	srvFound := []string{}
	for _, svc := range srvServices {
		_, addrs, err := resolver.LookupSRV(ctx, "", "", svc.service+"."+target)
		if err == nil && len(addrs) > 0 {
			for _, addr := range addrs {
				srvFound = append(srvFound, fmt.Sprintf("%s:%s:%d", svc.label, addr.Target, addr.Port))
			}
		}
	}
	if len(srvFound) > 0 {
		probe.Data["srv_records"] = strings.Join(srvFound, "; ")
	}

	probe.Latency = time.Since(start)
	probe.Status = model.ProbeStatusSuccess
	graph.UpdateProbe(nodeID, "dns_deep", probe)
}
