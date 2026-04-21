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

const (
	transferFldNone = iota
	transferFldDate
	transferFldDesc
)

type transfersMode int

const (
	transfersModeList transfersMode = iota
	transfersModeAdd
	transfersModeDelete
)

type TransfersModel struct {
	d            *db.DB
	allTransfers []db.Transfer
	filtered     []db.Transfer
	accounts     []db.Account
	cursor       int
	mode         transfersMode
	addForm      components.FormModel
	confirm      components.ConfirmModel
	pag          paginator.Model
	width        int
	height       int
	// filter state
	fAcct        string
	fDate        string
	fDesc        string
	filterField  int
	filterInput  textinput.Model
	filterActive bool
	acctPicking  bool
	acctCursor   int
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

	fi := textinput.New()
	fi.Prompt = ""
	fi.Width = 20

	return TransfersModel{d: d, addForm: f, pag: p, filterInput: fi}
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
	m.allTransfers = msg.Transfers
	m.applyFilter()
}

func (m *TransfersModel) applyFilter() {
	deref := func(s *string) string {
		if s == nil {
			return ""
		}
		return *s
	}

	var out []db.Transfer
	for _, t := range m.allTransfers {
		if m.fDate != "" && !strings.Contains(t.Date, m.fDate) {
			continue
		}
		if m.fAcct != "" {
			from := strings.ToLower(deref(t.FromAccountName))
			to := strings.ToLower(deref(t.ToAccountName))
			q := strings.ToLower(m.fAcct)
			if !strings.Contains(from, q) && !strings.Contains(to, q) {
				continue
			}
		}
		if m.fDesc != "" && !strings.Contains(strings.ToLower(t.Desc), strings.ToLower(m.fDesc)) {
			continue
		}
		out = append(out, t)
	}
	if out == nil {
		out = []db.Transfer{}
	}
	m.filtered = out
	m.pag.SetTotalPages(len(m.filtered))
	m.cursor = 0
}

func (m TransfersModel) Capturing() bool {
	return m.mode != transfersModeList || m.filterActive || m.acctPicking
}

func (m TransfersModel) Hints() string {
	switch m.mode {
	case transfersModeAdd:
		return "Tab next field  Enter submit  Esc cancel"
	case transfersModeDelete:
		return "y confirm  n cancel"
	}
	if m.filterActive {
		return "Enter apply  Esc cancel"
	}
	if m.acctPicking {
		return "↑/↓ navigate  Enter select (again to clear)  Esc cancel"
	}
	return "[n] new  [d] delete  [↑/↓] navigate  [←/→] page  [A] acct  [D] date  [S] desc  [/] clear"
}

func (m TransfersModel) openFilterInput(field int, current string) TransfersModel {
	m.filterField = field
	m.filterInput.SetValue(current)
	m.filterInput.Focus()
	m.filterActive = true
	return m
}

func (m TransfersModel) updateFilterInput(msg tea.Msg) (TransfersModel, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter":
			val := m.filterInput.Value()
			switch m.filterField {
			case transferFldDate:
				m.fDate = val
			case transferFldDesc:
				m.fDesc = val
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

func (m TransfersModel) updateAcctPicker(msg tea.Msg) (TransfersModel, tea.Cmd) {
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

func (m TransfersModel) Update(msg tea.Msg) (TransfersModel, tea.Cmd) {
	switch m.mode {
	case transfersModeAdd:
		return m.updateAdd(msg)
	case transfersModeDelete:
		return m.updateDelete(msg)
	}

	if m.filterActive {
		return m.updateFilterInput(msg)
	}
	if m.acctPicking {
		return m.updateAcctPicker(msg)
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
			start, end := m.pag.GetSliceBounds(len(m.filtered))
			page := m.filtered[start:end]
			if m.cursor < len(page) {
				tr := page[m.cursor]
				m.confirm = components.NewConfirm(*tr.ID, fmt.Sprintf("Delete transfer on %s?", format.DateDisplay(tr.Date)))
				m.mode = transfersModeDelete
			}
			return m, nil
		case "j", "down":
			start, end := m.pag.GetSliceBounds(len(m.filtered))
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
		case "D":
			return m.openFilterInput(transferFldDate, m.fDate), nil
		case "S":
			return m.openFilterInput(transferFldDesc, m.fDesc), nil
		case "/":
			m.fAcct = ""
			m.fDate = ""
			m.fDesc = ""
			m.applyFilter()
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

func (m TransfersModel) filterBarView() string {
	var chips []string
	accent := lipgloss.NewStyle().Foreground(styles.ColorAccent)
	if m.fDate != "" {
		chips = append(chips, accent.Render("[date: "+m.fDate+"]"))
	}
	if m.fAcct != "" {
		chips = append(chips, accent.Render("[acct: "+m.fAcct+"]"))
	}
	if m.fDesc != "" {
		chips = append(chips, accent.Render("[desc: "+m.fDesc+"]"))
	}
	if len(chips) == 0 {
		return ""
	}
	return strings.Join(chips, "  ") + "\n"
}

func (m TransfersModel) View() string {
	var sb strings.Builder

	if bar := m.filterBarView(); bar != "" {
		sb.WriteString(bar)
	}

	if m.filterActive {
		var label string
		switch m.filterField {
		case transferFldDate:
			label = "Filter date: "
		case transferFldDesc:
			label = "Filter desc: "
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
		bg := sb.String()
		switch m.mode {
		case transfersModeAdd:
			return components.RenderModal(m.addForm.View(), m.width, m.height)
		case transfersModeDelete:
			return components.RenderModal(m.confirm.View(), m.width, m.height)
		}
		return bg
	}

	if len(m.filtered) == 0 {
		sb.WriteString(styles.Faint.Render("No transfers yet. Press [n] to create one.") + "\n")
	} else {
		start, end := m.pag.GetSliceBounds(len(m.filtered))
		page := m.filtered[start:end]
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
