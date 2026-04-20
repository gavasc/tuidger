package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/paginator"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/gavasc/tuidger/internal/components"
	"github.com/gavasc/tuidger/internal/db"
	"github.com/gavasc/tuidger/internal/format"
	"github.com/gavasc/tuidger/internal/styles"
)

type transfersMode int

const (
	transfersModeList transfersMode = iota
	transfersModeAdd
	transfersModeDelete
)

type TransfersModel struct {
	d         *db.DB
	transfers []db.Transfer
	accounts  []db.Account
	cursor    int
	mode      transfersMode
	addForm   components.FormModel
	confirm   components.ConfirmModel
	pag       paginator.Model
	width     int
	height    int
}

func NewTransfersModel(d *db.DB) TransfersModel {
	p := paginator.New()
	p.Type = paginator.Dots
	p.PerPage = 10

	f := components.NewForm("New Transfer", "Enter → Submit")
	f.AddSelectField("From", []string{}, true)
	f.AddSelectField("To", []string{}, true)
	f.AddNumberField("Amount", true)
	f.AddDateField("Date", true)
	f.AddTextField("Description", false)

	return TransfersModel{d: d, addForm: f, pag: p}
}

func (m *TransfersModel) SetSize(w, h int) { m.width = w; m.height = h }

func (m *TransfersModel) OnAccountsLoaded(msg AccountsLoadedMsg) {
	m.accounts = msg.Accounts
	names := accountNames(msg.Accounts)
	if len(m.addForm.Fields) > 1 {
		m.addForm.Fields[0].Options = names
		m.addForm.Fields[1].Options = names
	}
}

func (m *TransfersModel) OnTransfersLoaded(msg TransfersLoadedMsg) {
	m.transfers = msg.Transfers
	m.pag.SetTotalPages(len(m.transfers))
}

func (m TransfersModel) Capturing() bool { return m.mode != transfersModeList }

func (m TransfersModel) Hints() string {
	switch m.mode {
	case transfersModeAdd:
		return "Tab next field  Enter submit  Esc cancel"
	case transfersModeDelete:
		return "y confirm  n cancel"
	}
	return "[n] new  [d] delete  [↑/↓] navigate  [←/→] page"
}

func (m TransfersModel) Update(msg tea.Msg) (TransfersModel, tea.Cmd) {
	switch m.mode {
	case transfersModeAdd:
		return m.updateAdd(msg)
	case transfersModeDelete:
		return m.updateDelete(msg)
	}

	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "n":
			m.mode = transfersModeAdd
			m.addForm.Reset()
			names := accountNames(m.accounts)
			m.addForm.Fields[0].Options = names
			m.addForm.Fields[1].Options = names
			m.addForm.FocusFirst()
			return m, nil
		case "d":
			start, end := m.pag.GetSliceBounds(len(m.transfers))
			page := m.transfers[start:end]
			if m.cursor < len(page) {
				tr := page[m.cursor]
				m.confirm = components.NewConfirm(*tr.ID, fmt.Sprintf("Delete transfer on %s?", format.DateDisplay(tr.Date)))
				m.mode = transfersModeDelete
			}
			return m, nil
		case "j", "down":
			start, end := m.pag.GetSliceBounds(len(m.transfers))
			if m.cursor < end-start-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "right":
			m.pag.NextPage()
			m.cursor = 0
		case "left":
			m.pag.PrevPage()
			m.cursor = 0
		}
	}
	return m, nil
}

func (m TransfersModel) updateAdd(msg tea.Msg) (TransfersModel, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			m.mode = transfersModeList
			return m, nil
		case "enter":
			return m.submitAdd()
		}
	}
	var cmd tea.Cmd
	m.addForm, cmd = m.addForm.Update(msg)
	return m, cmd
}

func (m TransfersModel) submitAdd() (TransfersModel, tea.Cmd) {
	if !m.addForm.Validate() {
		return m, nil
	}
	vals := m.addForm.Values()
	fromName := vals["From"]
	toName := vals["To"]
	if fromName == toName {
		m.addForm.Fields[1].Error = "From and To must differ"
		return m, nil
	}
	amount, _ := format.ParseFloat(vals["Amount"])
	date := vals["Date"]
	if date == "" {
		date = format.TodayISO()
	}

	var fromID, toID int64
	for _, a := range m.accounts {
		if a.Name == fromName {
			fromID = *a.ID
		}
		if a.Name == toName {
			toID = *a.ID
		}
	}

	tr := db.Transfer{
		FromAccountID: fromID,
		ToAccountID:   toID,
		Amount:        amount,
		Date:          date,
		Desc:          vals["Description"],
	}
	m.mode = transfersModeList
	return m, insertTransfer(m.d, tr)
}

func (m TransfersModel) updateDelete(msg tea.Msg) (TransfersModel, tea.Cmd) {
	var cmd tea.Cmd
	m.confirm, cmd = m.confirm.Update(msg)
	if !m.confirm.Active {
		m.mode = transfersModeList
	}
	if r, ok := callCmd(cmd).(components.ConfirmResultMsg); ok && r.Confirmed {
		return m, deleteTransfer(m.d, m.confirm.ID)
	}
	return m, cmd
}

func (m TransfersModel) View() string {
	var sb strings.Builder

	if len(m.transfers) == 0 {
		sb.WriteString(styles.Faint.Render("No transfers yet. Press [n] to create one.") + "\n")
	} else {
		start, end := m.pag.GetSliceBounds(len(m.transfers))
		page := m.transfers[start:end]
		for i, tr := range page {
			prefix := "  "
			if i == m.cursor {
				prefix = "▶ "
			}
			from, to := "", ""
			if tr.FromAccountName != nil {
				from = *tr.FromAccountName
			}
			if tr.ToAccountName != nil {
				to = *tr.ToAccountName
			}
			sb.WriteString(fmt.Sprintf("%s%s  %-14s → %-14s  %s  %s\n",
				prefix,
				styles.Faint.Render(format.DateDisplay(tr.Date)),
				truncate(from, 14),
				truncate(to, 14),
				styles.RevenueText.Render(format.Currency(tr.Amount)),
				styles.Faint.Render(tr.Desc),
			))
		}
		sb.WriteString("\n" + m.pag.View())
	}
	bg := sb.String()

	switch m.mode {
	case transfersModeAdd:
		return components.RenderModal(m.addForm.View(), m.width, m.height)
	case transfersModeDelete:
		return components.RenderModal(m.confirm.View(), m.width, m.height)
	}
	return bg
}
