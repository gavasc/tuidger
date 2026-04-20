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

	// Pad series to the same length before interpolating.
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

	// Interpolate to smooth the curves — more intermediate points = rounder look.
	const smoothFactor = 4
	expenses = interpolate(expenses, smoothFactor)
	revenues = interpolate(revenues, smoothFactor)

	chartWidth := width - 10
	if chartWidth < 10 {
		chartWidth = 10
	}
	chartHeight := height
	if chartHeight < 3 {
		chartHeight = 3
	}

	plot := asciigraph.PlotMany(
		[][]float64{expenses, revenues},
		asciigraph.Width(chartWidth),
		asciigraph.Height(chartHeight),
		asciigraph.SeriesColors(asciigraph.Red, asciigraph.Green),
		asciigraph.Precision(0),
	)

	legend := styles.Faint.Render("▪ ") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#994444")).Render("expenses") +
		styles.Faint.Render("   ▪ ") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#449944")).Render("revenues")

	return strings.Join([]string{legend, plot}, "\n")
}

// interpolate inserts (factor-1) linearly-spaced points between every pair of
// original data points, making the plotted line appear smoother.
func interpolate(data []float64, factor int) []float64 {
	if len(data) < 2 || factor <= 1 {
		return data
	}
	out := make([]float64, 0, (len(data)-1)*factor+1)
	for i := 0; i < len(data)-1; i++ {
		out = append(out, data[i])
		for j := 1; j < factor; j++ {
			t := float64(j) / float64(factor)
			out = append(out, data[i]*(1-t)+data[i+1]*t)
		}
	}
	out = append(out, data[len(data)-1])
	return out
}
