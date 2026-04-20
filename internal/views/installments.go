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

type installmentsMode int

const (
	installmentsModeList installmentsMode = iota
	installmentsModeAdd
	installmentsModeDelete
)

type InstallmentsModel struct {
	d            *db.DB
	installments []db.Installment
	accounts     []db.Account
	cursor       int
	mode         installmentsMode
	addForm      components.FormModel
	confirm      components.ConfirmModel
	width        int
	height       int
}

func NewInstallmentsModel(d *db.DB) InstallmentsModel {
	f := components.NewForm("New Installment", "Enter → Create")
	f.AddTextField("Description", true)
	f.AddTextField("Category", true)
	f.AddNumberField("Total Value", true)
	f.AddNumberField("N installments", true)
	f.AddDateField("Start Date", true)
	f.AddSelectField("Account", []string{}, false)
	return InstallmentsModel{d: d, addForm: f}
}

func (m *InstallmentsModel) SetSize(w, h int) { m.width = w; m.height = h }

func (m *InstallmentsModel) OnInstallmentsLoaded(msg InstallmentsLoadedMsg) {
	m.installments = msg.Installments
}

func (m InstallmentsModel) Capturing() bool { return m.mode != installmentsModeList }

func (m InstallmentsModel) Hints() string {
	switch m.mode {
	case installmentsModeAdd:
		return "Tab next field  Enter create  Esc cancel"
	case installmentsModeDelete:
		return "y confirm  n cancel"
	}
	return "[n] new  [d] delete  [↑/↓] navigate"
}

func (m InstallmentsModel) Update(msg tea.Msg) (InstallmentsModel, tea.Cmd) {
	switch m.mode {
	case installmentsModeAdd:
		return m.updateAdd(msg)
	case installmentsModeDelete:
		return m.updateDelete(msg)
	}

	switch msg := msg.(type) {
	case AccountsLoadedMsg:
		m.accounts = msg.Accounts
		names := accountNames(msg.Accounts)
		if len(m.addForm.Fields) > 5 {
			m.addForm.Fields[5].Options = names
		}
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "n":
			m.mode = installmentsModeAdd
			m.addForm.Reset()
			names := accountNames(m.accounts)
			if len(m.addForm.Fields) > 5 {
				m.addForm.Fields[5].Options = names
			}
			m.addForm.FocusFirst()
			return m, nil
		case "d":
			if len(m.installments) > 0 {
				inst := m.installments[m.cursor]
				m.confirm = components.NewConfirm(*inst.ID,
					fmt.Sprintf("Delete '%s' and all its transactions?", inst.Desc))
				m.mode = installmentsModeDelete
			}
			return m, nil
		case "j", "down":
			if m.cursor < len(m.installments)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		}
	}
	return m, nil
}

func (m InstallmentsModel) updateAdd(msg tea.Msg) (InstallmentsModel, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			m.mode = installmentsModeList
			return m, nil
		case "enter":
			return m.submitAdd()
		}
	}
	var cmd tea.Cmd
	m.addForm, cmd = m.addForm.Update(msg)
	return m, cmd
}

func (m InstallmentsModel) submitAdd() (InstallmentsModel, tea.Cmd) {
	if !m.addForm.Validate() {
		return m, nil
	}
	vals := m.addForm.Values()
	totalVal, _ := format.ParseFloat(vals["Total Value"])
	n, _ := format.ParseFloat(vals["N installments"])
	var accountID *int64
	for _, a := range m.accounts {
		if a.Name == vals["Account"] {
			id := *a.ID
			accountID = &id
			break
		}
	}
	inst := db.Installment{
		Desc:          vals["Description"],
		Cat:           vals["Category"],
		TotalVal:      totalVal,
		NInstallments: int64(n),
		StartDate:     vals["Start Date"],
		AccountID:     accountID,
	}
	m.mode = installmentsModeList
	return m, insertInstallment(m.d, inst)
}

func (m InstallmentsModel) updateDelete(msg tea.Msg) (InstallmentsModel, tea.Cmd) {
	var cmd tea.Cmd
	m.confirm, cmd = m.confirm.Update(msg)
	if !m.confirm.Active {
		m.mode = installmentsModeList
	}
	if r, ok := callCmd(cmd).(components.ConfirmResultMsg); ok && r.Confirmed {
		return m, deleteInstallment(m.d, m.confirm.ID)
	}
	return m, cmd
}

func (m InstallmentsModel) View() string {
	var sb strings.Builder

	if len(m.installments) == 0 {
		sb.WriteString(styles.Faint.Render("No installments yet. Press [n] to create one.") + "\n")
	} else {
		for i, inst := range m.installments {
			prefix := "  "
			if i == m.cursor {
				prefix = "▶ "
			}
			paid := int64(0)
			if inst.PaidCount != nil {
				paid = *inst.PaidCount
			}
			monthly := 0.0
			if inst.MonthlyVal != nil {
				monthly = *inst.MonthlyVal
			}
			total := inst.NInstallments
			barFilled := int(paid)
			barEmpty := int(total) - barFilled
			if barEmpty < 0 {
				barEmpty = 0
			}
			bar := styles.RevenueText.Render(strings.Repeat("█", barFilled)) +
				styles.Faint.Render(strings.Repeat("░", barEmpty))

			sb.WriteString(fmt.Sprintf("%s%-24s %-12s %s×%d  %s %d/%d  %s\n",
				prefix,
				truncate(inst.Desc, 24),
				truncate(inst.Cat, 12),
				format.Currency(monthly),
				total,
				bar,
				paid, total,
				styles.Faint.Render(format.DateDisplay(inst.StartDate)),
			))
		}
	}
	bg := sb.String()

	switch m.mode {
	case installmentsModeAdd:
		return components.RenderModal(m.addForm.View(), m.width, m.height)
	case installmentsModeDelete:
		return components.RenderModal(m.confirm.View(), m.width, m.height)
	}
	return bg
}
