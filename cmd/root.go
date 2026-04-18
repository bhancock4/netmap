package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/bhancock4/netmap/internal/export"
	"github.com/bhancock4/netmap/internal/model"
	"github.com/bhancock4/netmap/internal/scanner"
	"github.com/bhancock4/netmap/internal/ui"
)

var (
	depth      int
	breadth    int
	timeout    string
	outputFile string
	format     string
	headless   bool
	saveName   string
	soundOn    bool
)

var rootCmd = &cobra.Command{
	Use:   "netmap <target>",
	Short: "Network topology mapper with a rich terminal UI",
	Long: `netmap discovers and visualizes network topology from a starting
target (hostname or IP address). It uses DNS, ping, traceroute,
WHOIS, TLS, and HTTP inspection to build an interactive map
of the network neighborhood around your target.

Press ? inside the TUI for a full keybinding reference.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Parse timeout
		dur, err := time.ParseDuration(timeout)
		if err != nil {
			return fmt.Errorf("invalid timeout %q: %w", timeout, err)
		}

		cfg := model.Config{
			Target:  args[0],
			Depth:   depth,
			Breadth: breadth,
			Timeout: dur,
		}

		if err := cfg.Validate(); err != nil {
			return err
		}

		// Determine export format
		var exportFormat export.Format
		switch format {
		case "json":
			exportFormat = export.FormatJSON
		default:
			exportFormat = export.FormatYAML
		}

		s := scanner.New(cfg)

		// Headless mode: no TUI, just scan and output
		if headless {
			return runHeadless(s, exportFormat)
		}

		// Start the scan in a background goroutine
		ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
		defer cancel()

		go s.Run(ctx)

		// Start the TUI
		m := ui.New(s)
		m.SetCancel(cancel)
		m.OutputFile = outputFile
		if saveName != "" {
			ext := "yaml"
			if exportFormat == export.FormatJSON {
				ext = "json"
			}
			m.OutputFile = filepath.Join(sessionsDir(), saveName+"."+ext)
		}
		m.Format = exportFormat
		m.SetSound(soundOn)
		p := tea.NewProgram(m, tea.WithAltScreen())

		if _, err := p.Run(); err != nil {
			return fmt.Errorf("TUI error: %w", err)
		}

		return nil
	},
}

func runHeadless(s *scanner.Scanner, format export.Format) error {
	start := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), s.Config.Timeout)
	defer cancel()

	// Drain events in background (non-blocking)
	go func() {
		for {
			select {
			case _, ok := <-s.Events:
				if !ok {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	s.Run(ctx)
	duration := time.Since(start)

	report := export.BuildReport(s.Graph, s.Config.Target, duration)

	// Write to file or stdout
	data, err := export.Marshal(report, format)
	if err != nil {
		return err
	}

	// Save session if requested
	if saveName != "" {
		ext := "yaml"
		if format == export.FormatJSON {
			ext = "json"
		}
		sessPath := filepath.Join(sessionsDir(), saveName+"."+ext)
		if err := os.WriteFile(sessPath, data, 0644); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Session saved to %s\n", sessPath)
	}

	if outputFile != "" {
		if err := os.WriteFile(outputFile, data, 0644); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Results written to %s\n", outputFile)
	} else {
		fmt.Println(string(data))
	}

	return nil
}

func init() {
	rootCmd.Flags().IntVarP(&depth, "depth", "d", 3, "How many levels deep to crawl (1-5)")
	rootCmd.Flags().IntVarP(&breadth, "breadth", "b", 10, "Max child nodes to explore per node")
	rootCmd.Flags().StringVarP(&timeout, "timeout", "t", "5m", "Scan timeout (e.g. 30s, 2m, 5m)")
	rootCmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write results to file")
	rootCmd.Flags().StringVarP(&format, "format", "f", "yaml", "Export format: yaml or json")
	rootCmd.Flags().BoolVar(&headless, "headless", false, "Run without TUI, output to stdout (for piping/scripts)")
	rootCmd.Flags().StringVar(&saveName, "save", "", "Save session to ~/.netmap/sessions/<name>")
	rootCmd.Flags().BoolVar(&soundOn, "sound", false, "Enable sound effects (toggle with 'm' in TUI)")
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
