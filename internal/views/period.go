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

var modeNames = []string{"1m", "3m", "6m", "1y"}

type PeriodModel struct {
	mode      periodMode
	From, To  string
	fromInput textinput.Model
	toInput   textinput.Model
	customFocus int // 0 = from, 1 = to
	err       string
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
		mode: mode1m,
		From: from,
		To:   to,
		fromInput: fi,
		toInput:   ti,
	}
}

func (p PeriodModel) Update(msg tea.Msg) (PeriodModel, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "1":
			if p.mode != modeCustom {
				p.setMode(mode1m)
				return p, func() tea.Msg { return PeriodChangedMsg{From: p.From, To: p.To} }
			}
		case "2":
			if p.mode != modeCustom {
				p.setMode(mode3m)
				return p, func() tea.Msg { return PeriodChangedMsg{From: p.From, To: p.To} }
			}
		case "3":
			if p.mode != modeCustom {
				p.setMode(mode6m)
				return p, func() tea.Msg { return PeriodChangedMsg{From: p.From, To: p.To} }
			}
		case "4":
			if p.mode != modeCustom {
				p.setMode(mode1y)
				return p, func() tea.Msg { return PeriodChangedMsg{From: p.From, To: p.To} }
			}
		case "c":
			if p.mode != modeCustom {
				p.mode = modeCustom
				p.fromInput.SetValue(p.From)
				p.toInput.SetValue(p.To)
				p.fromInput.Focus()
				p.toInput.Blur()
				p.customFocus = 0
				return p, nil
			}
		case "tab":
			if p.mode == modeCustom {
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
			if p.mode == modeCustom {
				from, err1 := format.ParseDate(p.fromInput.Value())
				to, err2 := format.ParseDate(p.toInput.Value())
				if err1 != nil || err2 != nil {
					p.err = "Invalid date format (use YYYY-MM-DD)"
					return p, nil
				}
				if from > to {
					p.err = "From must be ≤ To"
					return p, nil
				}
				p.err = ""
				p.From = from
				p.To = to
				return p, func() tea.Msg { return PeriodChangedMsg{From: p.From, To: p.To} }
			}
		case "esc":
			if p.mode == modeCustom {
				p.mode = mode1m
				p.setMode(mode1m)
				return p, func() tea.Msg { return PeriodChangedMsg{From: p.From, To: p.To} }
			}
		}
	}

	if p.mode == modeCustom {
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
	names := []string{"1m", "3m", "6m", "1y"}
	if int(m) < len(names) {
		p.From, p.To = format.PeriodDates(names[m])
	}
}

func (p PeriodModel) View() string {
	var parts []string
	for i, name := range modeNames {
		if periodMode(i) == p.mode {
			parts = append(parts, lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#4a90d9")).
				Render("["+name+"]"))
		} else {
			parts = append(parts, styles.Faint.Render("["+name+"]"))
		}
	}
	if p.mode == modeCustom {
		parts = append(parts, lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#4a90d9")).Render("[c]"))
	} else {
		parts = append(parts, styles.Faint.Render("[c]"))
	}

	dateRange := ""
	if p.mode == modeCustom {
		fi := styles.Input.Render(p.fromInput.View())
		ti := styles.Input.Render(p.toInput.View())
		if p.customFocus == 0 {
			fi = styles.InputFocus.Render(p.fromInput.View())
		} else {
			ti = styles.InputFocus.Render(p.toInput.View())
		}
		dateRange = fi + styles.Faint.Render(" → ") + ti
		if p.err != "" {
			dateRange += "  " + styles.Error.Render(p.err)
		}
	} else {
		dateRange = styles.Faint.Render(p.From + " → " + p.To)
	}

	return strings.Join(parts, " ") + "  " + dateRange
}
