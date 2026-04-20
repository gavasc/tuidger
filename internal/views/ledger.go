package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/paginator"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gavasc/tuidger/internal/components"
	"github.com/gavasc/tuidger/internal/db"
	"github.com/gavasc/tuidger/internal/format"
	"github.com/gavasc/tuidger/internal/styles"
)

const ledgerPageSize = 40

// column widths for the custom renderer
const (
	colDate    = 12
	colType    = 9
	colDesc    = 26
	colCat     = 14
	colAmount  = 14
	colAccount = 14
)

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
	pag      paginator.Model
	filter   string
	cursor   int
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

	f := components.NewForm("Edit Transaction", "Enter → Save")
	f.AddSelectField("Type", []string{"expense", "revenue"}, true)
	f.AddTextField("Description", true)
	f.AddTextField("Category", true)
	f.AddNumberField("Amount", true)
	f.AddDateField("Date", true)
	f.AddSelectField("Account", []string{}, false)

	return LedgerModel{d: d, pag: p, editForm: f}
}

func (m *LedgerModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

func (m LedgerModel) OnTransactionsLoaded(msg TransactionsLoadedMsg) (LedgerModel, tea.Cmd) {
	m.allTxns = msg.Txns
	m.applyFilter()
	m.pag.SetTotalPages(len(m.filtered))
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

func (m LedgerModel) Capturing() bool { return m.mode != ledgerModeList }

func (m LedgerModel) Hints() string {
	switch m.mode {
	case ledgerModeEdit:
		return "Tab next field  Enter save  Esc cancel"
	case ledgerModeDelete:
		return "y confirm  n cancel"
	}
	return "[e] edit  [d] delete  [n] next page  [p] prev page"
}

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
		case "j", "down":
			start, end := m.pag.GetSliceBounds(len(m.filtered))
			if m.cursor < end-start-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "n":
			m.pag.NextPage()
			m.cursor = 0
		case "p":
			m.pag.PrevPage()
			m.cursor = 0
		case "/":
			m.filter = ""
			m.applyFilter()
			m.pag.SetTotalPages(len(m.filtered))
		}
	}
	return m, nil
}

func (m LedgerModel) startEdit() (LedgerModel, tea.Cmd) {
	start, end := m.pag.GetSliceBounds(len(m.filtered))
	page := m.filtered[start:end]
	if m.cursor >= len(page) {
		return m, nil
	}
	t := page[m.cursor]
	m.editID = *t.ID
	m.editForm.Reset()
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
	start, end := m.pag.GetSliceBounds(len(m.filtered))
	page := m.filtered[start:end]
	if m.cursor >= len(page) {
		return m, nil
	}
	t := page[m.cursor]
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
	var sb strings.Builder
	sb.WriteString(styles.Faint.Render(fmt.Sprintf("%d transactions", len(m.filtered))) + "\n")

	if len(m.filtered) == 0 {
		sb.WriteString(styles.Faint.Render("No transactions in this period."))
		bg := sb.String()
		switch m.mode {
		case ledgerModeEdit:
			return components.RenderModal(m.editForm.View(), m.width, m.height)
		case ledgerModeDelete:
			return components.RenderModal(m.confirm.View(), m.width, m.height)
		}
		return bg
	}

	// Header row — plain text, no ANSI colour so fmt widths are accurate
	header := fmt.Sprintf("  %-*s %-*s %-*s %-*s %-*s %s",
		colDate, "Date",
		colType, "Type",
		colDesc, "Description",
		colCat, "Category",
		colAmount, "Amount",
		"Account",
	)
	sb.WriteString(styles.Faint.Render(header) + "\n")
	if m.width > 2 {
		sb.WriteString(styles.Faint.Render(strings.Repeat("─", m.width-2)) + "\n")
	}

	start, end := m.pag.GetSliceBounds(len(m.filtered))
	page := m.filtered[start:end]

	for i, t := range page {
		accName := ""
		if t.AccountID != nil {
			for _, a := range m.accounts {
				if *a.ID == *t.AccountID {
					accName = a.Name
				}
			}
		}

		// Pick colour and sign based on transaction type
		txnStyle := lipgloss.NewStyle().Foreground(styles.ColorExpense)
		sign := "-"
		if t.Type == "revenue" {
			txnStyle = lipgloss.NewStyle().Foreground(styles.ColorRevenue)
			sign = "+"
		}

		prefix := "  "
		if i == m.cursor {
			prefix = lipgloss.NewStyle().Foreground(styles.ColorAccent).Render("▶") + " "
		}

		// Each cell: plain cells use fmt padding (safe for ASCII); coloured cells
		// use lipgloss.Width() which is ANSI-aware and produces exact visual width.
		line := prefix +
			fmt.Sprintf("%-*s ", colDate, format.DateDisplay(t.Date)) +
			txnStyle.Width(colType).Render(t.Type) + " " +
			fmt.Sprintf("%-*s ", colDesc, truncate(t.Desc, colDesc)) +
			fmt.Sprintf("%-*s ", colCat, truncate(t.Cat, colCat)) +
			txnStyle.Width(colAmount).Render(sign+format.Currency(t.Val)) + " " +
			truncate(accName, colAccount)

		sb.WriteString(line + "\n")
	}

	sb.WriteString("\n" + m.pag.View())
	bg := sb.String()

	switch m.mode {
	case ledgerModeEdit:
		return components.RenderModal(m.editForm.View(), m.width, m.height)
	case ledgerModeDelete:
		return components.RenderModal(m.confirm.View(), m.width, m.height)
	}
	return bg
}
