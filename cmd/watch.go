package cmd

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/bhancock4/netmap/internal/model"
	"github.com/bhancock4/netmap/internal/scanner"
	"github.com/bhancock4/netmap/internal/ui"
)

var watchInterval string

var watchCmd = &cobra.Command{
	Use:   "watch <target>",
	Short: "Continuously monitor a target with periodic rescans",
	Long: `Run netmap in watch mode — performs an initial scan, then
rescans at the specified interval. Changed nodes flash in the UI.
Press 'q' to stop watching.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		interval, err := time.ParseDuration(watchInterval)
		if err != nil {
			return fmt.Errorf("invalid interval %q: %w", watchInterval, err)
		}
		if interval < 10*time.Second {
			interval = 10 * time.Second
		}

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

		s := scanner.New(cfg)

		ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
		defer cancel()

		go s.Run(ctx)

		m := ui.New(s)
		m.SetCancel(cancel)
		m.WatchMode = true
		m.WatchInterval = interval
		p := tea.NewProgram(m, tea.WithAltScreen())

		if _, err := p.Run(); err != nil {
			return fmt.Errorf("TUI error: %w", err)
		}
		return nil
	},
}

func init() {
	watchCmd.Flags().StringVarP(&watchInterval, "interval", "i", "60s", "Rescan interval (e.g. 30s, 2m, 5m)")
	watchCmd.Flags().IntVarP(&depth, "depth", "d", 3, "How many levels deep to crawl (1-5)")
	watchCmd.Flags().IntVarP(&breadth, "breadth", "b", 10, "Max child nodes to explore per node")
	watchCmd.Flags().StringVarP(&timeout, "timeout", "t", "5m", "Scan timeout (e.g. 30s, 2m, 5m)")
	rootCmd.AddCommand(watchCmd)
}
