package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/bhancock4/netmap/internal/export"
	"github.com/bhancock4/netmap/internal/model"
	"github.com/bhancock4/netmap/internal/scanner"
	"github.com/bhancock4/netmap/internal/ui"
)

var loadCmd = &cobra.Command{
	Use:   "load <name-or-file>",
	Short: "Load a saved scan session into the TUI",
	Long: `Load a previously saved netmap scan (YAML or JSON) and open it
in the interactive TUI for exploration, without re-scanning.

You can pass a session name (looks in ~/.netmap/sessions/) or a file path.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := resolveSessionPath(args[0])

		report, err := export.LoadReport(path)
		if err != nil {
			return fmt.Errorf("loading session: %w", err)
		}

		// Rebuild graph from report
		graph := rebuildGraph(report)

		// Create a dummy scanner with the loaded graph
		cfg := model.Config{Target: report.Target}
		s := scanner.New(cfg)
		s.Graph = graph
		close(s.Events) // no live events

		m := ui.New(s)
		m.SetCancel(func() {}) // no-op cancel
		p := tea.NewProgram(m, tea.WithAltScreen())

		if _, err := p.Run(); err != nil {
			return fmt.Errorf("TUI error: %w", err)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(loadCmd)
}

// sessionsDir returns the path to the sessions directory, creating it if needed.
func sessionsDir() string {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".netmap", "sessions")
	os.MkdirAll(dir, 0755)
	return dir
}

// resolveSessionPath checks if the arg is a file path or a session name.
func resolveSessionPath(nameOrPath string) string {
	// If it looks like a file path, use it directly
	if strings.Contains(nameOrPath, "/") || strings.Contains(nameOrPath, ".") {
		return nameOrPath
	}
	// Otherwise look in sessions dir
	dir := sessionsDir()
	// Try yaml first, then json
	yamlPath := filepath.Join(dir, nameOrPath+".yaml")
	if _, err := os.Stat(yamlPath); err == nil {
		return yamlPath
	}
	jsonPath := filepath.Join(dir, nameOrPath+".json")
	if _, err := os.Stat(jsonPath); err == nil {
		return jsonPath
	}
	return yamlPath // will error on load
}

// rebuildGraph reconstructs a Graph from a report.
func rebuildGraph(report export.Report) *model.Graph {
	graph := model.NewGraph()

	// Map to track node types
	typeMap := map[string]model.NodeType{
		"HOST":   model.NodeTypeHost,
		"IP":     model.NodeTypeIP,
		"ROUTER": model.NodeTypeRouter,
	}

	// First pass: add all nodes
	for _, rn := range report.Nodes {
		nt := typeMap[rn.Type]
		node := &model.Node{
			ID:      rn.ID,
			Label:   rn.Label,
			Type:    nt,
			Address: rn.Address,
			Depth:   rn.Depth,
		}
		graph.AddNode(node)
	}

	// Second pass: add probes
	for _, rn := range report.Nodes {
		for _, rp := range rn.Probes {
			probe := model.ProbeResult{
				Type:   rp.Type,
				Status: parseStatus(rp.Status),
				Data:   rp.Data,
				Error:  rp.Error,
			}
			graph.AddProbe(rn.ID, probe)
		}
	}

	// Add edges and wire parent/children
	for _, re := range report.Edges {
		edgeTypeMap := map[string]model.EdgeType{
			"DNS":   model.EdgeTypeDNS,
			"ROUTE": model.EdgeTypeRoute,
			"LINK":  model.EdgeTypeLink,
			"CERT":  model.EdgeTypeCert,
		}
		graph.AddEdge(model.Edge{
			From:  re.From,
			To:    re.To,
			Type:  edgeTypeMap[re.Type],
			Label: re.Label,
		})
	}

	return graph
}

func parseStatus(s string) model.ProbeStatus {
	switch s {
	case "ok":
		return model.ProbeStatusSuccess
	case "fail":
		return model.ProbeStatusFailed
	case "timeout":
		return model.ProbeStatusTimeout
	case "running":
		return model.ProbeStatusRunning
	default:
		return model.ProbeStatusPending
	}
}
