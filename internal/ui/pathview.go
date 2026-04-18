package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/bhancock4/netmap/internal/model"
)

// pathNode holds display info for a node in the visual path.
type pathNode struct {
	id      string
	label   string
	ntype   model.NodeType
	latency string
}

// buildPath builds the traceroute path from root to the selected node.
func (m Model) buildPath() []pathNode {
	if len(m.flatTree) == 0 || m.cursor >= len(m.flatTree) {
		return nil
	}

	targetID := m.flatTree[m.cursor]

	// Walk up from target to root via parent links
	var chain []pathNode
	current := targetID
	for current != "" {
		node, ok := m.graph.GetNode(current)
		if !ok {
			break
		}

		pn := pathNode{
			id:    current,
			label: node.Label,
			ntype: node.Type,
		}

		// Find latency from ping or traceroute data
		for _, probe := range node.Probes {
			if probe.Type == "ping" && probe.Data["avg_ms"] != "" {
				pn.latency = probe.Data["avg_ms"] + "ms"
				break
			}
		}

		chain = append([]pathNode{pn}, chain...) // prepend
		current = node.Parent
	}

	// Add "YOU" at the start
	if len(chain) > 0 {
		you := pathNode{
			id:    "__you__",
			label: "YOU",
			ntype: -1, // special
		}
		chain = append([]pathNode{you}, chain...)
	}

	return chain
}

// renderPathView renders the visual network path with device icons.
func (m Model) renderPathView(width, height int) string {
	path := m.buildPath()
	if len(path) == 0 {
		return StylePanel.Width(width).Render(
			StyleDim.Render("  Select a node and press v to view its network path"))
	}

	// Clamp path cursor
	pathCursor := m.pathCursor
	if pathCursor >= len(path) {
		pathCursor = len(path) - 1
	}

	boxWidth := 16
	connWidth := 8
	nodesPerRow := (width - 4) / (boxWidth + connWidth + 2)
	if nodesPerRow < 2 {
		nodesPerRow = 2
	}

	var rows []string

	for i := 0; i < len(path); i += nodesPerRow {
		end := i + nodesPerRow
		if end > len(path) {
			end = len(path)
		}
		chunk := path[i:end]

		var boxes []string
		for j, pn := range chunk {
			globalIdx := i + j
			selected := globalIdx == pathCursor

			box := m.renderPathBox(pn, boxWidth, selected)
			boxes = append(boxes, box)

			// Connector to next node
			if j < len(chunk)-1 {
				nextLatency := ""
				if chunk[j+1].latency != "" {
					nextLatency = chunk[j+1].latency
				}
				connector := m.renderConnector(connWidth, nextLatency, globalIdx+1 == pathCursor)
				boxes = append(boxes, connector)
			}
		}

		row := lipgloss.JoinHorizontal(lipgloss.Center, boxes...)
		rows = append(rows, row)

		// Vertical connector between rows
		if end < len(path) {
			down := StylePathLine.Render("        │\n        │\n        ╰──▶")
			rows = append(rows, down)
		}
	}

	content := lipgloss.JoinVertical(lipgloss.Left, rows...)

	title := StyleTitle.Render("  Network Path")
	legend := pathLegend()
	hint := StyleDim.Render("  ◄ ► to traverse │ d deep scan │ v return to tree")

	fullContent := title + "\n" + legend + "\n\n" + content + "\n\n" + hint
	return StylePanel.Width(width).Height(height).Render(fullContent)
}

// renderPathBox renders a single device box in the path view.
func (m Model) renderPathBox(pn pathNode, boxWidth int, selected bool) string {
	icon := m.deviceIcon(pn, selected)

	label := pn.label
	if len(label) > boxWidth-4 {
		label = label[:boxWidth-7] + "..."
	}

	// Style label based on selection
	var styledLabel string
	if selected {
		styledLabel = lipgloss.NewStyle().
			Foreground(ColorCyan).
			Bold(true).
			Render(label)
	} else {
		styledLabel = StyleValue.Render(label)
	}

	// Deep scan badge
	deepBadge := ""
	if pn.id != "__you__" {
		node, ok := m.graph.GetNode(pn.id)
		if ok && node.DeepScanned {
			deepBadge = StyleDeepScan.Render("⬢")
		}
	}

	content := lipgloss.JoinVertical(lipgloss.Center,
		icon+deepBadge,
		styledLabel,
	)

	if selected {
		// Pulsing selection indicator
		pulse := DeepScanFrames[m.spinFrame%len(DeepScanFrames)]
		pointer := StyleSpinner.Render("▼")
		topIndicator := lipgloss.PlaceHorizontal(boxWidth, lipgloss.Center, pointer)

		box := lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(ColorCyan).
			Padding(0, 1).
			Width(boxWidth).
			Render(content)

		_ = pulse
		return topIndicator + "\n" + box
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.deviceColor(pn)).
		Padding(0, 1).
		Width(boxWidth).
		Render(content)

	spacer := strings.Repeat(" ", boxWidth+2)
	return spacer + "\n" + box
}

// renderConnector draws a line between two path boxes.
func (m Model) renderConnector(width int, latency string, nextSelected bool) string {
	lineChar := "─"
	arrow := "▶"

	var lineStyle lipgloss.Style
	if nextSelected {
		// Animate the connection line leading to the selected node
		lineStyle = lipgloss.NewStyle().Foreground(ColorCyan).Bold(true)
	} else {
		lineStyle = StylePathLine
	}

	line := strings.Repeat(lineChar, width-1) + arrow
	styledLine := lineStyle.Render(line)

	top := strings.Repeat(" ", width)

	bot := ""
	if latency != "" {
		bot = lipgloss.PlaceHorizontal(width, lipgloss.Center, StyleDim.Render(latency))
	} else {
		bot = strings.Repeat(" ", width)
	}

	return lipgloss.JoinVertical(lipgloss.Center, top, styledLine, bot)
}

// deviceIcon returns the icon for a device in the path view.
func (m Model) deviceIcon(pn pathNode, selected bool) string {
	if pn.ntype == -1 { // YOU
		if selected {
			// Animate the home icon
			frames := []string{"⌂", "⌂", "⌂", "⌂"}
			icon := frames[m.spinFrame%len(frames)]
			return lipgloss.NewStyle().Foreground(ColorGreen).Bold(true).Render(icon)
		}
		return StyleDeviceYou.Render("⌂")
	}

	// When selected, use a pulsing version
	if selected {
		pulse := []lipgloss.Color{ColorCyan, ColorTeal, ColorCyan, ColorWhite}
		color := pulse[m.spinFrame/2%len(pulse)]
		style := lipgloss.NewStyle().Foreground(color).Bold(true)

		switch pn.ntype {
		case model.NodeTypeHost:
			return style.Render("◆")
		case model.NodeTypeIP:
			return style.Render("●")
		case model.NodeTypeRouter:
			return style.Render("◇")
		default:
			return style.Render("○")
		}
	}

	switch pn.ntype {
	case model.NodeTypeHost:
		node, ok := m.graph.GetNode(pn.id)
		if ok && node.DeepScanned {
			return StyleDeepScan.Render("◆")
		}
		return StyleDeviceServer.Render("█")
	case model.NodeTypeIP:
		return StyleNodeIP.Render("●")
	case model.NodeTypeRouter:
		return StyleDeviceRouter.Render("◇")
	default:
		return StyleDim.Render("○")
	}
}

// deviceColor returns the border color for a device type.
func (m Model) deviceColor(pn pathNode) lipgloss.Color {
	if pn.ntype == -1 {
		return ColorGreen
	}
	switch pn.ntype {
	case model.NodeTypeHost:
		return ColorCyan
	case model.NodeTypeIP:
		return ColorGreen
	case model.NodeTypeRouter:
		return ColorAmber
	default:
		return ColorDim
	}
}

// pathLegend renders a legend for the path view device types.
func pathLegend() string {
	return fmt.Sprintf("  %s You  %s Router  %s Server  %s IP  %s Deep Scanned",
		StyleDeviceYou.Render("⌂"),
		StyleDeviceRouter.Render("◇"),
		StyleDeviceServer.Render("█"),
		StyleNodeIP.Render("●"),
		StyleDeepScan.Render("⬢"),
	)
}
