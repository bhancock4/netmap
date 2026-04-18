# netmap

Network topology mapper with a rich terminal UI. Give it a hostname or IP — it discovers and visualizes the network neighborhood using DNS, ping, traceroute, WHOIS, TLS, and HTTP inspection.

Built with Go, [Bubble Tea](https://github.com/charmbracelet/bubbletea), and [Lip Gloss](https://github.com/charmbracelet/lipgloss).

## Install

```bash
# Homebrew (macOS / Linux)
brew install bhancock4/tap/netmap

# Go
go install github.com/bhancock4/netmap@latest

# Binary (GitHub Releases)
gh release download v1.0.0 --repo bhancock4/netmap
```

## Quick Start

```bash
netmap example.com
```

That's it. The TUI launches, probes fan out concurrently, and nodes appear in real-time as they're discovered. Press `?` for a full keybinding reference.

## Features

### Interactive Tree View

The main interface is a split-pane layout: a navigable tree of discovered nodes on the left, detailed probe results on the right. Nodes are color-coded by type and collapsible.

| Icon | Type | Color |
|------|------|-------|
| ◆ | Hostname | Cyan |
| ● | IP Address | Green |
| ◇ | Router (traceroute hop) | Amber |
| ⬢ | Deep Scanned | Magenta |

### Visual Network Path

Press `v` on any node to see the route from your machine to that node, rendered as connected device boxes with latency labels. Arrow keys traverse the path.

```
╔══════╗        ╭────────╮        ╭────────╮        ╔══════════╗
║  ⌂   ║──────▶│   ◇    │──────▶│   ◇    │──────▶║    ◆     ║
║ YOU  ║        │10.0.0.1│        │72.14.2.│        ║google.com║
╚══════╝        ╰────────╯        ╰────────╯        ╚══════════╝
                   2ms               15ms               45ms
```

### Standard Probes

Run automatically on all discovered nodes:

- **DNS** — A, MX, NS, TXT, CNAME records
- **Ping** — ICMP echo with min/avg/max latency
- **Traceroute** — hop-by-hop route discovery, adds router nodes to the graph
- **TLS** — certificate subject, issuer, SANs, expiry date
- **HTTP** — status code, server headers, discovers linked hosts
- **WHOIS** — registrar, organization, country, creation/expiry dates

### Deep Scan

Press `d` on any node to trigger an in-depth analysis:

- **Port Scan** — top 25 common ports via TCP connect (SSH, HTTP, MySQL, Redis, etc.)
- **Banner Grab** — reads service identification strings from open ports
- **TLS Deep** — protocol version support (TLS 1.0–1.3), weak cipher detection, full cert chain, days until expiry
- **DNS Deep** — reverse DNS (PTR), SPF/DMARC/DKIM analysis, SRV record discovery
- **HTTP Security** — 7 security headers scored, cookie flags audit, robots.txt/sitemap discovery

All deep scan probes use standard TCP connect — the same mechanism your browser uses. No raw packets, no SYN scans.

## Keybindings

| Key | Action |
|-----|--------|
| `↑/k` `↓/j` | Navigate tree / scroll detail (path view) |
| `enter` / `space` | Expand/collapse node |
| `d` | Deep scan selected node |
| `v` | Toggle visual path view |
| `←/h` `→/l` | Traverse path (in path view) |
| `n` | Scan a new target |
| `r` | Rescan current target |
| `s` | Save results to file |
| `esc` | Abort running scan |
| `tab` / `shift+tab` | Scroll detail panel |
| `?` | Help overlay |
| `q` / `ctrl+c` | Quit |

## CLI Flags

```
netmap <target> [flags]

Flags:
  -d, --depth int      Crawl depth, 1-5 (default 3)
  -b, --breadth int    Max child nodes per parent (default 10)
  -t, --timeout dur    Scan timeout (default 5m)
  -o, --output file    Write results to file
  -f, --format fmt     Export format: yaml (default) or json
      --headless       No TUI, output to stdout
  -h, --help           Help
```

## Export & Scripting

```bash
# Save results from TUI
# Press 's' during a scan — writes netmap_<target>_<timestamp>.yaml

# Headless YAML to stdout
netmap cloudflare.com --headless

# Headless JSON piped to jq
netmap google.com --headless -f json | jq '.nodes[] | select(.type == "IP") | .label'

# Auto-save to file with TUI
netmap github.com -o scan.yaml

# Cron job
netmap myserver.com --headless -t 2m -o /var/log/netmap/$(date +%F).yaml
```

## Man Page

```bash
netmap man
```

Renders a full themed manual in the terminal with the sonar color scheme.

## Architecture

```
netmap/
├── main.go
├── cmd/
│   ├── root.go              # CLI entry, cobra flags
│   └── man.go               # Themed man page
├── internal/
│   ├── model/
│   │   ├── graph.go          # Thread-safe graph (nodes, edges, probes)
│   │   └── config.go         # Validated scan config
│   ├── scanner/
│   │   ├── scanner.go        # Orchestrator, concurrency semaphore, event bus
│   │   ├── dns.go            # DNS probe
│   │   ├── ping.go           # Ping probe
│   │   ├── traceroute.go     # Traceroute probe
│   │   ├── tls.go            # TLS probe
│   │   ├── http.go           # HTTP probe
│   │   ├── whois.go          # WHOIS probe
│   │   ├── portscan.go       # Port scan + banner grab
│   │   ├── deep_tls.go       # TLS deep analysis
│   │   ├── deep_dns.go       # DNS deep analysis (SPF/DMARC/DKIM)
│   │   └── deep_http.go      # HTTP security audit
│   ├── export/
│   │   └── export.go         # YAML/JSON report builder
│   └── ui/
│       ├── app.go            # Bubble Tea model, views, keybindings
│       ├── pathview.go        # Visual network path renderer
│       └── theme.go          # Sonar color palette, icons, animations
├── .github/workflows/ci.yml  # Test + GoReleaser release pipeline
├── .goreleaser.yaml           # Cross-compile config + Homebrew tap
└── Dockerfile
```

**Concurrency model:** Each probe runs in its own goroutine, gated by a semaphore (max 20 concurrent). The scanner pushes events through a buffered channel to the TUI. The graph is protected by `sync.RWMutex` and `GetNode()` returns copies to avoid read races.

**Color scheme:** "Sonar" — cyan/teal primary, electric green for live signals, amber for routers/warnings, magenta for deep scan, red for failures, on a dark background.

## Development

```bash
# Build
go build -o netmap .

# Test (with race detector)
go test -race -v ./...

# Install locally
go install

# Build with race detector for debugging
go build -race -o netmap_race .
```

## License

MIT
