package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/guptarohit/asciigraph"
	"github.com/gavasc/tuidger/internal/styles"
)

// RenderLineChart renders a dual-series line chart (expenses red, revenues green).
func RenderLineChart(expenses, revenues []float64, width, height int) string {
	if len(expenses) == 0 && len(revenues) == 0 {
		return styles.Faint.Render("No data for this period")
	}

	// pad series to same length
	maxLen := len(expenses)
	if len(revenues) > maxLen {
		maxLen = len(revenues)
	}
	for len(expenses) < maxLen {
		expenses = append(expenses, 0)
	}
	for len(revenues) < maxLen {
		revenues = append(revenues, 0)
	}

	chartWidth := width - 10
	if chartWidth < 10 {
		chartWidth = 10
	}
	chartHeight := height
	if chartHeight < 4 {
		chartHeight = 4
	}

	plot := asciigraph.PlotMany(
		[][]float64{expenses, revenues},
		asciigraph.Width(chartWidth),
		asciigraph.Height(chartHeight),
		asciigraph.SeriesColors(asciigraph.Red, asciigraph.Green),
	)

	legend := lipgloss.NewStyle().Foreground(lipgloss.Color("#cc0000")).Render("── expenses") +
		"  " +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#00aa00")).Render("── revenues")

	return strings.Join([]string{legend, plot}, "\n")
}
