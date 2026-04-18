package cmd

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	manCyan  = lipgloss.NewStyle().Foreground(lipgloss.Color("#00E5FF")).Bold(true)
	manTeal  = lipgloss.NewStyle().Foreground(lipgloss.Color("#00BFA5"))
	manGreen = lipgloss.NewStyle().Foreground(lipgloss.Color("#69F0AE"))
	manAmber = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFB74D"))
	manDim   = lipgloss.NewStyle().Foreground(lipgloss.Color("#616161"))
	manWhite = lipgloss.NewStyle().Foreground(lipgloss.Color("#E0E0E0"))
	manBold  = lipgloss.NewStyle().Foreground(lipgloss.Color("#E0E0E0")).Bold(true)
	manMag   = lipgloss.NewStyle().Foreground(lipgloss.Color("#E040FB")).Bold(true)
)

func section(title string) string {
	return "\n" + manCyan.Render(title) + "\n" + manDim.Render(strings.Repeat("─", 60))
}

func opt(flag, desc string) string {
	return fmt.Sprintf("  %s  %s", manGreen.Render(fmt.Sprintf("%-22s", flag)), manWhite.Render(desc))
}

func key(binding, desc string) string {
	return fmt.Sprintf("  %s  %s", manAmber.Render(fmt.Sprintf("%-18s", binding)), manWhite.Render(desc))
}

var manCmd = &cobra.Command{
	Use:   "man",
	Short: "Display the netmap manual",
	Long:  "Display a comprehensive manual for netmap in the terminal.",
	Run: func(cmd *cobra.Command, args []string) {
		printMan()
	},
}

func init() {
	rootCmd.AddCommand(manCmd)
}

func printMan() {
	logo := manCyan.Render(`
 ███╗   ██╗███████╗████████╗███╗   ███╗ █████╗ ██████╗
 ████╗  ██║██╔════╝╚══██╔══╝████╗ ████║██╔══██╗██╔══██╗
 ██╔██╗ ██║█████╗     ██║   ██╔████╔██║███████║██████╔╝
 ██║╚██╗██║██╔══╝     ██║   ██║╚██╔╝██║██╔══██║██╔═══╝
 ██║ ╚████║███████╗   ██║   ██║ ╚═╝ ██║██║  ██║██║
 ╚═╝  ╚═══╝╚══════╝   ╚═╝   ╚═╝     ╚═╝╚═╝  ╚═╝╚═╝`)

	subtitle := manTeal.Render("  Network topology mapper with a rich terminal interface")
	version := manDim.Render("  v1.0.0")

	lines := []string{
		logo,
		subtitle,
		version,

		section("NAME"),
		"  " + manBold.Render("netmap") + manWhite.Render(" — discover and visualize network topology"),

		section("SYNOPSIS"),
		"  " + manBold.Render("netmap") + manWhite.Render(" <target> [flags]"),
		"  " + manBold.Render("netmap man"),
		"  " + manBold.Render("netmap") + manWhite.Render(" <target> --headless [--output file] [--format yaml|json]"),

		section("DESCRIPTION"),
		"  " + manWhite.Render("netmap takes a hostname or IP address and maps the network"),
		"  " + manWhite.Render("topology around it using DNS, ping, traceroute, WHOIS, TLS,"),
		"  " + manWhite.Render("and HTTP inspection. Results are displayed in a rich interactive"),
		"  " + manWhite.Render("terminal UI with a navigable tree view, detail panel, and"),
		"  " + manWhite.Render("visual network path view."),

		section("FLAGS"),
		opt("-d, --depth <n>", "How many levels deep to crawl (default: 3)"),
		opt("-b, --breadth <n>", "Max child nodes per parent (default: 10)"),
		opt("-t, --timeout <dur>", "Scan timeout, e.g. 30s, 2m (default: 5m)"),
		opt("-o, --output <file>", "Write results to file when scan completes"),
		opt("-f, --format <fmt>", "Export format: yaml (default) or json"),
		opt("    --headless", "No TUI — output to stdout for piping"),
		opt("-h, --help", "Show help"),

		section("INTERACTIVE KEYBINDINGS"),
		"",
		manTeal.Render("  Navigation"),
		key("↑ / k", "Move cursor up"),
		key("↓ / j", "Move cursor down"),
		key("g", "Jump to top"),
		key("G", "Jump to bottom"),
		key("enter / space", "Expand/collapse tree node"),
		key("tab", "Scroll detail panel down"),
		key("shift+tab", "Scroll detail panel up"),
		"",
		manTeal.Render("  Views"),
		key("v", "Toggle visual network path view"),
		key("◄ / h", "Traverse path left (in path view)"),
		key("► / l", "Traverse path right (in path view)"),
		"",
		manTeal.Render("  Actions"),
		key("d", "Deep scan the selected node"),
		key("n", "Scan a new target"),
		key("esc", "Abort a running scan"),
		key("r", "Rescan the current target"),
		key("s", "Save results to file"),
		key("?", "Show/hide help overlay"),
		key("q / ctrl+c", "Quit"),

		section("SCAN PROBES"),
		"",
		manTeal.Render("  Standard (run on all discovered nodes)"),
		"  " + manWhite.Render("DNS        A, MX, NS, TXT, CNAME records"),
		"  " + manWhite.Render("Ping       ICMP echo with latency stats"),
		"  " + manWhite.Render("Traceroute Hop-by-hop route discovery"),
		"  " + manWhite.Render("TLS        Certificate, issuer, SANs, expiry"),
		"  " + manWhite.Render("HTTP       Status, headers, linked hosts"),
		"  " + manWhite.Render("WHOIS      Registrar, org, country, dates"),
		"",
		manMag.Render("  Deep Scan (triggered with 'd' key)"),
		"  " + manWhite.Render("Ports      Top 25 common ports via TCP connect"),
		"  " + manWhite.Render("Banners    Service identification on open ports"),
		"  " + manWhite.Render("TLS Deep   Protocol versions, cipher audit, cert chain"),
		"  " + manWhite.Render("DNS Deep   Reverse DNS, SPF, DMARC, DKIM, SRV records"),
		"  " + manWhite.Render("HTTP Deep  Security header audit, cookies, robots.txt"),

		section("NODE TYPES"),
		fmt.Sprintf("  %s  %s", manCyan.Render("◆"), manWhite.Render("Hostname — a domain name target")),
		fmt.Sprintf("  %s  %s", manGreen.Render("●"), manWhite.Render("IP Address — resolved from DNS")),
		fmt.Sprintf("  %s  %s", manAmber.Render("◇"), manWhite.Render("Router — traceroute hop")),
		fmt.Sprintf("  %s  %s", manMag.Render("⬢"), manWhite.Render("Deep Scanned — node has deep scan data")),

		section("EXAMPLES"),
		"",
		"  " + manDim.Render("# Interactive scan of a domain"),
		"  " + manBold.Render("netmap example.com"),
		"",
		"  " + manDim.Render("# Quick shallow scan with 30s timeout"),
		"  " + manBold.Render("netmap google.com -d 1 -b 5 -t 30s"),
		"",
		"  " + manDim.Render("# Save results to YAML file"),
		"  " + manBold.Render("netmap github.com -o scan.yaml"),
		"",
		"  " + manDim.Render("# Headless JSON output piped to jq"),
		"  " + manBold.Render("netmap cloudflare.com --headless -f json | jq '.nodes[].label'"),
		"",
		"  " + manDim.Render("# Headless scan in a cron job"),
		"  " + manBold.Render("netmap myserver.com --headless -t 2m -o /var/log/netmap/$(date +%F).yaml"),
		"",
		"  " + manDim.Render("# Scan an IP address"),
		"  " + manBold.Render("netmap 8.8.8.8"),

		section("VISUAL PATH VIEW"),
		"  " + manWhite.Render("Press 'v' on any node to see the network route from your"),
		"  " + manWhite.Render("machine to that node, displayed as connected device boxes."),
		"  " + manWhite.Render("Use arrow keys to traverse the path. Each device shows its"),
		"  " + manWhite.Render("type icon and latency. The selected device pulses cyan and"),
		"  " + manWhite.Render("shows its full details below. Press 'd' on any hop to deep"),
		"  " + manWhite.Render("scan it. Press 'v' or 'esc' to return to tree view."),
		"",
		"  " + manDim.Render("Example:"),
		"",
		"  " + manGreen.Render("╔══════╗") + manDim.Render("────▶") + manAmber.Render("╭────────╮") + manDim.Render("────▶") + manAmber.Render("╭────────╮") + manDim.Render("────▶") + manCyan.Render("╔══════════╗"),
		"  " + manGreen.Render("║  ⌂   ║") + "     " + manAmber.Render("│   ◇    │") + "     " + manAmber.Render("│   ◇    │") + "     " + manCyan.Render("║    ◆     ║"),
		"  " + manGreen.Render("║ YOU  ║") + "     " + manAmber.Render("│10.0.0.1│") + "     " + manAmber.Render("│72.14.2.│") + "     " + manCyan.Render("║google.com║"),
		"  " + manGreen.Render("╚══════╝") + "     " + manAmber.Render("╰────────╯") + "     " + manAmber.Render("╰────────╯") + "     " + manCyan.Render("╚══════════╝"),
		"  " + "              " + manDim.Render("2ms") + "            " + manDim.Render("15ms") + "            " + manDim.Render("45ms"),

		section("EXPORT FORMAT"),
		"  " + manWhite.Render("YAML (default) and JSON exports include all discovered nodes,"),
		"  " + manWhite.Render("edges, probe results, and timing data. The output is designed"),
		"  " + manWhite.Render("to be both human-readable and machine-parseable."),
		"",
		"  " + manWhite.Render("Export methods:"),
		"  " + manBold.Render("  --output/-o   ") + manWhite.Render("Write to file (TUI still runs)"),
		"  " + manBold.Render("  --headless    ") + manWhite.Render("Dump to stdout (no TUI)"),
		"  " + manBold.Render("  s key         ") + manWhite.Render("Save snapshot from within the TUI"),

		section("NOTES"),
		"  " + manWhite.Render("• Traceroute may require elevated privileges on some systems."),
		"  " + manWhite.Render("• Deep scan port checks use standard TCP connect — the same"),
		"  " + manWhite.Render("  mechanism your browser uses. No raw packets or SYN scans."),
		"  " + manWhite.Render("• Only scan targets you own or have permission to scan."),
		"  " + manWhite.Render("• The scan timeout applies to the overall scan duration,"),
		"  " + manWhite.Render("  not individual probes."),
		"",
		manDim.Render("  " + strings.Repeat("─", 60)),
		manDim.Render("  netmap — built with Go, bubbletea, and lipgloss"),
		manDim.Render("  Color scheme: Sonar (cyan/teal/green on dark)"),
		"",
	}

	fmt.Println(strings.Join(lines, "\n"))
}
