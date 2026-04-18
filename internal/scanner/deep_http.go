package scanner

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/bhancock4/netmap/internal/model"
)

// HTTPDeepProbe performs a security-focused HTTP analysis.
func HTTPDeepProbe(ctx context.Context, graph *model.Graph, nodeID string, host string) {
	probe := model.ProbeResult{
		Type:   "http_deep",
		Status: model.ProbeStatusRunning,
		Data:   make(map[string]string),
	}
	graph.UpdateProbe(nodeID, "http_deep", probe)

	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	start := time.Now()

	// Main request
	url := "https://" + host
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		probe.Status = model.ProbeStatusFailed
		probe.Error = err.Error()
		graph.UpdateProbe(nodeID, "http_deep", probe)
		return
	}
	req.Header.Set("User-Agent", "netmap/1.0 (network mapping tool)")

	resp, err := client.Do(req)
	if err != nil {
		probe.Status = model.ProbeStatusFailed
		probe.Error = err.Error()
		probe.Latency = time.Since(start)
		graph.UpdateProbe(nodeID, "http_deep", probe)
		return
	}
	defer resp.Body.Close()
	io.ReadAll(io.LimitReader(resp.Body, 1024)) // drain body

	// Security headers audit
	securityHeaders := map[string]struct {
		header string
		good   string
		bad    string
	}{
		"hsts":            {header: "Strict-Transport-Security", good: "present", bad: "MISSING — no HSTS"},
		"csp":             {header: "Content-Security-Policy", good: "present", bad: "MISSING — no CSP"},
		"x_frame":         {header: "X-Frame-Options", good: "present", bad: "MISSING — clickjacking risk"},
		"x_content_type":  {header: "X-Content-Type-Options", good: "present", bad: "MISSING — MIME sniffing risk"},
		"x_xss":           {header: "X-XSS-Protection", good: "present", bad: "MISSING"},
		"referrer_policy": {header: "Referrer-Policy", good: "present", bad: "MISSING"},
		"permissions":     {header: "Permissions-Policy", good: "present", bad: "MISSING"},
	}

	score := 0
	total := len(securityHeaders)

	for key, sh := range securityHeaders {
		val := resp.Header.Get(sh.header)
		if val != "" {
			probe.Data[key] = val
			score++
		} else {
			probe.Data[key] = sh.bad
		}
	}

	probe.Data["security_score"] = fmt.Sprintf("%d/%d headers present", score, total)

	// HSTS details
	if hsts := resp.Header.Get("Strict-Transport-Security"); hsts != "" {
		if strings.Contains(hsts, "includeSubDomains") {
			probe.Data["hsts_subdomains"] = "yes"
		}
		if strings.Contains(hsts, "preload") {
			probe.Data["hsts_preload"] = "yes"
		}
	}

	// Cookie analysis
	cookies := resp.Cookies()
	if len(cookies) > 0 {
		cookieInfo := []string{}
		for _, c := range cookies {
			flags := []string{c.Name}
			if c.Secure {
				flags = append(flags, "Secure")
			} else {
				flags = append(flags, "NOT-Secure")
			}
			if c.HttpOnly {
				flags = append(flags, "HttpOnly")
			} else {
				flags = append(flags, "NOT-HttpOnly")
			}
			if c.SameSite != 0 {
				switch c.SameSite {
				case http.SameSiteStrictMode:
					flags = append(flags, "SameSite=Strict")
				case http.SameSiteLaxMode:
					flags = append(flags, "SameSite=Lax")
				case http.SameSiteNoneMode:
					flags = append(flags, "SameSite=None")
				}
			}
			cookieInfo = append(cookieInfo, strings.Join(flags, " "))
		}
		probe.Data["cookies"] = strings.Join(cookieInfo, "; ")
	}

	// Redirect chain
	if resp.Request.URL.String() != url {
		probe.Data["final_url"] = resp.Request.URL.String()
		probe.Data["redirected"] = "yes"
	}

	// Check robots.txt
	robotsReq, _ := http.NewRequestWithContext(ctx, "GET", "https://"+host+"/robots.txt", nil)
	robotsReq.Header.Set("User-Agent", "netmap/1.0")
	robotsResp, err := client.Do(robotsReq)
	if err == nil {
		defer robotsResp.Body.Close()
		if robotsResp.StatusCode == 200 {
			body, _ := io.ReadAll(io.LimitReader(robotsResp.Body, 2048))
			lines := strings.Split(string(body), "\n")
			disallowed := []string{}
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(strings.ToLower(line), "disallow:") {
					path := strings.TrimSpace(strings.TrimPrefix(line, "Disallow:"))
					path = strings.TrimSpace(strings.TrimPrefix(path, "disallow:"))
					if path != "" {
						disallowed = append(disallowed, path)
					}
				}
			}
			probe.Data["robots_txt"] = "present"
			if len(disallowed) > 0 {
				if len(disallowed) > 10 {
					disallowed = disallowed[:10]
					disallowed = append(disallowed, "...")
				}
				probe.Data["robots_disallowed"] = strings.Join(disallowed, ", ")
			}
		} else {
			probe.Data["robots_txt"] = "not found"
		}
	}

	// Check sitemap.xml
	sitemapReq, _ := http.NewRequestWithContext(ctx, "GET", "https://"+host+"/sitemap.xml", nil)
	sitemapReq.Header.Set("User-Agent", "netmap/1.0")
	sitemapResp, err := client.Do(sitemapReq)
	if err == nil {
		defer sitemapResp.Body.Close()
		if sitemapResp.StatusCode == 200 {
			probe.Data["sitemap"] = "present"
		} else {
			probe.Data["sitemap"] = "not found"
		}
	}

	probe.Latency = time.Since(start)
	probe.Status = model.ProbeStatusSuccess
	graph.UpdateProbe(nodeID, "http_deep", probe)
}
