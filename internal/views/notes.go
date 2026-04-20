package views

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/gavasc/tuidger/internal/db"
	"github.com/gavasc/tuidger/internal/styles"
)

type NotesModel struct {
	d          *db.DB
	expArea    textarea.Model
	revArea    textarea.Model
	focus      int // 0 = expenses, 1 = revenues
	periodFrom string
	periodTo   string
	width      int
	height     int
}

func NewNotesModel(d *db.DB) NotesModel {
	expA := textarea.New()
	expA.Placeholder = "Notes on expenses for this period…"
	expA.ShowLineNumbers = false

	revA := textarea.New()
	revA.Placeholder = "Notes on revenues for this period…"
	revA.ShowLineNumbers = false

	return NotesModel{
		d:       d,
		expArea: expA,
		revArea: revA,
	}
}

func (m *NotesModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	half := w/2 - 4
	areaH := h - 4
	if areaH < 3 {
		areaH = 3
	}
	m.expArea.SetWidth(half)
	m.expArea.SetHeight(areaH)
	m.revArea.SetWidth(half)
	m.revArea.SetHeight(areaH)
}

func (m *NotesModel) OnNoteLoaded(msg NoteLoadedMsg) {
	m.periodFrom = msg.From
	m.periodTo = msg.To
	if msg.Section == "expenses" {
		m.expArea.SetValue(msg.Content)
	} else {
		m.revArea.SetValue(msg.Content)
	}
}

func (m NotesModel) Capturing() bool { return m.expArea.Focused() || m.revArea.Focused() }

func (m NotesModel) Update(msg tea.Msg) (NotesModel, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "tab":
			if m.focus == 0 {
				m.expArea.Blur()
				m.revArea.Focus()
				m.focus = 1
			} else {
				m.revArea.Blur()
				m.expArea.Focus()
				m.focus = 0
			}
			return m, nil
		case "esc":
			m.expArea.Blur()
			m.revArea.Blur()
			return m, nil
		case "ctrl+s":
			return m, m.saveCmd()
		case "enter", "a", "i": // activate editing
			if !m.expArea.Focused() && !m.revArea.Focused() {
				if m.focus == 0 {
					m.expArea.Focus()
				} else {
					m.revArea.Focus()
				}
				return m, nil
			}
		}
	}

	var cmd tea.Cmd
	if m.focus == 0 {
		m.expArea, cmd = m.expArea.Update(msg)
	} else {
		m.revArea, cmd = m.revArea.Update(msg)
	}
	return m, cmd
}

func (m NotesModel) saveCmd() tea.Cmd {
	expContent := m.expArea.Value()
	revContent := m.revArea.Value()
	from := m.periodFrom
	to := m.periodTo
	d := m.d
	return func() tea.Msg {
		if err := d.UpsertNote("expenses", from, to, expContent); err != nil {
			return StatusMsg{Text: "Save error: " + err.Error(), IsErr: true}
		}
		if err := d.UpsertNote("revenues", from, to, revContent); err != nil {
			return StatusMsg{Text: "Save error: " + err.Error(), IsErr: true}
		}
		return StatusMsg{Text: "Notes saved", IsErr: false}
	}
}

func (m NotesModel) View() string {
	var sb strings.Builder
	sb.WriteString(styles.Faint.Render("[Tab] switch area  [Ctrl+S] save  [Esc] blur") + "\n\n")

	expStyle := styles.Box
	revStyle := styles.Box
	if m.focus == 0 {
		expStyle = styles.InputFocus
	} else {
		revStyle = styles.InputFocus
	}

	expPane := expStyle.Render("Expenses\n" + m.expArea.View())
	revPane := revStyle.Render("Revenues\n" + m.revArea.View())
	sb.WriteString(strings.Join([]string{expPane, revPane}, "  "))
	return sb.String()
}
