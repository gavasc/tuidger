package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/paginator"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gavasc/tuidger/internal/components"
	"github.com/gavasc/tuidger/internal/db"
	"github.com/gavasc/tuidger/internal/format"
	"github.com/gavasc/tuidger/internal/styles"
)

const ledgerPageSize = 15

type ledgerMode int

const (
	ledgerModeList ledgerMode = iota
	ledgerModeEdit
	ledgerModeDelete
)

type LedgerModel struct {
	d        *db.DB
	allTxns  []db.Transaction
	filtered []db.Transaction
	accounts []db.Account
	mode     ledgerMode
	table    table.Model
	pag      paginator.Model
	filter   string
	editID   int64
	editForm components.FormModel
	confirm  components.ConfirmModel
	width    int
	height   int
}

func NewLedgerModel(d *db.DB) LedgerModel {
	p := paginator.New()
	p.Type = paginator.Dots
	p.PerPage = ledgerPageSize

	cols := []table.Column{
		{Title: "Date", Width: 12},
		{Title: "Type", Width: 9},
		{Title: "Description", Width: 28},
		{Title: "Category", Width: 16},
		{Title: "Amount", Width: 14},
		{Title: "Account", Width: 14},
	}
	t := table.New(
		table.WithColumns(cols),
		table.WithFocused(true),
		table.WithHeight(ledgerPageSize),
	)
	ts := table.DefaultStyles()
	ts.Header = ts.Header.BorderStyle(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("#555555")).Bold(true)
	ts.Selected = ts.Selected.Foreground(lipgloss.Color("#ffffff")).Background(lipgloss.Color("#4a90d9"))
	t.SetStyles(ts)

	f := components.NewForm("Edit Transaction", "Enter → Save")
	f.AddSelectField("Type", []string{"expense", "revenue"}, true)
	f.AddTextField("Description", true)
	f.AddTextField("Category", true)
	f.AddNumberField("Amount", true)
	f.AddDateField("Date", true)
	f.AddSelectField("Account", []string{}, false)

	return LedgerModel{
		d:        d,
		table:    t,
		pag:      p,
		editForm: f,
	}
}

func (m *LedgerModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.table.SetHeight(h - 5)
	m.refreshTable()
}

func (m LedgerModel) OnTransactionsLoaded(msg TransactionsLoadedMsg) (LedgerModel, tea.Cmd) {
	m.allTxns = msg.Txns
	m.applyFilter()
	m.pag.SetTotalPages(len(m.filtered))
	m.refreshTable()
	return m, nil
}

func (m *LedgerModel) applyFilter() {
	if m.filter == "" {
		m.filtered = m.allTxns
		return
	}
	f := strings.ToLower(m.filter)
	m.filtered = m.filtered[:0]
	for _, t := range m.allTxns {
		if strings.Contains(strings.ToLower(t.Desc), f) ||
			strings.Contains(strings.ToLower(t.Cat), f) {
			m.filtered = append(m.filtered, t)
		}
	}
}

func (m *LedgerModel) refreshTable() {
	start, end := m.pag.GetSliceBounds(len(m.filtered))
	page := m.filtered[start:end]
	rows := make([]table.Row, len(page))
	for i, t := range page {
		accName := ""
		if t.AccountID != nil {
			for _, a := range m.accounts {
				if *a.ID == *t.AccountID {
					accName = a.Name
				}
			}
		}
		var valStr string
		if t.Type == "expense" {
			valStr = "-" + format.Currency(t.Val)
		} else {
			valStr = "+" + format.Currency(t.Val)
		}
		rows[i] = table.Row{format.DateDisplay(t.Date), t.Type, truncate(t.Desc, 28), truncate(t.Cat, 16), valStr, accName}
	}
	m.table.SetRows(rows)
}

func (m LedgerModel) Capturing() bool { return m.mode != ledgerModeList }

func (m LedgerModel) Update(msg tea.Msg) (LedgerModel, tea.Cmd) {
	switch m.mode {
	case ledgerModeEdit:
		return m.updateEdit(msg)
	case ledgerModeDelete:
		return m.updateDelete(msg)
	}

	switch msg := msg.(type) {
	case AccountsLoadedMsg:
		m.accounts = msg.Accounts
		names := accountNames(msg.Accounts)
		if len(m.editForm.Fields) > 5 {
			m.editForm.Fields[5].Options = names
		}
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "e":
			return m.startEdit()
		case "d":
			return m.startDelete()
		case "n":
			m.pag.NextPage()
			m.refreshTable()
		case "p":
			m.pag.PrevPage()
			m.refreshTable()
		case "/":
			// toggle filter (simplified: clear or re-enter — full filter input omitted for brevity)
			m.filter = ""
			m.applyFilter()
			m.refreshTable()
		}
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m LedgerModel) startEdit() (LedgerModel, tea.Cmd) {
	idx := m.table.Cursor()
	start, end := m.pag.GetSliceBounds(len(m.filtered))
	page := m.filtered[start:end]
	if idx >= len(page) {
		return m, nil
	}
	t := page[idx]
	m.editID = *t.ID
	m.editForm.Reset()

	// set type
	for i, opt := range m.editForm.Fields[0].Options {
		if opt == t.Type {
			m.editForm.Fields[0].SelectedIdx = i
		}
	}
	m.editForm.Fields[1].Input.SetValue(t.Desc)
	m.editForm.Fields[2].Input.SetValue(t.Cat)
	m.editForm.Fields[3].Input.SetValue(fmt.Sprintf("%.2f", t.Val))
	m.editForm.Fields[4].Input.SetValue(t.Date)
	if t.AccountID != nil {
		for i, a := range m.accounts {
			if *a.ID == *t.AccountID {
				m.editForm.Fields[5].SelectedIdx = i
			}
		}
	}
	m.editForm.FocusFirst()
	m.mode = ledgerModeEdit
	return m, nil
}

func (m LedgerModel) startDelete() (LedgerModel, tea.Cmd) {
	idx := m.table.Cursor()
	start, end := m.pag.GetSliceBounds(len(m.filtered))
	page := m.filtered[start:end]
	if idx >= len(page) {
		return m, nil
	}
	t := page[idx]
	m.editID = *t.ID
	m.confirm = components.NewConfirm(*t.ID, fmt.Sprintf("Delete '%s' on %s?", t.Desc, format.DateDisplay(t.Date)))
	m.mode = ledgerModeDelete
	return m, nil
}

func (m LedgerModel) updateEdit(msg tea.Msg) (LedgerModel, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			m.mode = ledgerModeList
			return m, nil
		case "enter":
			return m.submitEdit()
		}
	}
	var cmd tea.Cmd
	m.editForm, cmd = m.editForm.Update(msg)
	return m, cmd
}

func (m LedgerModel) submitEdit() (LedgerModel, tea.Cmd) {
	if !m.editForm.Validate() {
		return m, nil
	}
	vals := m.editForm.Values()
	val, _ := format.ParseFloat(vals["Amount"])
	id := m.editID
	var accountID *int64
	for _, a := range m.accounts {
		if a.Name == vals["Account"] {
			aid := *a.ID
			accountID = &aid
			break
		}
	}
	t := db.Transaction{
		ID:        &id,
		Type:      vals["Type"],
		Desc:      vals["Description"],
		Cat:       vals["Category"],
		Val:       val,
		Date:      vals["Date"],
		AccountID: accountID,
	}
	m.mode = ledgerModeList
	return m, updateTransaction(m.d, t)
}

func (m LedgerModel) updateDelete(msg tea.Msg) (LedgerModel, tea.Cmd) {
	var cmd tea.Cmd
	m.confirm, cmd = m.confirm.Update(msg)
	if !m.confirm.Active {
		m.mode = ledgerModeList
	}
	if r, ok := cmd().(components.ConfirmResultMsg); ok && r.Confirmed {
		return m, deleteTransaction(m.d, m.editID)
	}
	return m, cmd
}

func (m LedgerModel) View() string {
	switch m.mode {
	case ledgerModeEdit:
		return m.editForm.View()
	case ledgerModeDelete:
		return m.table.View() + "\n" + m.confirm.View()
	}

	var sb strings.Builder
	sb.WriteString(styles.Faint.Render(fmt.Sprintf("%d transactions  ", len(m.filtered))))
	sb.WriteString(styles.Faint.Render("[e] edit  [d] delete  [n/p] page") + "\n")
	if len(m.filtered) == 0 {
		sb.WriteString(styles.Faint.Render("No transactions in this period."))
		return sb.String()
	}
	sb.WriteString(m.table.View() + "\n")
	sb.WriteString(m.pag.View())
	return sb.String()
}
