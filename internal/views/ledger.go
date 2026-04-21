package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/paginator"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gavasc/tuidger/internal/components"
	"github.com/gavasc/tuidger/internal/db"
	"github.com/gavasc/tuidger/internal/format"
	"github.com/gavasc/tuidger/internal/styles"
)

const ledgerPageSize = 40

const (
	colDate    = 12
	colType    = 9
	colDesc    = 26
	colCat     = 14
	colAmount  = 14
	colAccount = 14
)

const (
	ledgerFldNone = iota
	ledgerFldDate
	ledgerFldCat
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
	cursor   int
	editID   int64
	editForm components.FormModel
	confirm  components.ConfirmModel
	width    int
	height   int
	// filter state
	fType       string
	fDate       string
	fCat        string
	fAcct       string
	filterField int
	filterInput textinput.Model
	filterActive bool
	acctPicking  bool
	acctCursor   int
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

	fi := textinput.New()
	fi.Prompt = ""
	fi.Width = 20

	return LedgerModel{d: d, pag: p, editForm: f, filterInput: fi}
}

func (m *LedgerModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

func (m LedgerModel) OnTransactionsLoaded(msg TransactionsLoadedMsg) (LedgerModel, tea.Cmd) {
	m.allTxns = msg.Txns
	m.applyFilter()
	return m, nil
}

func (m *LedgerModel) applyFilter() {
	acctName := func(id *int64) string {
		if id == nil {
			return ""
		}
		for _, a := range m.accounts {
			if *a.ID == *id {
				return a.Name
			}
		}
		return ""
	}

	var out []db.Transaction
	for _, t := range m.allTxns {
		if m.fType != "" && t.Type != m.fType {
			continue
		}
		if m.fDate != "" && !strings.Contains(t.Date, m.fDate) {
			continue
		}
		if m.fCat != "" && !strings.Contains(strings.ToLower(t.Cat), strings.ToLower(m.fCat)) {
			continue
		}
		if m.fAcct != "" && !strings.EqualFold(acctName(t.AccountID), m.fAcct) {
			continue
		}
		out = append(out, t)
	}
	if out == nil {
		out = []db.Transaction{}
	}
	m.filtered = out
	m.pag.SetTotalPages(len(m.filtered))
	m.cursor = 0
}

func (m LedgerModel) Capturing() bool {
	return m.mode != ledgerModeList || m.filterActive || m.acctPicking
}

func (m LedgerModel) Hints() string {
	switch m.mode {
	case ledgerModeEdit:
		return "Tab next field  Enter save  Esc cancel"
	case ledgerModeDelete:
		return "y confirm  n cancel"
	}
	if m.filterActive {
		return "Enter apply  Esc cancel"
	}
	if m.acctPicking {
		return "↑/↓ navigate  Enter select (again to clear)  Esc cancel"
	}
	return "[e] edit  [d] delete  [n]/[p] page  [T] type  [D] date  [C] cat  [A] acct  [/] clear filters"
}

func (m LedgerModel) openFilterInput(field int, current string) LedgerModel {
	m.filterField = field
	m.filterInput.SetValue(current)
	m.filterInput.Focus()
	m.filterActive = true
	return m
}

func (m LedgerModel) updateFilterInput(msg tea.Msg) (LedgerModel, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter":
			val := m.filterInput.Value()
			switch m.filterField {
			case ledgerFldDate:
				m.fDate = val
			case ledgerFldCat:
				m.fCat = val
			}
			m.filterInput.Blur()
			m.filterActive = false
			m.applyFilter()
			return m, nil
		case "esc":
			m.filterInput.Blur()
			m.filterActive = false
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.filterInput, cmd = m.filterInput.Update(msg)
	return m, cmd
}

func (m LedgerModel) updateAcctPicker(msg tea.Msg) (LedgerModel, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "j", "down":
			if m.acctCursor < len(m.accounts)-1 {
				m.acctCursor++
			}
		case "k", "up":
			if m.acctCursor > 0 {
				m.acctCursor--
			}
		case "enter":
			if m.acctCursor < len(m.accounts) {
				selected := m.accounts[m.acctCursor].Name
				if strings.EqualFold(m.fAcct, selected) {
					m.fAcct = ""
				} else {
					m.fAcct = selected
				}
			}
			m.acctPicking = false
			m.applyFilter()
		case "esc":
			m.acctPicking = false
		}
	}
	return m, nil
}

func (m LedgerModel) Update(msg tea.Msg) (LedgerModel, tea.Cmd) {
	switch m.mode {
	case ledgerModeEdit:
		return m.updateEdit(msg)
	case ledgerModeDelete:
		return m.updateDelete(msg)
	}

	if m.filterActive {
		return m.updateFilterInput(msg)
	}
	if m.acctPicking {
		return m.updateAcctPicker(msg)
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
		case "T":
			switch m.fType {
			case "":
				m.fType = "expense"
			case "expense":
				m.fType = "revenue"
			default:
				m.fType = ""
			}
			m.applyFilter()
		case "D":
			return m.openFilterInput(ledgerFldDate, m.fDate), nil
		case "C":
			return m.openFilterInput(ledgerFldCat, m.fCat), nil
		case "A":
			m.acctCursor = 0
			if m.fAcct != "" {
				for i, a := range m.accounts {
					if strings.EqualFold(a.Name, m.fAcct) {
						m.acctCursor = i
						break
					}
				}
			}
			m.acctPicking = true
		case "/":
			m.fType = ""
			m.fDate = ""
			m.fCat = ""
			m.fAcct = ""
			m.applyFilter()
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

func (m LedgerModel) filterBarView() string {
	var chips []string
	accent := lipgloss.NewStyle().Foreground(styles.ColorAccent)
	if m.fType != "" {
		chips = append(chips, accent.Render("[type: "+m.fType+"]"))
	}
	if m.fDate != "" {
		chips = append(chips, accent.Render("[date: "+m.fDate+"]"))
	}
	if m.fCat != "" {
		chips = append(chips, accent.Render("[cat: "+m.fCat+"]"))
	}
	if m.fAcct != "" {
		chips = append(chips, accent.Render("[acct: "+m.fAcct+"]"))
	}
	if len(chips) == 0 {
		return ""
	}
	return strings.Join(chips, "  ") + "\n"
}

func (m LedgerModel) View() string {
	var sb strings.Builder
	sb.WriteString(styles.Faint.Render(fmt.Sprintf("%d transactions", len(m.filtered))) + "\n")

	if bar := m.filterBarView(); bar != "" {
		sb.WriteString(bar)
	}

	if m.filterActive {
		var label string
		switch m.filterField {
		case ledgerFldDate:
			label = "Filter date: "
		case ledgerFldCat:
			label = "Filter cat: "
		}
		sb.WriteString(styles.Faint.Render(label) + m.filterInput.View() + "\n")
	}

	if m.acctPicking {
		sb.WriteString(styles.Faint.Render("Pick account  Enter select  Esc cancel") + "\n\n")
		for i, a := range m.accounts {
			prefix := "  "
			if i == m.acctCursor {
				prefix = lipgloss.NewStyle().Foreground(styles.ColorAccent).Render("▶") + " "
			}
			name := a.Name
			if strings.EqualFold(a.Name, m.fAcct) {
				name = lipgloss.NewStyle().Bold(true).Foreground(styles.ColorAccent).Render(a.Name)
			}
			sb.WriteString(prefix + name + "\n")
		}
		return sb.String()
	}

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
