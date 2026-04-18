package ui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/bhancock4/netmap/internal/model"
)

// graphNode holds position for force-directed layout.
type graphNode struct {
	id    string
	label string
	ntype model.NodeType
	x, y  float64
	vx, vy float64
	deep  bool
}

// buildGraphLayout computes positions for all nodes using a simple
// force-directed algorithm.
func (m Model) buildGraphLayout(width, height int) []graphNode {
	nodes, edges := m.graph.Snapshot()
	if len(nodes) == 0 {
		return nil
	}

	// Initialize positions in a circle
	gNodes := make([]graphNode, 0, len(nodes))
	nodeIndex := make(map[string]int)
	cx, cy := float64(width)/2, float64(height)/2
	radius := math.Min(float64(width), float64(height)) * 0.35

	i := 0
	for id, n := range nodes {
		angle := 2 * math.Pi * float64(i) / float64(len(nodes))
		gn := graphNode{
			id:    id,
			label: n.Label,
			ntype: n.Type,
			x:     cx + radius*math.Cos(angle),
			y:     cy + radius*math.Sin(angle),
			deep:  n.DeepScanned,
		}
		// Place root at center
		if id == m.graph.Root {
			gn.x = cx
			gn.y = cy
		}
		gNodes = append(gNodes, gn)
		nodeIndex[id] = i
		i++
	}

	// Build adjacency from edges
	type edgePair struct{ from, to int }
	var edgePairs []edgePair
	for _, e := range edges {
		fi, fok := nodeIndex[e.From]
		ti, tok := nodeIndex[e.To]
		if fok && tok {
			edgePairs = append(edgePairs, edgePair{fi, ti})
		}
	}

	// Run force simulation (50 iterations)
	for iter := 0; iter < 50; iter++ {
		cooling := 1.0 - float64(iter)/50.0

		// Repulsion between all nodes
		for i := range gNodes {
			for j := i + 1; j < len(gNodes); j++ {
				dx := gNodes[i].x - gNodes[j].x
				dy := gNodes[i].y - gNodes[j].y
				dist := math.Sqrt(dx*dx+dy*dy) + 0.1
				force := 500.0 / (dist * dist) * cooling

				fx := force * dx / dist
				fy := force * dy / dist
				gNodes[i].vx += fx
				gNodes[i].vy += fy
				gNodes[j].vx -= fx
				gNodes[j].vy -= fy
			}
		}

		// Attraction along edges
		for _, e := range edgePairs {
			dx := gNodes[e.to].x - gNodes[e.from].x
			dy := gNodes[e.to].y - gNodes[e.from].y
			dist := math.Sqrt(dx*dx + dy*dy)
			force := dist * 0.01 * cooling

			fx := force * dx / (dist + 0.1)
			fy := force * dy / (dist + 0.1)
			gNodes[e.from].vx += fx
			gNodes[e.from].vy += fy
			gNodes[e.to].vx -= fx
			gNodes[e.to].vy -= fy
		}

		// Center gravity
		for i := range gNodes {
			gNodes[i].vx += (cx - gNodes[i].x) * 0.001
			gNodes[i].vy += (cy - gNodes[i].y) * 0.001
		}

		// Apply velocity with damping
		for i := range gNodes {
			gNodes[i].x += gNodes[i].vx * 0.5
			gNodes[i].y += gNodes[i].vy * 0.5
			gNodes[i].vx *= 0.8
			gNodes[i].vy *= 0.8

			// Clamp to bounds
			gNodes[i].x = math.Max(2, math.Min(float64(width-2), gNodes[i].x))
			gNodes[i].y = math.Max(1, math.Min(float64(height-1), gNodes[i].y))
		}
	}

	return gNodes
}

// renderGraphView renders the ASCII topology graph.
func (m Model) renderGraphView(width, height int) string {
	graphHeight := height - 4 // room for title + legend + hints
	graphWidth := width - 4

	gNodes := m.buildGraphLayout(graphWidth, graphHeight)
	if len(gNodes) == 0 {
		return StylePanel.Width(width).Render(StyleDim.Render("  No nodes to display"))
	}

	// Create a character grid
	grid := make([][]rune, graphHeight)
	colors := make([][]lipgloss.Color, graphHeight)
	for i := range grid {
		grid[i] = make([]rune, graphWidth)
		colors[i] = make([]lipgloss.Color, graphWidth)
		for j := range grid[i] {
			grid[i][j] = ' '
			colors[i][j] = ColorDim
		}
	}

	// Draw edges first (behind nodes)
	_, edges := m.graph.Snapshot()
	nodePos := make(map[string][2]int)
	for _, gn := range gNodes {
		px, py := int(gn.x), int(gn.y)
		nodePos[gn.id] = [2]int{px, py}
	}

	for _, e := range edges {
		from, fok := nodePos[e.From]
		to, tok := nodePos[e.To]
		if !fok || !tok {
			continue
		}
		drawLine(grid, colors, from[0], from[1], to[0], to[1], ColorDeepCyan)
	}

	// Draw nodes on top
	for _, gn := range gNodes {
		px := int(gn.x)
		py := int(gn.y)
		if py >= 0 && py < graphHeight && px >= 0 && px < graphWidth {
			icon, color := graphNodeIcon(gn)
			grid[py][px] = icon

			// Highlight selected node
			isSelected := gn.id == m.selectedID
			if isSelected {
				color = ColorCyan
				// Draw selection bracket
				if px > 0 {
					grid[py][px-1] = '['
					colors[py][px-1] = ColorCyan
				}
				if px < graphWidth-1 {
					grid[py][px+1] = ']'
					colors[py][px+1] = ColorCyan
				}
			}
			colors[py][px] = color

			// Draw label near the node
			label := gn.label
			if len(label) > 12 {
				label = label[:9] + "..."
			}
			labelStart := px + 2
			if isSelected {
				labelStart = px + 3
			}
			if labelStart+len(label) > graphWidth {
				labelStart = px - len(label) - 1
			}
			if labelStart >= 0 {
				for li, ch := range label {
					lx := labelStart + li
					if lx >= 0 && lx < graphWidth {
						grid[py][lx] = ch
						if isSelected {
							colors[py][lx] = ColorCyan
						} else {
							colors[py][lx] = ColorWhite
						}
					}
				}
			}
		}
	}

	// Render the grid with colors
	var lines []string
	for y := 0; y < graphHeight; y++ {
		var line strings.Builder
		for x := 0; x < graphWidth; x++ {
			style := lipgloss.NewStyle().Foreground(colors[y][x])
			line.WriteString(style.Render(string(grid[y][x])))
		}
		lines = append(lines, line.String())
	}

	content := strings.Join(lines, "\n")

	title := StyleTitle.Render("  Topology Graph")
	legend := fmt.Sprintf("  %s Host  %s IP  %s Router  %s Deep",
		lipgloss.NewStyle().Foreground(ColorCyan).Render("◆"),
		lipgloss.NewStyle().Foreground(ColorGreen).Render("●"),
		lipgloss.NewStyle().Foreground(ColorAmber).Render("◇"),
		lipgloss.NewStyle().Foreground(ColorMagenta).Render("⬢"),
	)
	hint := StyleDim.Render("  g return to tree")

	return title + "\n" + legend + "\n" + content + "\n" + hint
}

func graphNodeIcon(gn graphNode) (rune, lipgloss.Color) {
	if gn.deep {
		return '⬢', ColorMagenta
	}
	switch gn.ntype {
	case model.NodeTypeHost:
		return '◆', ColorCyan
	case model.NodeTypeIP:
		return '●', ColorGreen
	case model.NodeTypeRouter:
		return '◇', ColorAmber
	default:
		return '○', ColorDim
	}
}

// drawLine draws a line between two points using braille-like characters.
func drawLine(grid [][]rune, colors [][]lipgloss.Color, x0, y0, x1, y1 int, color lipgloss.Color) {
	dx := abs(x1 - x0)
	dy := abs(y1 - y0)
	sx := 1
	if x0 > x1 {
		sx = -1
	}
	sy := 1
	if y0 > y1 {
		sy = -1
	}
	err := dx - dy

	maxH := len(grid)
	maxW := 0
	if maxH > 0 {
		maxW = len(grid[0])
	}

	for {
		if y0 >= 0 && y0 < maxH && x0 >= 0 && x0 < maxW {
			if grid[y0][x0] == ' ' {
				// Choose line character based on direction
				if dx > dy*2 {
					grid[y0][x0] = '─'
				} else if dy > dx*2 {
					grid[y0][x0] = '│'
				} else {
					grid[y0][x0] = '·'
				}
				colors[y0][x0] = color
			}
		}

		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x0 += sx
		}
		if e2 < dx {
			err += dx
			y0 += sy
		}
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
