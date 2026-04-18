# CLAUDE.md

## Project overview

netmap is a CLI network topology mapper with a rich terminal UI. It takes a hostname or IP, runs concurrent network probes (DNS, ping, traceroute, WHOIS, TLS, HTTP), and displays an interactive map of discovered nodes. Built with Go using Bubble Tea for the TUI.

## Build & run

```bash
go build -o netmap .
go install
go test -race -v ./...
```

Requires Go 1.23+. No other build dependencies.

## Architecture

- `cmd/` — CLI entry points (root command + man page). Uses cobra for flag parsing.
- `internal/model/` — Core data structures. `Graph` is the central type: thread-safe via `sync.RWMutex`, `GetNode()` returns copies to avoid read races. `Config` has `Validate()` with clamping.
- `internal/scanner/` — Network probes. Each probe is a standalone function with signature `func(ctx, graph, nodeID, address)`. The orchestrator (`scanner.go`) runs probes concurrently with a semaphore (max 20 goroutines). Events flow to the UI via a buffered channel.
- `internal/export/` — YAML/JSON report builder. Uses `Graph.Snapshot()` for consistent reads.
- `internal/ui/` — Bubble Tea TUI. Three view modes: tree view, path view, help overlay. Theme is in `theme.go` (sonar palette).

## Key conventions

- All graph mutations go through `Graph` methods which hold the lock. Never mutate nodes directly from scanner code.
- Event types are constants in `model/graph.go` (`EventScanDone`, `EventDeepDone`, etc.), not magic strings.
- Probes must respect the passed `context.Context` for cancellation and timeouts.
- The UI tracks cursor by node ID (`selectedID`), not by index, so tree rebuilds don't cause cursor jumps.
- Deep scan probes (port scan, banner grab, TLS/DNS/HTTP deep) are separate from standard probes and gated behind the `d` key or programmatic `DeepScan()` call.

## Testing

Tests live alongside their packages (`_test.go` files). Run with `-race` flag always. The model and export packages have full coverage. Scanner probes hit real network and are not unit-tested — they're tested manually.

## Release process

Tag and push — CI does the rest:

```bash
git tag v1.x.x
git push origin v1.x.x
```

GitHub Actions runs tests, GoReleaser builds binaries for 6 platforms (macOS/Linux/Windows x amd64/arm64), publishes a GitHub release, and pushes a Homebrew formula to `bhancock4/homebrew-tap`.

## Things to know

- Traceroute may need elevated privileges on some systems.
- The `.gitignore` excludes `*.yaml` (scan output files) but not `.goreleaser.yaml`.
- The `--headless` flag skips the TUI entirely for scripting/piping.
- The hand-rolled YAML serializer in `export/` avoids a dependency but is simplistic — complex strings may not round-trip perfectly.
