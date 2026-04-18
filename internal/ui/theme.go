package ui

import "github.com/charmbracelet/lipgloss"

// Sonar color palette — cyan/teal + electric green on dark background
var (
	// Primary colors
	ColorCyan      = lipgloss.Color("#00E5FF")
	ColorTeal      = lipgloss.Color("#00BFA5")
	ColorGreen     = lipgloss.Color("#69F0AE")
	ColorDimGreen  = lipgloss.Color("#2E7D32")
	ColorAmber     = lipgloss.Color("#FFB74D")
	ColorRed       = lipgloss.Color("#FF5252")
	ColorDimRed    = lipgloss.Color("#B71C1C")
	ColorWhite     = lipgloss.Color("#E0E0E0")
	ColorDim       = lipgloss.Color("#616161")
	ColorBg        = lipgloss.Color("#0D1117")
	ColorBgPanel   = lipgloss.Color("#161B22")
	ColorHighlight = lipgloss.Color("#1A3A4A")
	ColorMagenta   = lipgloss.Color("#E040FB")
	ColorDeepCyan  = lipgloss.Color("#006064")

	// Styles
	StyleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorCyan).
			PaddingLeft(1)

	StyleSubtitle = lipgloss.NewStyle().
			Foreground(ColorTeal)

	StyleLabel = lipgloss.NewStyle().
			Foreground(ColorDim).
			Width(20).
			Align(lipgloss.Right).
			PaddingRight(1)

	StyleValue = lipgloss.NewStyle().
			Foreground(ColorWhite)

	StyleSuccess = lipgloss.NewStyle().
			Foreground(ColorGreen)

	StyleWarning = lipgloss.NewStyle().
			Foreground(ColorAmber)

	StyleError = lipgloss.NewStyle().
			Foreground(ColorRed)

	StyleDim = lipgloss.NewStyle().
			Foreground(ColorDim)

	StyleSelected = lipgloss.NewStyle().
			Background(ColorHighlight).
			Foreground(ColorCyan).
			Bold(true)

	StyleNodeHost = lipgloss.NewStyle().
			Foreground(ColorCyan).
			Bold(true)

	StyleNodeIP = lipgloss.NewStyle().
			Foreground(ColorGreen)

	StyleNodeRouter = lipgloss.NewStyle().
			Foreground(ColorAmber)

	StyleTreeBranch = lipgloss.NewStyle().
			Foreground(ColorDim)

	StyleBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorTeal)

	StylePanel = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorDim).
			Padding(0, 1)

	StyleStatusBar = lipgloss.NewStyle().
			Foreground(ColorDim).
			PaddingLeft(1)

	StyleSpinner = lipgloss.NewStyle().
			Foreground(ColorCyan)

	StyleCyan = lipgloss.NewStyle().
			Foreground(ColorCyan).
			Bold(true)

	StyleLogo = lipgloss.NewStyle().
			Foreground(ColorCyan).
			Bold(true)

	StyleDeepScan = lipgloss.NewStyle().
			Foreground(ColorMagenta).
			Bold(true)

	StylePathBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(0, 1)

	StylePathBoxSelected = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(ColorCyan).
			Padding(0, 1)

	StylePathLine = lipgloss.NewStyle().
			Foreground(ColorDim)

	// Device colors for path view
	StyleDeviceYou    = lipgloss.NewStyle().Foreground(ColorGreen).Bold(true)
	StyleDeviceRouter = lipgloss.NewStyle().Foreground(ColorAmber).Bold(true)
	StyleDeviceServer = lipgloss.NewStyle().Foreground(ColorCyan).Bold(true)
	StyleDeviceCloud  = lipgloss.NewStyle().Foreground(ColorMagenta).Bold(true)
)

// Logo is the ASCII art banner.
const Logo = `
 ███╗   ██╗███████╗████████╗███╗   ███╗ █████╗ ██████╗
 ████╗  ██║██╔════╝╚══██╔══╝████╗ ████║██╔══██╗██╔══██╗
 ██╔██╗ ██║█████╗     ██║   ██╔████╔██║███████║██████╔╝
 ██║╚██╗██║██╔══╝     ██║   ██║╚██╔╝██║██╔══██║██╔═══╝
 ██║ ╚████║███████╗   ██║   ██║ ╚═╝ ██║██║  ██║██║
 ╚═╝  ╚═══╝╚══════╝   ╚═╝   ╚═╝     ╚═╝╚═╝  ╚═╝╚═╝     `

// Jellyfish mascot — bioluminescent deep-sea sonar vibes
// Animated by cycling through these frames
var JellyfishFrames = [][]string{
	{
		"         ╭───────╮         ",
		"       ╭─┤ ◉   ◉ ├─╮       ",
		"      ╭┤ ╰───┬───╯ ├╮      ",
		"      │╰─────┴─────╯│      ",
		"      ╰──┬──┬─┬──┬──╯      ",
		"         │  ╰╮│  │         ",
		"         ╰╮  ││ ╭╯         ",
		"          │  ╰╯  │         ",
		"          ╰╮   ╭╯          ",
		"           ╰───╯           ",
	},
	{
		"         ╭───────╮         ",
		"       ╭─┤ ◉   ◉ ├─╮       ",
		"      ╭┤ ╰───┬───╯ ├╮      ",
		"      │╰─────┴─────╯│      ",
		"      ╰──┬──┬─┬──┬──╯      ",
		"        ╭╯  │╰╮  │         ",
		"        │  ╭╯ │  ╰╮        ",
		"        ╰╮ │  ╰╮  │        ",
		"         ╰╮╰╮  │╭╯         ",
		"          ╰─┴──╯           ",
	},
	{
		"         ╭───────╮         ",
		"       ╭─┤ ◉   ◉ ├─╮       ",
		"      ╭┤ ╰───┬───╯ ├╮      ",
		"      │╰─────┴─────╯│      ",
		"      ╰──┬──┬─┬──┬──╯      ",
		"         ╰╮ │ │ ╭╯         ",
		"          │ │╭╯ │          ",
		"         ╭╯ ││ ╭╯          ",
		"         │  ╰╯  │          ",
		"         ╰──────╯          ",
	},
	{
		"         ╭───────╮         ",
		"       ╭─┤ ◠   ◠ ├─╮       ",
		"      ╭┤ ╰───┬───╯ ├╮      ",
		"      │╰─────┴─────╯│      ",
		"      ╰──┬──┬─┬──┬──╯      ",
		"        ╭╯  ╰╮│ ╭╯         ",
		"        │   ╭╯│ │          ",
		"        ╰╮  │ ╰╮╰╮         ",
		"         ╰──╯  │ │         ",
		"               ╰─╯         ",
	},
}

// Device icons for the visual path view
const (
	DeviceYou = `┌─────┐
│ ▓▓▓ │
│ YOU │
└──┬──┘`

	DeviceRouter = `  ╱╲
 ╱  ╲
╱ ◇◇ ╲
╲    ╱
 ╲  ╱
  ╲╱`

	DeviceServer = `┌─────┐
│║║║║║│
│║║║║║│
│ ●●● │
└─────┘`

	DeviceCloud = ` ╭────╮
╭┤    ├╮
│ ╰──╯ │
╰──────╯`

	DeviceUnknown = `╭─────╮
│  ?  │
╰─────╯`
)

// Compact device icons for tighter path view
const (
	DeviceYouSmall    = "[YOU]"
	DeviceRouterSmall = "<◇>"
	DeviceServerSmall = "[█]"
	DeviceCloudSmall  = "(☁)"
)

// Spinners — fun loading animation frames
var SpinnerFrames = []string{
	"◐", "◓", "◑", "◒",
}

var SonarFrames = []string{
	"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏",
}

var RadarFrames = []string{
	"▁", "▂", "▃", "▄", "▅", "▆", "▇", "█", "▇", "▆", "▅", "▄", "▃", "▂",
}

var PulseFrames = []string{
	"░", "▒", "▓", "█", "▓", "▒",
}

// Deep scan animation — sonar ripple effect
var DeepScanFrames = []string{
	"◯",
	"◎",
	"◉",
	"●",
	"◉",
	"◎",
}
