package components

import "github.com/charmbracelet/lipgloss"

var modalBoxStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("#4a90d9")).
	Padding(1, 3)

// RenderModal centers content in a rounded, blue-bordered box over a dark
// background that fills the given width/height.
func RenderModal(content string, w, h int) string {
	box := modalBoxStyle.Render(content)
	return lipgloss.Place(
		w, h,
		lipgloss.Center, lipgloss.Center,
		box,
		lipgloss.WithWhitespaceBackground(lipgloss.Color("#0d0d0d")),
	)
}
