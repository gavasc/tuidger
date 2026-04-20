package views

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gavasc/tuidger/internal/components"
	"github.com/gavasc/tuidger/internal/db"
	"github.com/gavasc/tuidger/internal/format"
	"github.com/gavasc/tuidger/internal/styles"
)

type accountsMode int

const (
	accountsModeList accountsMode = iota
	accountsModeAdd
	accountsModeDelete
)

type AccountsModel struct {
	d        *db.DB
	balances []db.AccountBalance
	cursor   int
	mode     accountsMode
	addForm  components.FormModel
	confirm  components.ConfirmModel
	width    int
	height   int
}

func NewAccountsModel(d *db.DB) AccountsModel {
	f := components.NewForm("New Account", "Enter → Create")
	f.AddTextField("Name", true)
	f.AddNumberField("Initial Balance", false)
	return AccountsModel{d: d, addForm: f}
}

func (m *AccountsModel) SetSize(w, h int) { m.width = w; m.height = h }

func (m *AccountsModel) OnAccountsLoaded(msg AccountsLoadedMsg) {
	m.balances = msg.Balances
	if m.cursor >= len(m.balances) && m.cursor > 0 {
		m.cursor = len(m.balances) - 1
	}
}

func (m AccountsModel) Update(msg tea.Msg) (AccountsModel, tea.Cmd) {
	switch m.mode {
	case accountsModeAdd:
		return m.updateAdd(msg)
	case accountsModeDelete:
		return m.updateDelete(msg)
	}

	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "n":
			m.mode = accountsModeAdd
			m.addForm.Reset()
			m.addForm.FocusFirst()
			return m, nil
		case "d":
			if len(m.balances) > 0 {
				ab := m.balances[m.cursor]
				m.confirm = components.NewConfirm(ab.ID, fmt.Sprintf("Delete account '%s'?", ab.Name))
				m.mode = accountsModeDelete
			}
			return m, nil
		case "j":
			if m.cursor < len(m.balances)-1 {
				m.cursor++
			}
		case "k":
			if m.cursor > 0 {
				m.cursor--
			}
		}
	}
	return m, nil
}

func (m AccountsModel) updateAdd(msg tea.Msg) (AccountsModel, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			m.mode = accountsModeList
			return m, nil
		case "enter":
			return m.submitAdd()
		}
	}
	var cmd tea.Cmd
	m.addForm, cmd = m.addForm.Update(msg)
	return m, cmd
}

func (m AccountsModel) submitAdd() (AccountsModel, tea.Cmd) {
	if !m.addForm.Validate() {
		return m, nil
	}
	vals := m.addForm.Values()
	bal, _ := format.ParseFloat(vals["Initial Balance"])
	a := db.Account{Name: vals["Name"], InitialBalance: bal}
	m.mode = accountsModeList
	return m, insertAccount(m.d, a)
}

func (m AccountsModel) updateDelete(msg tea.Msg) (AccountsModel, tea.Cmd) {
	var cmd tea.Cmd
	m.confirm, cmd = m.confirm.Update(msg)
	if !m.confirm.Active {
		m.mode = accountsModeList
	}
	if r, ok := callCmd(cmd).(components.ConfirmResultMsg); ok && r.Confirmed {
		return m, deleteAccount(m.d, m.confirm.ID)
	}
	return m, cmd
}

func callCmd(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	return cmd()
}

func (m AccountsModel) View() string {
	if m.mode == accountsModeAdd {
		return m.addForm.View()
	}

	var sb strings.Builder
	sb.WriteString(styles.Faint.Render("[n] New  [d] Delete  [j/k] Navigate") + "\n\n")

	if len(m.balances) == 0 {
		sb.WriteString(styles.Faint.Render("No accounts yet. Press [n] to create one.") + "\n")
		return sb.String()
	}

	var total float64
	for i, ab := range m.balances {
		prefix := "  "
		if i == m.cursor {
			prefix = "▶ "
		}
		balStr := format.Currency(ab.Balance)
		if ab.Balance >= 0 {
			balStr = styles.RevenueText.Render(balStr)
		} else {
			balStr = styles.ExpenseText.Render(balStr)
		}
		line := fmt.Sprintf("%s%-30s %s", prefix, ab.Name, balStr)
		if m.mode == accountsModeDelete && i == m.cursor {
			line += "  " + m.confirm.View()
		}
		sb.WriteString(line + "\n")
		total += ab.Balance
	}

	sb.WriteString("\n")
	totalStr := format.Currency(total)
	if total >= 0 {
		totalStr = styles.RevenueText.Render("Total: " + totalStr)
	} else {
		totalStr = styles.ExpenseText.Render("Total: " + totalStr)
	}
	sb.WriteString("  " + totalStr + "\n")
	return sb.String()
}
