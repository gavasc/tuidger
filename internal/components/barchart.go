package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/gavasc/tuidger/internal/format"
	"github.com/gavasc/tuidger/internal/styles"
)

type BarEntry struct {
	Label string
	Value float64
	Pct   float64
	Color lipgloss.Color
}

// RenderBarChart renders a horizontal bar chart with labels, bars, percentages and values.
func RenderBarChart(entries []BarEntry, maxWidth int, title string) string {
	if len(entries) == 0 {
		return styles.Faint.Render("No data")
	}

	// find longest label
	maxLabel := 0
	for _, e := range entries {
		if len(e.Label) > maxLabel {
			maxLabel = len(e.Label)
		}
	}

	// " 34.5%  R$ 1.234,56" = ~22 chars
	const metaWidth = 22
	barArea := maxWidth - maxLabel - metaWidth - 2
	if barArea < 4 {
		barArea = 4
	}

	var sb strings.Builder
	if title != "" {
		sb.WriteString(styles.Title.Render(title) + "\n")
	}

	for _, e := range entries {
		label := fmt.Sprintf("%-*s", maxLabel, e.Label)
		filled := int(e.Pct / 100.0 * float64(barArea))
		if filled > barArea {
			filled = barArea
		}
		empty := barArea - filled
		bar := lipgloss.NewStyle().Foreground(e.Color).Render(strings.Repeat("█", filled)) +
			styles.Faint.Render(strings.Repeat("░", empty))
		meta := fmt.Sprintf(" %5.1f%%  %s", e.Pct, format.Currency(e.Value))
		sb.WriteString(label + " " + bar + meta + "\n")
	}
	return sb.String()
}
