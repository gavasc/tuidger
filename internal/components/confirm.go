package components

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gavasc/tuidger/internal/styles"
)

// ConfirmResultMsg is emitted when the user answers the confirm prompt.
type ConfirmResultMsg struct {
	ID        int64
	Confirmed bool
}

type ConfirmModel struct {
	ID      int64
	Message string
	Active  bool
}

func NewConfirm(id int64, label string) ConfirmModel {
	return ConfirmModel{ID: id, Message: label, Active: true}
}

func (c ConfirmModel) Update(msg tea.Msg) (ConfirmModel, tea.Cmd) {
	if !c.Active {
		return c, nil
	}
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "y", "Y":
			c.Active = false
			return c, func() tea.Msg { return ConfirmResultMsg{ID: c.ID, Confirmed: true} }
		case "n", "N", "esc":
			c.Active = false
			return c, func() tea.Msg { return ConfirmResultMsg{ID: c.ID, Confirmed: false} }
		}
	}
	return c, nil
}

func (c ConfirmModel) View() string {
	if !c.Active {
		return ""
	}
	return fmt.Sprintf("%s  %s / %s",
		styles.Error.Render(c.Message),
		styles.Success.Render("[y]es"),
		styles.Faint.Render("[n]o"),
	)
}
