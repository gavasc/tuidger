package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/gavasc/tuidger/internal/db"
	"github.com/gavasc/tuidger/internal/format"
	"github.com/gavasc/tuidger/internal/styles"
)

type exportMode int

const (
	exportModeIdle exportMode = iota
	exportModeConfirm
)

type ExportModel struct {
	d         *db.DB
	mode      exportMode
	format    string // "json" | "csv"
	pathInput textinput.Model
}

func NewExportModel(d *db.DB) ExportModel {
	ti := textinput.New()
	ti.Prompt = ""
	ti.Width = 48
	return ExportModel{d: d, pathInput: ti}
}

// Capturing returns true while the path input is open.
func (m ExportModel) Capturing() bool { return m.mode == exportModeConfirm }

func (m ExportModel) OnExportDone(msg ExportDoneMsg) (ExportModel, tea.Cmd) {
	m.mode = exportModeIdle
	m.pathInput.Blur()
	return m, nil
}

func (m ExportModel) Update(msg tea.Msg) (ExportModel, tea.Cmd) {
	if m.mode == exportModeConfirm {
		if key, ok := msg.(tea.KeyMsg); ok {
			switch key.String() {
			case "esc":
				m.mode = exportModeIdle
				m.pathInput.Blur()
				return m, nil
			case "enter":
				path := m.pathInput.Value()
				m.pathInput.Blur()
				return m, exportCmd(m.d, m.format, path)
			}
		}
		var cmd tea.Cmd
		m.pathInput, cmd = m.pathInput.Update(msg)
		return m, cmd
	}

	// Activate from bottom-bar key presses
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "j":
			m.format = "json"
			m.pathInput.SetValue(defaultPath("json"))
			m.pathInput.Focus()
			m.mode = exportModeConfirm
		case "c":
			m.format = "csv"
			m.pathInput.SetValue(defaultPath("csv"))
			m.pathInput.Focus()
			m.mode = exportModeConfirm
		}
	}
	return m, nil
}

// ModalView returns the content to render inside the export modal.
func (m ExportModel) ModalView() string {
	// Do NOT wrap pathInput.View() in lipgloss — it strips the cursor ESC byte.
	title := styles.Title.Render("Export " + strings.ToUpper(m.format))
	pathLine := styles.Faint.Render("Save to: ") + m.pathInput.View()
	hint := styles.Faint.Render("Enter confirm  Esc cancel")
	return title + "\n\n" + pathLine + "\n\n" + hint
}

func defaultPath(ext string) string {
	return fmt.Sprintf("~/tuidger-export-%s.%s", format.TodayISO(), ext)
}
