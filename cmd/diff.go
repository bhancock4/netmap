package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/bhancock4/netmap/internal/export"
)

var (
	diffCyan  = lipgloss.NewStyle().Foreground(lipgloss.Color("#00E5FF")).Bold(true)
	diffGreen = lipgloss.NewStyle().Foreground(lipgloss.Color("#69F0AE"))
	diffRed   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5252"))
	diffAmber = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFB74D"))
	diffDim   = lipgloss.NewStyle().Foreground(lipgloss.Color("#616161"))
	diffWhite = lipgloss.NewStyle().Foreground(lipgloss.Color("#E0E0E0"))
	diffMag   = lipgloss.NewStyle().Foreground(lipgloss.Color("#E040FB"))
)

var diffCmd = &cobra.Command{
	Use:   "diff <old-scan> <new-scan>",
	Short: "Compare two scan results and show what changed",
	Long: `Compare two netmap scan files (YAML or JSON) and display
the differences: new nodes, removed nodes, changed probe results,
port changes, certificate rotations, and latency shifts.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		oldReport, err := export.LoadReport(args[0])
		if err != nil {
			return fmt.Errorf("loading old scan: %w", err)
		}
		newReport, err := export.LoadReport(args[1])
		if err != nil {
			return fmt.Errorf("loading new scan: %w", err)
		}

		diff := export.Diff(oldReport, newReport)
		printDiff(diff, args[0], args[1])
		return nil
	},
}

func init() {
	rootCmd.AddCommand(diffCmd)
}

func printDiff(d export.DiffResult, oldFile, newFile string) {
	divider := diffDim.Render(strings.Repeat("─", 60))

	// Header
	fmt.Println()
	fmt.Println(diffCyan.Render("  NETMAP DIFF"))
	fmt.Println(divider)
	fmt.Printf("  %s %s\n", diffDim.Render("Target:"), diffWhite.Render(d.Target))
	fmt.Printf("  %s %s\n", diffDim.Render("Old:"), diffWhite.Render(oldFile)+" "+diffDim.Render("("+d.OldScan+")"))
	fmt.Printf("  %s %s\n", diffDim.Render("New:"), diffWhite.Render(newFile)+" "+diffDim.Render("("+d.NewScan+")"))
	fmt.Println()

	// Summary box
	s := d.Summary
	total := s.NodesAdded + s.NodesRemoved + s.NodesChanged
	if total == 0 && s.EdgesAdded == 0 && s.EdgesRemoved == 0 {
		fmt.Println(diffGreen.Render("  ✓ No changes detected"))
		fmt.Println()
		return
	}

	fmt.Println(diffCyan.Render("  Summary"))
	fmt.Println(divider)
	if s.NodesAdded > 0 {
		fmt.Printf("  %s %d nodes added\n", diffGreen.Render("+"), s.NodesAdded)
	}
	if s.NodesRemoved > 0 {
		fmt.Printf("  %s %d nodes removed\n", diffRed.Render("-"), s.NodesRemoved)
	}
	if s.NodesChanged > 0 {
		fmt.Printf("  %s %d nodes changed\n", diffAmber.Render("~"), s.NodesChanged)
	}
	if s.EdgesAdded > 0 {
		fmt.Printf("  %s %d connections added\n", diffGreen.Render("+"), s.EdgesAdded)
	}
	if s.EdgesRemoved > 0 {
		fmt.Printf("  %s %d connections removed\n", diffRed.Render("-"), s.EdgesRemoved)
	}
	if len(s.PortsOpened) > 0 {
		fmt.Printf("  %s Ports opened: %s\n", diffMag.Render("!"), diffWhite.Render(strings.Join(s.PortsOpened, ", ")))
	}
	if len(s.PortsClosed) > 0 {
		fmt.Printf("  %s Ports closed: %s\n", diffMag.Render("!"), diffWhite.Render(strings.Join(s.PortsClosed, ", ")))
	}
	if len(s.CertsChanged) > 0 {
		// Deduplicate
		seen := make(map[string]bool)
		unique := []string{}
		for _, c := range s.CertsChanged {
			if !seen[c] {
				seen[c] = true
				unique = append(unique, c)
			}
		}
		fmt.Printf("  %s Certs changed: %s\n", diffAmber.Render("⚠"), diffWhite.Render(strings.Join(unique, ", ")))
	}
	fmt.Println()

	// Added nodes
	if len(d.AddedNodes) > 0 {
		fmt.Println(diffGreen.Render("  + Added Nodes"))
		fmt.Println(divider)
		for _, n := range d.AddedNodes {
			fmt.Printf("    %s %s %s\n",
				diffGreen.Render("+"),
				diffWhite.Render(n.Label),
				diffDim.Render("("+n.Type+" depth:"+fmt.Sprintf("%d", n.Depth)+")"),
			)
		}
		fmt.Println()
	}

	// Removed nodes
	if len(d.RemovedNodes) > 0 {
		fmt.Println(diffRed.Render("  - Removed Nodes"))
		fmt.Println(divider)
		for _, n := range d.RemovedNodes {
			fmt.Printf("    %s %s %s\n",
				diffRed.Render("-"),
				diffWhite.Render(n.Label),
				diffDim.Render("("+n.Type+" depth:"+fmt.Sprintf("%d", n.Depth)+")"),
			)
		}
		fmt.Println()
	}

	// Changed nodes
	if len(d.ChangedNodes) > 0 {
		fmt.Println(diffAmber.Render("  ~ Changed Nodes"))
		fmt.Println(divider)
		for _, nd := range d.ChangedNodes {
			fmt.Printf("    %s %s\n", diffAmber.Render("~"), diffCyan.Render(nd.Label))
			for _, c := range nd.Changes {
				probe := diffDim.Render("[" + c.Probe + "]")
				field := diffWhite.Render(c.Field)
				if c.OldValue == "" {
					fmt.Printf("      %s %s %s = %s\n",
						probe, field,
						diffGreen.Render("(new)"),
						diffGreen.Render(truncate(c.NewValue, 60)),
					)
				} else {
					fmt.Printf("      %s %s\n", probe, field)
					fmt.Printf("        %s %s\n", diffRed.Render("-"), diffDim.Render(truncate(c.OldValue, 60)))
					fmt.Printf("        %s %s\n", diffGreen.Render("+"), diffWhite.Render(truncate(c.NewValue, 60)))
				}
			}
			fmt.Println()
		}
	}

	_ = os.Stderr // keep import
}

func truncate(s string, max int) string {
	if len(s) > max {
		return s[:max-3] + "..."
	}
	return s
}
