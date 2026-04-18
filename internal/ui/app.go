package ui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/bhancock4/netmap/internal/export"
	"github.com/bhancock4/netmap/internal/model"
	"github.com/bhancock4/netmap/internal/scanner"
)

// View mode
type viewMode int

const (
	viewTree viewMode = iota
	viewPath
	viewGraph
)

// scanEventMsg wraps a scanner event for the bubbletea event loop.
type scanEventMsg scanner.Event

// tickMsg drives the spinner animation.
type tickMsg time.Time

// savedMsg is sent after a save completes.
type savedMsg struct {
	path string
	err  error
}

// deepScanDoneMsg is sent when a deep scan finishes.
type deepScanDoneMsg struct {
	nodeID string
}

// Model is the main bubbletea model.
type Model struct {
	graph        *model.Graph
	scanner      *scanner.Scanner
	config       model.Config
	events       []scanner.Event
	width        int
	height       int
	cursor       int
	selectedID   string
	flatTree     []string
	collapsed    map[string]bool
	scroll       int
	detScroll    int
	scanning     bool
	deepScanning bool
	deepNodeID   string
	spinFrame    int
	statusMsg    string
	startTime    time.Time
	showHelp     bool
	mode         viewMode
	pathCursor   int // cursor position in path view
	inputMode    bool   // true when typing a new target
	inputBuffer  string // the text being typed
	OutputFile    string
	Format        export.Format
	WatchMode     bool
	WatchInterval time.Duration
	watchNext     time.Time     // when the next rescan fires
	sound         *Sound
	cancelScan    context.CancelFunc
}

// New creates a new UI model.
func New(s *scanner.Scanner) Model {
	return Model{
		graph:     s.Graph,
		scanner:   s,
		config:    s.Config,
		scanning:  true,
		collapsed: make(map[string]bool),
		startTime: time.Now(),
		statusMsg: "Starting scan...",
		Format:    export.FormatYAML,
		mode:      viewTree,
		sound:     NewSound(),
	}
}

// SetCancel sets the cancel function for the current scan context.
func (m *Model) SetCancel(cancel context.CancelFunc) {
	m.cancelScan = cancel
}

// SetSound enables sound from the start (--sound flag).
func (m *Model) SetSound(on bool) {
	if on {
		m.sound.enabled = true
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.listenForEvents(),
		m.tickCmd(),
	)
}

func (m Model) listenForEvents() tea.Cmd {
	return func() tea.Msg {
		event, ok := <-m.scanner.Events
		if !ok {
			return scanEventMsg{Type: model.EventScanDone}
		}
		return scanEventMsg(event)
	}
}

func (m Model) tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Input mode — typing a new target
		if m.inputMode {
			return m.updateInput(msg)
		}

		// Help overlay
		if m.showHelp {
			switch msg.String() {
			case "?", "escape", "enter":
				m.showHelp = false
			case "q", "ctrl+c":
				return m, tea.Quit
			}
			return m, nil
		}

		// Path view keys
		if m.mode == viewPath {
			return m.updatePathView(msg)
		}

		// Graph view keys
		if m.mode == viewGraph {
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			case "t", "escape", "v":
				m.mode = viewTree
			case "?":
				m.showHelp = true
			}
			return m, nil
		}

		// Tree view keys
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "escape":
			if m.scanning && m.cancelScan != nil {
				m.cancelScan()
				m.scanning = false
				m.statusMsg = fmt.Sprintf("Scan aborted — %d nodes discovered in %s",
					m.graph.NodeCount(), time.Since(m.startTime).Round(time.Millisecond))
			}
		case "?":
			m.showHelp = true
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				m.syncSelectedID()
				m.detScroll = 0
				m.adjustScroll()
			}
		case "down", "j":
			if m.cursor < len(m.flatTree)-1 {
				m.cursor++
				m.syncSelectedID()
				m.detScroll = 0
				m.adjustScroll()
			}
		case "home", "g":
			m.cursor = 0
			m.syncSelectedID()
			m.scroll = 0
			m.detScroll = 0
		case "end", "G":
			if len(m.flatTree) > 0 {
				m.cursor = len(m.flatTree) - 1
				m.syncSelectedID()
				m.adjustScroll()
				m.detScroll = 0
			}
		case "enter", " ":
			if m.cursor < len(m.flatTree) {
				nodeID := m.flatTree[m.cursor]
				node, ok := m.graph.GetNode(nodeID)
				if ok && len(node.Children) > 0 {
					m.collapsed[nodeID] = !m.collapsed[nodeID]
					m.rebuildTree()
				}
			}
		case "tab":
			m.detScroll++
		case "shift+tab":
			if m.detScroll > 0 {
				m.detScroll--
			}
		case "d":
			// Deep scan selected node
			if m.cursor < len(m.flatTree) && !m.deepScanning {
				nodeID := m.flatTree[m.cursor]
				node, ok := m.graph.GetNode(nodeID)
				if ok && !node.DeepScanned {
					return m, m.startDeepScan(nodeID)
				}
			}
		case "t":
			// Switch to topology graph view
			m.mode = viewGraph
		case "v":
			// Switch to path view
			if m.cursor < len(m.flatTree) {
				m.mode = viewPath
				m.pathCursor = 0
				path := m.buildPath()
				if len(path) > 0 {
					m.pathCursor = len(path) - 1 // start at the target
				}
			}
		case "m":
			// Toggle sound
			on := m.sound.Toggle()
			if on {
				m.statusMsg = StyleSuccess.Render("♪") + " Sound on"
				m.sound.Play(SoundNodeDiscovered)
			} else {
				m.statusMsg = StyleDim.Render("♪") + " Sound off"
			}
		case "n":
			// New target
			if !m.scanning && !m.deepScanning {
				m.inputMode = true
				m.inputBuffer = ""
				m.statusMsg = ""
			}
		case "r":
			if !m.scanning && !m.deepScanning {
				return m, m.startRescan()
			}
		case "s":
			if !m.scanning {
				return m, m.saveSnapshot()
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tickMsg:
		m.spinFrame = (m.spinFrame + 1) % 10000
		m.rebuildTree()

		// Watch mode: auto-rescan when interval elapses
		if m.WatchMode && !m.scanning && !m.deepScanning && !m.watchNext.IsZero() && time.Now().After(m.watchNext) {
			m.watchNext = time.Time{} // clear to prevent double-trigger
			return m, m.startRescan()
		}

		return m, m.tickCmd()

	case scanEventMsg:
		event := scanner.Event(msg)
		m.events = append(m.events, event)
		m.statusMsg = event.Message

		if event.Type == model.EventNodeAdded {
			m.sound.Play(SoundNodeDiscovered)
		}

		if event.Type == model.EventScanDone {
			m.sound.Play(SoundScanComplete)
			m.scanning = false
			elapsed := time.Since(m.startTime).Round(time.Millisecond)
			m.statusMsg = fmt.Sprintf("Scan complete — %d nodes discovered in %s",
				m.graph.NodeCount(), elapsed)
			if m.WatchMode {
				m.watchNext = time.Now().Add(m.WatchInterval)
				m.statusMsg += fmt.Sprintf(" │ next scan in %s", m.WatchInterval)
			}
			if m.OutputFile != "" {
				return m, m.saveToFile(m.OutputFile)
			}
		}

		if event.Type == model.EventDeepDone {
			m.sound.Play(SoundDeepScanComplete)
			m.deepScanning = false
			m.deepNodeID = ""
			m.statusMsg = StyleDeepScan.Render("⬢") + " " + event.Message
		}

		m.rebuildTree()

		if m.scanning || m.deepScanning {
			return m, m.listenForEvents()
		}
		return m, nil

	case deepScanDoneMsg:
		m.deepScanning = false
		m.deepNodeID = ""

	case savedMsg:
		if msg.err != nil {
			m.statusMsg = StyleError.Render("Save failed: " + msg.err.Error())
		} else {
			m.statusMsg = StyleSuccess.Render("Saved to " + msg.path)
		}
	}

	return m, nil
}

func (m *Model) updateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape:
		m.inputMode = false
		m.inputBuffer = ""
		m.statusMsg = "Cancelled"
	case tea.KeyEnter:
		target := strings.TrimSpace(m.inputBuffer)
		m.inputMode = false
		m.inputBuffer = ""
		if target == "" {
			m.statusMsg = "No target entered"
			return m, nil
		}
		return m, m.startNewScan(target)
	case tea.KeyBackspace:
		if len(m.inputBuffer) > 0 {
			m.inputBuffer = m.inputBuffer[:len(m.inputBuffer)-1]
		}
	case tea.KeyRunes:
		m.inputBuffer += msg.String()
	case tea.KeySpace:
		// ignore spaces in target
	}
	return m, nil
}

func (m *Model) startNewScan(target string) tea.Cmd {
	if m.cancelScan != nil {
		m.cancelScan()
	}

	m.config.Target = target
	s := scanner.New(m.config)
	m.scanner = s
	m.graph = s.Graph
	m.events = nil
	m.flatTree = nil
	m.collapsed = make(map[string]bool)
	m.cursor = 0
	m.selectedID = ""
	m.scroll = 0
	m.detScroll = 0
	m.scanning = true
	m.deepScanning = false
	m.deepNodeID = ""
	m.startTime = time.Now()
	m.statusMsg = "Scanning " + target + "..."
	m.mode = viewTree

	ctx, cancel := context.WithCancel(context.Background())
	m.cancelScan = cancel

	return func() tea.Msg {
		go s.Run(ctx)
		event, ok := <-s.Events
		if !ok {
			return scanEventMsg{Type: model.EventScanDone}
		}
		return scanEventMsg(event)
	}
}

func (m *Model) updatePathView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "?":
		m.showHelp = true
	case "v", "escape":
		m.mode = viewTree
	case "left", "h":
		if m.pathCursor > 0 {
			m.pathCursor--
			m.detScroll = 0
		}
	case "right", "l":
		path := m.buildPath()
		if m.pathCursor < len(path)-1 {
			m.pathCursor++
			m.detScroll = 0
		}
	case "up", "k":
		if m.detScroll > 0 {
			m.detScroll--
		}
	case "down", "j":
		m.detScroll++
	case "d":
		// Deep scan from path view too
		path := m.buildPath()
		if m.pathCursor < len(path) && !m.deepScanning {
			pn := path[m.pathCursor]
			if pn.id != "__you__" {
				node, ok := m.graph.GetNode(pn.id)
				if ok && !node.DeepScanned {
					return m, m.startDeepScan(pn.id)
				}
			}
		}
	}
	return m, nil
}

func (m *Model) startDeepScan(nodeID string) tea.Cmd {
	m.deepScanning = true
	m.deepNodeID = nodeID
	m.statusMsg = StyleDeepScan.Render("◉") + " Deep scanning..."

	s := m.scanner
	timeout := m.config.Timeout
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		s.DeepScan(ctx, nodeID)
		return scanEventMsg{Type: model.EventDeepDone, NodeID: nodeID, Message: "Deep scan complete"}
	}
}

func (m *Model) syncSelectedID() {
	if m.cursor >= 0 && m.cursor < len(m.flatTree) {
		m.selectedID = m.flatTree[m.cursor]
	}
}

func (m *Model) restoreCursor() {
	if m.selectedID == "" {
		return
	}
	for i, id := range m.flatTree {
		if id == m.selectedID {
			m.cursor = i
			m.adjustScroll()
			return
		}
	}
	if m.cursor >= len(m.flatTree) && len(m.flatTree) > 0 {
		m.cursor = len(m.flatTree) - 1
		m.syncSelectedID()
	}
}

func (m *Model) startRescan() tea.Cmd {
	if m.cancelScan != nil {
		m.cancelScan()
	}

	s := scanner.New(m.config)
	m.scanner = s
	m.graph = s.Graph
	m.events = nil
	m.flatTree = nil
	m.collapsed = make(map[string]bool)
	m.cursor = 0
	m.selectedID = ""
	m.scroll = 0
	m.detScroll = 0
	m.scanning = true
	m.startTime = time.Now()
	m.statusMsg = "Rescanning..."
	m.mode = viewTree

	ctx, cancel := context.WithCancel(context.Background())
	m.cancelScan = cancel

	return func() tea.Msg {
		go s.Run(ctx)
		event, ok := <-s.Events
		if !ok {
			return scanEventMsg{Type: model.EventScanDone}
		}
		return scanEventMsg(event)
	}
}

func (m Model) saveSnapshot() tea.Cmd {
	target := m.scanner.Config.Target
	ext := "yaml"
	if m.Format == export.FormatJSON {
		ext = "json"
	}
	filename := fmt.Sprintf("netmap_%s_%s.%s",
		strings.ReplaceAll(target, ".", "_"),
		time.Now().Format("20060102_150405"),
		ext,
	)
	return m.saveToFile(filename)
}

func (m Model) saveToFile(path string) tea.Cmd {
	return func() tea.Msg {
		report := export.BuildReport(m.graph, m.scanner.Config.Target, time.Since(m.startTime))
		err := export.WriteFile(report, path, m.Format)
		return savedMsg{path: path, err: err}
	}
}

func (m *Model) adjustScroll() {
	treeHeight := m.treeViewHeight()
	if m.cursor < m.scroll {
		m.scroll = m.cursor
	}
	if m.cursor >= m.scroll+treeHeight {
		m.scroll = m.cursor - treeHeight + 1
	}
}

func (m *Model) treeViewHeight() int {
	return max(m.height-12, 5)
}

func (m *Model) rebuildTree() {
	m.flatTree = nil
	if m.graph.Root == "" {
		return
	}
	m.walkTree(m.graph.Root, 0)
	m.restoreCursor()
}

func (m *Model) walkTree(nodeID string, depth int) {
	m.flatTree = append(m.flatTree, nodeID)
	if m.collapsed[nodeID] {
		return
	}
	node, ok := m.graph.GetNode(nodeID)
	if !ok {
		return
	}
	for _, childID := range node.Children {
		m.walkTree(childID, depth+1)
	}
}

// ─────────────────────── VIEW ───────────────────────

func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	if m.showHelp {
		return m.renderHelp()
	}

	var sections []string

	// Header
	sections = append(sections, m.renderHeader())

	// Fixed height for the main content area — prevents bouncing
	headerHeight := 9 // logo + scan info + blank line
	statusHeight := 1
	mainHeight := max(m.height-headerHeight-statusHeight, 10)

	if m.mode == viewGraph {
		// Topology graph view: full width
		graphView := m.renderGraphView(m.width-2, mainHeight)
		graphView = lipgloss.NewStyle().Height(mainHeight).MaxHeight(mainHeight).Render(graphView)
		sections = append(sections, graphView)
	} else if m.mode == viewPath {
		// Path view: size path to content, give the rest to detail
		path := m.buildPath()
		nodesPerRow := max((m.width-4-4)/(16+8+2), 2)
		pathRows := (len(path) + nodesPerRow - 1) / nodesPerRow
		pathContentHeight := pathRows*6 + max(pathRows-1, 0)*3 + 7
		pathHeight := min(pathContentHeight, mainHeight/2)
		detailHeight := mainHeight - pathHeight

		pathView := m.renderPathView(m.width-4, pathHeight)
		pathView = lipgloss.NewStyle().Height(pathHeight).MaxHeight(pathHeight).Render(pathView)

		detail := m.renderPathDetail(m.width - 4)
		detail = lipgloss.NewStyle().Height(detailHeight).MaxHeight(detailHeight).Render(detail)

		sections = append(sections, pathView)
		sections = append(sections, detail)
	} else {
		// Tree view: side by side, fixed height
		treeWidth := min(m.width*2/5, 60)
		detailWidth := m.width - treeWidth - 4

		tree := m.renderTree(treeWidth)
		detail := m.renderDetail(detailWidth)

		tree = lipgloss.NewStyle().Height(mainHeight).Render(tree)
		detail = lipgloss.NewStyle().Height(mainHeight).MaxHeight(mainHeight).Render(detail)

		mainContent := lipgloss.JoinHorizontal(lipgloss.Top, tree, "  ", detail)
		sections = append(sections, mainContent)
	}

	// Status bar
	sections = append(sections, m.renderStatusBar())

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m Model) renderHelp() string {
	title := StyleLogo.Render(" NETMAP HELP ")
	divider := StyleDim.Render(strings.Repeat("─", 50))

	sections := []string{
		title,
		"",
		StyleSubtitle.Render("  Navigation"),
		divider,
		helpLine("↑/k", "Move up"),
		helpLine("↓/j", "Move down"),
		helpLine("g", "Jump to top"),
		helpLine("G", "Jump to bottom"),
		helpLine("enter/space", "Expand/collapse node"),
		helpLine("tab", "Scroll detail panel down"),
		helpLine("shift+tab", "Scroll detail panel up"),
		"",
		StyleSubtitle.Render("  Views"),
		divider,
		helpLine("v", "Toggle visual path view"),
		helpLine("◄/►", "Traverse path (in path view)"),
		"",
		StyleSubtitle.Render("  Actions"),
		divider,
		helpLine("d", "Deep scan selected node"),
		helpLine("m", "Toggle sound effects"),
		helpLine("n", "New target (enter address)"),
		helpLine("esc", "Abort running scan"),
		helpLine("r", "Rescan current target"),
		helpLine("s", "Save results to file"),
		helpLine("?", "Toggle this help"),
		helpLine("q/ctrl+c", "Quit"),
		"",
		StyleSubtitle.Render("  Node Icons"),
		divider,
		helpLine(StyleNodeHost.Render("◆"), "Hostname"),
		helpLine(StyleNodeIP.Render("●"), "IP Address"),
		helpLine(StyleNodeRouter.Render("◇"), "Router (traceroute hop)"),
		helpLine(StyleDeepScan.Render("⬢"), "Deep scanned"),
		"",
		StyleSubtitle.Render("  Status Icons"),
		divider,
		helpLine(StyleSuccess.Render("✓"), "All probes succeeded"),
		helpLine(StyleError.Render("✗"), "One or more probes failed"),
		helpLine(StyleWarning.Render("⏱"), "Timeout"),
		helpLine(StyleDim.Render("…"), "Pending"),
		"",
		StyleDim.Render("  Press ? or escape to close"),
	}

	content := strings.Join(sections, "\n")

	overlay := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(ColorCyan).
		Padding(1, 2).
		Width(56).
		Render(content)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, overlay)
}

func helpLine(key, desc string) string {
	k := lipgloss.NewStyle().
		Foreground(ColorCyan).
		Width(18).
		Align(lipgloss.Right).
		PaddingRight(2).
		Render(key)
	d := StyleValue.Render(desc)
	return k + d
}

func (m Model) renderHeader() string {
	logo := StyleLogo.Render(Logo)

	var scanInfo string
	if m.deepScanning {
		frame := DeepScanFrames[m.spinFrame%len(DeepScanFrames)]
		jelly := m.renderMiniJellyfish()
		scanInfo = StyleDeepScan.Render(frame) + " " +
			StyleDeepScan.Render("Deep scanning ") +
			StyleNodeHost.Render(m.deepNodeID) +
			"  " + jelly
	} else if m.scanning {
		frame := SonarFrames[m.spinFrame%len(SonarFrames)]
		scanInfo = StyleSpinner.Render(frame) + " " +
			StyleSubtitle.Render("Scanning ") +
			StyleNodeHost.Render(m.scanner.Config.Target)
	} else {
		scanInfo = StyleSuccess.Render("●") + " " +
			StyleSubtitle.Render("Mapped ") +
			StyleNodeHost.Render(m.scanner.Config.Target)
	}

	return logo + "\n" + scanInfo + "\n"
}

// renderMiniJellyfish renders a single-line animated jellyfish.
func (m Model) renderMiniJellyfish() string {
	frames := []string{
		StyleDeepScan.Render("~(°_°)~"),
		StyleDeepScan.Render("~(°_°)~~"),
		StyleDeepScan.Render("~~(°_°)~"),
		StyleDeepScan.Render("~(°_°)~"),
		StyleDeepScan.Render("~~(°_°)~~"),
		StyleDeepScan.Render("~(°_°)~~~"),
	}
	return frames[m.spinFrame%len(frames)]
}

func (m Model) renderTree(width int) string {
	if len(m.flatTree) == 0 {
		return StylePanel.Width(width).Render("Discovering nodes...")
	}

	treeHeight := m.treeViewHeight()
	var lines []string

	end := min(m.scroll+treeHeight, len(m.flatTree))
	for i := m.scroll; i < end; i++ {
		nodeID := m.flatTree[i]
		node, ok := m.graph.GetNode(nodeID)
		if !ok {
			continue
		}

		prefix := m.treePrefix(node.Depth)
		icon := m.nodeIcon(node)
		label := node.Label

		// Deep scan badge
		deepBadge := ""
		if node.DeepScanned {
			deepBadge = StyleDeepScan.Render(" ⬢")
		} else if m.deepScanning && nodeID == m.deepNodeID {
			frame := DeepScanFrames[m.spinFrame%len(DeepScanFrames)]
			deepBadge = StyleDeepScan.Render(" " + frame)
		}

		// Collapse indicator
		collapseIndicator := ""
		if len(node.Children) > 0 {
			if m.collapsed[nodeID] {
				collapseIndicator = StyleDim.Render(fmt.Sprintf(" [+%d]", m.countDescendants(nodeID)))
			} else {
				collapseIndicator = StyleDim.Render(" [-]")
			}
		}

		status := m.probeStatusIcon(node)
		line := prefix + icon + " " + label + deepBadge + collapseIndicator + " " + status

		if i == m.cursor {
			line = StyleSelected.Width(width - 2).Render(line)
		} else {
			if lipgloss.Width(line) > width-2 {
				runes := []rune(line)
				if len(runes) > width-5 {
					line = string(runes[:width-5]) + "..."
				}
			}
		}

		lines = append(lines, line)
	}

	title := StyleTitle.Render("  Network Map")
	if m.scroll > 0 {
		title += StyleDim.Render(" ▲")
	}
	if end < len(m.flatTree) {
		lines = append(lines, StyleDim.Render("  ▼ more..."))
	}

	content := title + "\n" + strings.Join(lines, "\n")
	return StylePanel.Width(width).Render(content)
}

func (m Model) countDescendants(nodeID string) int {
	node, ok := m.graph.GetNode(nodeID)
	if !ok {
		return 0
	}
	count := len(node.Children)
	for _, childID := range node.Children {
		count += m.countDescendants(childID)
	}
	return count
}

func (m Model) treePrefix(depth int) string {
	if depth == 0 {
		return ""
	}
	prefix := strings.Repeat("  ", depth-1)
	return StyleTreeBranch.Render(prefix + "├─ ")
}

func (m Model) nodeIcon(node *model.Node) string {
	if node.DeepScanned {
		switch node.Type {
		case model.NodeTypeHost:
			return StyleDeepScan.Render("◆")
		case model.NodeTypeIP:
			return StyleDeepScan.Render("●")
		case model.NodeTypeRouter:
			return StyleDeepScan.Render("◇")
		default:
			return StyleDeepScan.Render("○")
		}
	}
	switch node.Type {
	case model.NodeTypeHost:
		return StyleNodeHost.Render("◆")
	case model.NodeTypeIP:
		return StyleNodeIP.Render("●")
	case model.NodeTypeRouter:
		return StyleNodeRouter.Render("◇")
	default:
		return "○"
	}
}

func (m Model) probeStatusIcon(node *model.Node) string {
	switch node.StatusSummary() {
	case model.ProbeStatusRunning:
		frame := PulseFrames[m.spinFrame%len(PulseFrames)]
		return StyleSpinner.Render(frame)
	case model.ProbeStatusSuccess:
		return StyleSuccess.Render("✓")
	case model.ProbeStatusFailed:
		return StyleError.Render("✗")
	case model.ProbeStatusTimeout:
		return StyleWarning.Render("⏱")
	default:
		return StyleDim.Render("…")
	}
}

func (m Model) renderDetail(width int) string {
	if len(m.flatTree) == 0 || m.cursor >= len(m.flatTree) {
		return StylePanel.Width(width).Render(StyleDim.Render("Select a node to view details"))
	}

	nodeID := m.flatTree[m.cursor]
	return m.renderNodeDetail(nodeID, width)
}

func (m Model) renderPathDetail(width int) string {
	path := m.buildPath()
	if m.pathCursor >= len(path) || len(path) == 0 {
		return ""
	}
	pn := path[m.pathCursor]
	if pn.id == "__you__" {
		return StylePanel.Width(width).Render(
			StyleDeviceYou.Render("⌂ YOU") + "\n" + StyleDim.Render("  Local machine"))
	}
	return m.renderNodeDetail(pn.id, width)
}

func (m Model) renderNodeDetail(nodeID string, width int) string {
	node, ok := m.graph.GetNode(nodeID)
	if !ok {
		return ""
	}

	var lines []string

	// Node header
	header := m.nodeIcon(node) + " " + StyleTitle.Render(node.Label)
	if node.DeepScanned {
		header += " " + StyleDeepScan.Render("DEEP SCANNED")
	}
	lines = append(lines, header)
	lines = append(lines, StyleDim.Render(fmt.Sprintf("  Type: %s  Depth: %d", node.Type, node.Depth)))
	lines = append(lines, "")

	// Group probes: standard first, then deep
	standardProbes := []model.ProbeResult{}
	deepProbes := []model.ProbeResult{}
	for _, probe := range node.Probes {
		switch probe.Type {
		case "ports", "banners", "tls_deep", "dns_deep", "http_deep":
			deepProbes = append(deepProbes, probe)
		default:
			standardProbes = append(standardProbes, probe)
		}
	}

	// Standard probes
	for _, probe := range standardProbes {
		lines = append(lines, m.renderProbeDetail(probe, width)...)
	}

	// Deep probes with section header
	if len(deepProbes) > 0 {
		lines = append(lines, StyleDeepScan.Render("── Deep Scan Results ──"))
		lines = append(lines, "")
		for _, probe := range deepProbes {
			lines = append(lines, m.renderProbeDetail(probe, width)...)
		}
	}

	if len(node.Probes) == 0 {
		if m.scanning || m.deepScanning {
			frame := SonarFrames[m.spinFrame%len(SonarFrames)]
			lines = append(lines, StyleSpinner.Render(frame)+" "+StyleDim.Render("Waiting for probes..."))
		} else {
			lines = append(lines, StyleDim.Render("No probe data"))
		}
	}

	// Edges
	edges := m.edgesFor(nodeID)
	if len(edges) > 0 {
		lines = append(lines, StyleSubtitle.Render("Connections"))
		for _, e := range edges {
			dir := "→"
			other := e.To
			if e.To == nodeID {
				dir = "←"
				other = e.From
			}
			lines = append(lines, fmt.Sprintf("  %s %s %s %s",
				StyleDim.Render(dir),
				StyleDim.Render(e.Type.String()),
				StyleValue.Render(other),
				StyleDim.Render(e.Label),
			))
		}
	}

	// Apply detail scroll
	detailHeight := m.treeViewHeight()
	if m.detScroll > 0 && m.detScroll < len(lines) {
		lines = lines[m.detScroll:]
	}
	if len(lines) > detailHeight {
		lines = lines[:detailHeight]
		lines = append(lines, StyleDim.Render("  ▼ tab to scroll"))
	}

	content := strings.Join(lines, "\n")

	title := StyleTitle.Render("  Node Detail")
	scrollHint := ""
	if m.detScroll > 0 {
		scrollHint = StyleDim.Render(" ▲")
	}
	return StylePanel.Width(width).Render(title + scrollHint + "\n" + content)
}

func (m Model) renderProbeDetail(probe model.ProbeResult, width int) []string {
	var lines []string

	probeHeader := m.probeIcon(probe.Status) + " " +
		StyleSubtitle.Render(strings.ToUpper(probe.Type))
	if probe.Latency > 0 {
		probeHeader += StyleDim.Render(fmt.Sprintf(" (%s)", probe.Latency.Round(time.Millisecond)))
	}
	lines = append(lines, probeHeader)

	if probe.Error != "" {
		lines = append(lines, "  "+StyleError.Render(probe.Error))
	}

	keys := make([]string, 0, len(probe.Data))
	for k := range probe.Data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		v := probe.Data[k]
		maxValWidth := width - 26
		if maxValWidth > 0 && len(v) > maxValWidth {
			v = v[:maxValWidth-3] + "..."
		}
		line := StyleLabel.Render(k) + StyleValue.Render(v)
		lines = append(lines, line)
	}
	lines = append(lines, "")
	return lines
}

func (m Model) probeIcon(status model.ProbeStatus) string {
	switch status {
	case model.ProbeStatusRunning:
		frame := SpinnerFrames[m.spinFrame%len(SpinnerFrames)]
		return StyleSpinner.Render(frame)
	case model.ProbeStatusSuccess:
		return StyleSuccess.Render("●")
	case model.ProbeStatusFailed:
		return StyleError.Render("●")
	case model.ProbeStatusTimeout:
		return StyleWarning.Render("●")
	default:
		return StyleDim.Render("○")
	}
}

func (m Model) edgesFor(nodeID string) []model.Edge {
	var result []model.Edge
	for _, e := range m.graph.Edges {
		if e.From == nodeID || e.To == nodeID {
			result = append(result, e)
		}
	}
	return result
}

func (m Model) renderStatusBar() string {
	// Input mode: show the target prompt
	if m.inputMode {
		prompt := StyleCyan.Render(" Target: ")
		cursor := StyleCyan.Render("█")
		input := StyleValue.Render(m.inputBuffer) + cursor
		hint := StyleDim.Render("  (enter to scan, esc to cancel)")
		return prompt + input + hint
	}

	left := StyleStatusBar.Render(m.statusMsg)

	nodeCount := fmt.Sprintf("%d nodes", m.graph.NodeCount())
	elapsed := time.Since(m.startTime).Round(time.Second).String()

	// Watch mode countdown
	watchInfo := ""
	if m.WatchMode && !m.watchNext.IsZero() && !m.scanning {
		remaining := time.Until(m.watchNext).Round(time.Second)
		if remaining > 0 {
			watchInfo = fmt.Sprintf("│ rescan in %s ", remaining)
		}
	}

	var hints string
	if m.mode == viewGraph {
		hints = "t return to tree │ ? help │ q quit"
	} else if m.mode == viewPath {
		hints = "◄► traverse │ ↑↓ scroll detail │ d deep │ v tree view │ ? help │ q quit"
	} else if m.scanning {
		hints = "↑↓ navigate │ esc abort │ ? help │ q quit"
	} else if m.deepScanning {
		hints = "↑↓ navigate │ v path │ ? help │ q quit"
	} else if m.WatchMode {
		hints = "↑↓ nav │ enter fold │ d deep │ v path │ s save │ ? help │ q quit"
	} else {
		hints = "↑↓ nav │ enter fold │ d deep │ v path │ t topo │ n new │ r rescan │ s save │ ? help │ q quit"
	}

	_ = watchInfo

	right := StyleDim.Render(fmt.Sprintf("%s │ %s %s│ %s", nodeCount, elapsed, watchInfo, hints))

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}

	return left + strings.Repeat(" ", gap) + right
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
