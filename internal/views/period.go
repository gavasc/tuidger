package views

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gavasc/tuidger/internal/format"
	"github.com/gavasc/tuidger/internal/styles"
)

type periodMode int

const (
	mode1m periodMode = iota
	mode3m
	mode6m
	mode1y
	modeCustom
)

var presetModes = []periodMode{mode1m, mode3m, mode6m, mode1y}
var modeNames = []string{"1m", "3m", "6m", "1y"}

type PeriodModel struct {
	mode        periodMode
	editing     bool // true while the custom date modal is open
	From, To    string
	fromInput   textinput.Model
	toInput     textinput.Model
	customFocus int // 0 = from, 1 = to
	err         string
}

func NewPeriodModel() PeriodModel {
	from, to := format.PeriodDates("1m")
	fi := textinput.New()
	fi.Prompt = ""
	fi.Width = 10
	fi.SetValue(from)

	ti := textinput.New()
	ti.Prompt = ""
	ti.Width = 10
	ti.SetValue(to)

	return PeriodModel{
		mode:      mode1m,
		From:      from,
		To:        to,
		fromInput: fi,
		toInput:   ti,
	}
}

func (p PeriodModel) Update(msg tea.Msg) (PeriodModel, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {

		// [ cycles backward through presets, ] cycles forward
		case "[":
			idx := int(p.mode)
			if p.mode == modeCustom {
				idx = 0
			} else {
				idx = (idx - 1 + len(presetModes)) % len(presetModes)
			}
			p.setMode(presetModes[idx])
			return p, func() tea.Msg { return PeriodChangedMsg{From: p.From, To: p.To} }

		case "]":
			idx := int(p.mode)
			if p.mode == modeCustom {
				idx = 0
			} else {
				idx = (idx + 1) % len(presetModes)
			}
			p.setMode(presetModes[idx])
			return p, func() tea.Msg { return PeriodChangedMsg{From: p.From, To: p.To} }


		case "p":
			// Open the custom date editor (re-open if already in custom mode)
			p.mode = modeCustom
			p.editing = true
			p.fromInput.SetValue(p.From)
			p.toInput.SetValue(p.To)
			p.fromInput.Focus()
			p.toInput.Blur()
			p.customFocus = 0
			p.err = ""
			return p, nil

		case "tab":
			if p.editing {
				if p.customFocus == 0 {
					p.fromInput.Blur()
					p.toInput.Focus()
					p.customFocus = 1
				} else {
					p.toInput.Blur()
					p.fromInput.Focus()
					p.customFocus = 0
				}
				return p, nil
			}

		case "enter":
			if p.editing {
				from, err1 := format.ParseDate(p.fromInput.Value())
				to, err2 := format.ParseDate(p.toInput.Value())
				if err1 != nil || err2 != nil {
					p.err = "Invalid date (use YYYY-MM-DD)"
					return p, nil
				}
				if from > to {
					p.err = "From must be ≤ To"
					return p, nil
				}
				p.err = ""
				p.From = from
				p.To = to
				p.editing = false // close modal, stay in modeCustom
				p.fromInput.Blur()
				p.toInput.Blur()
				return p, func() tea.Msg { return PeriodChangedMsg{From: p.From, To: p.To} }
			}

		case "esc":
			if p.editing {
				p.editing = false
				p.fromInput.Blur()
				p.toInput.Blur()
				if p.mode == modeCustom && p.From == "" {
					// Never applied a custom range — revert to 1m
					p.setMode(mode1m)
					return p, func() tea.Msg { return PeriodChangedMsg{From: p.From, To: p.To} }
				}
				return p, nil
			}
		}
	}

	if p.editing {
		var cmd tea.Cmd
		if p.customFocus == 0 {
			p.fromInput, cmd = p.fromInput.Update(msg)
		} else {
			p.toInput, cmd = p.toInput.Update(msg)
		}
		return p, cmd
	}
	return p, nil
}

func (p *PeriodModel) setMode(m periodMode) {
	p.mode = m
	p.editing = false
	if int(m) < len(modeNames) {
		p.From, p.To = format.PeriodDates(modeNames[m])
	}
}

// ModalActive reports whether the custom date picker modal should be shown.
func (p PeriodModel) ModalActive() bool { return p.editing }

// ModalView returns the content to render inside the custom date picker modal.
func (p PeriodModel) ModalView() string {
	// Do NOT wrap textinput.View() in any lipgloss Render() call — it strips
	// the ESC byte from the cursor sequence, leaving "[7m [0m" as literal text.
	// Focus is shown via the label colour instead.
	fromLabel := styles.Faint.Render("From: ")
	toLabel := styles.Faint.Render("To:   ")
	if p.customFocus == 0 {
		fromLabel = lipgloss.NewStyle().Foreground(lipgloss.Color("#4a90d9")).Bold(true).Render("From: ")
	} else {
		toLabel = lipgloss.NewStyle().Foreground(lipgloss.Color("#4a90d9")).Bold(true).Render("To:   ")
	}

	var sb strings.Builder
	sb.WriteString(styles.Title.Render("Custom Date Range") + "\n\n")
	sb.WriteString(fromLabel + p.fromInput.View() + "\n")
	sb.WriteString(toLabel + p.toInput.View() + "\n")
	if p.err != "" {
		sb.WriteString("\n" + styles.Error.Render(p.err) + "\n")
	}
	sb.WriteString("\n" + styles.Faint.Render("Tab switch  Enter apply  Esc cancel"))
	return sb.String()
}

func (p PeriodModel) View() string {
	var parts []string
	for i, name := range modeNames {
		if periodMode(i) == p.mode {
			parts = append(parts, lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#4a90d9")).
				Render(name))
		} else {
			parts = append(parts, styles.Faint.Render(name))
		}
	}

	var modeLabel string
	if p.mode == modeCustom {
		modeLabel = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#4a90d9")).Render("custom")
	} else {
		modeLabel = styles.Faint.Render("[p]custom")
	}

	modeBar := strings.Join(parts, styles.Faint.Render(" | ")) + "  " + modeLabel
	dateRange := styles.Faint.Render(p.From + " → " + p.To)

	nav := styles.Faint.Render("[ ] cycle")
	return nav + "  " + modeBar + "  " + dateRange
}
