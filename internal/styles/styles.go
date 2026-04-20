package styles

import "github.com/charmbracelet/lipgloss"

const (
	ColorExpense = lipgloss.Color("#8c1f1f")
	ColorRevenue = lipgloss.Color("#1a5c30")
	ColorFaint   = lipgloss.Color("#555555")
	ColorAccent  = lipgloss.Color("#4a90d9")
	ColorError   = lipgloss.Color("#b71c1c")
	ColorSuccess = lipgloss.Color("#2e7d32")
	ColorNeutral = lipgloss.Color("#cccccc")
)

var (
	TabActive = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#ffffff")).
			Background(lipgloss.Color("#4a90d9")).
			Padding(0, 1)

	TabInactive = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			Padding(0, 1)

	Box = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#444444")).
		Padding(0, 1)

	Button = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#cccccc")).
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#555555")).
		Padding(0, 2)

	ButtonFocus = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffffff")).
			Bold(true).
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#4a90d9")).
			Padding(0, 2)

	Input = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#555555")).
		Padding(0, 1)

	InputFocus = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#4a90d9")).
			Padding(0, 1)

	Error = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#b71c1c"))

	Success = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#2e7d32"))

	Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#ffffff"))

	Faint = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#555555"))

	ExpenseText = lipgloss.NewStyle().
			Foreground(ColorExpense)

	RevenueText = lipgloss.NewStyle().
			Foreground(ColorRevenue)
)
