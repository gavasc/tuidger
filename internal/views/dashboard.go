package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gavasc/tuidger/internal/components"
	"github.com/gavasc/tuidger/internal/db"
	"github.com/gavasc/tuidger/internal/format"
	"github.com/gavasc/tuidger/internal/styles"
)

type addMode int

const (
	addModeNone addMode = iota
	addModeExpense
	addModeRevenue
)

type DashboardModel struct {
	d        *db.DB
	txns     []db.Transaction
	accounts []db.Account
	mode     addMode
	expForm  components.FormModel
	revForm  components.FormModel
	vp       viewport.Model
	width    int
	height   int
}

func NewDashboardModel(d *db.DB) DashboardModel {
	m := DashboardModel{d: d}
	m.expForm = buildTxnForm("Add Expense", "expense")
	m.revForm = buildTxnForm("Add Revenue", "revenue")
	return m
}

func buildTxnForm(title, _ string) components.FormModel {
	f := components.NewForm(title, "Enter → Submit")
	f.AddNumberField("Amount", true)
	f.AddTextField("Description", true)
	f.AddTextField("Category", true)
	f.AddDateField("Date", true)
	f.AddSelectField("Account", []string{}, false)
	f.AddToggleField("Installments?")
	f.AddNumberField("N installments", false)
	f.AddDateField("Start date", false)
	return f
}

func (m *DashboardModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.vp = viewport.New(w, h)
	m.vp.SetContent(m.buildContent())
}

func (m *DashboardModel) updateAccounts(accounts []db.Account) {
	opts := make([]string, len(accounts))
	for i, a := range accounts {
		opts[i] = a.Name
	}
	if len(m.expForm.Fields) > 4 {
		m.expForm.Fields[4].Options = opts
	}
	if len(m.revForm.Fields) > 4 {
		m.revForm.Fields[4].Options = opts
	}
}

func (m DashboardModel) OnTransactionsLoaded(msg TransactionsLoadedMsg) (DashboardModel, tea.Cmd) {
	m.txns = msg.Txns
	m.vp.SetContent(m.buildContent())
	return m, nil
}

func (m *DashboardModel) OnAccountsLoaded(msg AccountsLoadedMsg) {
	m.accounts = msg.Accounts
	m.updateAccounts(msg.Accounts)
}

func (m DashboardModel) Capturing() bool { return m.mode != addModeNone }

func (m DashboardModel) Update(msg tea.Msg) (DashboardModel, tea.Cmd) {
	if m.mode != addModeNone {
		return m.updateForm(msg)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "e":
			m.mode = addModeExpense
			m.expForm.Reset()
			m.updateAccounts(m.accounts)
			if len(m.accounts) > 0 {
				m.expForm.Fields[4].Options = accountNames(m.accounts)
			}
			m.expForm.FocusFirst()
			return m, nil
		case "r":
			m.mode = addModeRevenue
			m.revForm.Reset()
			m.updateAccounts(m.accounts)
			if len(m.accounts) > 0 {
				m.revForm.Fields[4].Options = accountNames(m.accounts)
			}
			m.revForm.FocusFirst()
			return m, nil
		case "j":
			m.vp.LineDown(1)
		case "k":
			m.vp.LineUp(1)
		}
	}
	return m, nil
}

func (m DashboardModel) updateForm(msg tea.Msg) (DashboardModel, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			m.mode = addModeNone
			return m, nil
		case "enter":
			return m.submitForm()
		}
	}

	var cmd tea.Cmd
	if m.mode == addModeExpense {
		m.expForm, cmd = m.expForm.Update(msg)
		// show/hide installment fields
		toggleIdx := 5
		nIdx := 6
		startIdx := 7
		if toggleIdx < len(m.expForm.Fields) {
			on := m.expForm.Fields[toggleIdx].Value
			m.expForm.Fields[nIdx].Hidden = !on
			m.expForm.Fields[startIdx].Hidden = !on
		}
	} else {
		m.revForm, cmd = m.revForm.Update(msg)
	}
	return m, cmd
}

func (m DashboardModel) submitForm() (DashboardModel, tea.Cmd) {
	var f *components.FormModel
	txnType := "expense"
	if m.mode == addModeRevenue {
		f = &m.revForm
		txnType = "revenue"
	} else {
		f = &m.expForm
	}

	if !f.Validate() {
		return m, nil
	}

	vals := f.Values()
	val, _ := format.ParseFloat(vals["Amount"])
	date := vals["Date"]
	if date == "" {
		date = format.TodayISO()
	}

	// find account id
	var accountID *int64
	for _, a := range m.accounts {
		if a.Name == vals["Account"] {
			id := *a.ID
			accountID = &id
			break
		}
	}

	isInstallment := vals["Installments?"] == "true"
	if isInstallment && txnType == "expense" {
		n, _ := format.ParseFloat(vals["N installments"])
		startDate := vals["Start date"]
		if startDate == "" {
			startDate = date
		}
		inst := db.Installment{
			Desc:          vals["Description"],
			Cat:           vals["Category"],
			TotalVal:      val * float64(int(n)),
			NInstallments: int64(n),
			StartDate:     startDate,
			AccountID:     accountID,
		}
		m.mode = addModeNone
		return m, insertInstallment(m.d, inst)
	}

	t := db.Transaction{
		Type:      txnType,
		Desc:      vals["Description"],
		Cat:       vals["Category"],
		Val:       val,
		Date:      date,
		AccountID: accountID,
	}
	m.mode = addModeNone
	return m, insertTransaction(m.d, t, "", "")
}

func (m DashboardModel) View() string {
	if m.mode != addModeNone {
		if m.mode == addModeExpense {
			return m.expForm.View()
		}
		return m.revForm.View()
	}
	m.vp.SetContent(m.buildContent())
	return m.vp.View()
}

func (m DashboardModel) buildContent() string {
	var sb strings.Builder

	// Totals
	var totalExp, totalRev float64
	for _, t := range m.txns {
		if t.Type == "expense" {
			totalExp += t.Val
		} else {
			totalRev += t.Val
		}
	}
	balance := totalRev - totalExp

	expStr := styles.ExpenseText.Render("Expenses: " + format.Currency(totalExp))
	revStr := styles.RevenueText.Render("Revenues: " + format.Currency(totalRev))
	var balStr string
	if balance >= 0 {
		balStr = styles.RevenueText.Render("Balance: " + format.Currency(balance))
	} else {
		balStr = styles.ExpenseText.Render("Balance: " + format.Currency(balance))
	}
	sb.WriteString(expStr + "   " + revStr + "   " + balStr + "\n\n")

	// Quick-add hint
	sb.WriteString(styles.Faint.Render("[e] Add Expense  [r] Add Revenue") + "\n\n")

	// Recent 4 transactions
	if len(m.txns) > 0 {
		sb.WriteString(styles.Title.Render("Recent") + "\n")
		n := 4
		if len(m.txns) < n {
			n = len(m.txns)
		}
		for _, t := range m.txns[:n] {
			var valStr string
			if t.Type == "expense" {
				valStr = styles.ExpenseText.Render(format.Currency(t.Val))
			} else {
				valStr = styles.RevenueText.Render(format.Currency(t.Val))
			}
			sb.WriteString(fmt.Sprintf("  %s  %-28s  %s\n",
				valStr,
				truncate(t.Desc+" · "+t.Cat, 28),
				styles.Faint.Render(format.DateDisplay(t.Date))))
		}
		sb.WriteString("\n")
	} else {
		sb.WriteString(styles.Faint.Render("No transactions in this period. Press [e] or [r] to add one.") + "\n\n")
	}

	// Category breakdown
	expCats := catBreakdown(m.txns, "expense")
	revCats := catBreakdown(m.txns, "revenue")

	halfW := m.width/2 - 2
	expChart := components.RenderBarChart(expCats, halfW, "Expenses by Category")
	revChart := components.RenderBarChart(revCats, halfW, "Revenues by Category")

	// side by side
	expLines := strings.Split(expChart, "\n")
	revLines := strings.Split(revChart, "\n")
	maxLines := len(expLines)
	if len(revLines) > maxLines {
		maxLines = len(revLines)
	}
	for i := 0; i < maxLines; i++ {
		el, rl := "", ""
		if i < len(expLines) {
			el = expLines[i]
		}
		if i < len(revLines) {
			rl = revLines[i]
		}
		sb.WriteString(lipgloss.NewStyle().Width(halfW).Render(el) + "  " + rl + "\n")
	}
	sb.WriteString("\n")

	// Line chart
	expenses, revenues := dailyBuckets(m.txns)
	chart := components.RenderLineChart(expenses, revenues, m.width-4, 8)
	sb.WriteString(chart + "\n")

	return sb.String()
}

func catBreakdown(txns []db.Transaction, txType string) []components.BarEntry {
	totals := map[string]float64{}
	var overall float64
	for _, t := range txns {
		if t.Type == txType {
			totals[t.Cat] += t.Val
			overall += t.Val
		}
	}
	if overall == 0 {
		return nil
	}
	entries := make([]components.BarEntry, 0, len(totals))
	color := styles.ColorExpense
	if txType == "revenue" {
		color = styles.ColorRevenue
	}
	for cat, val := range totals {
		entries = append(entries, components.BarEntry{
			Label: cat,
			Value: val,
			Pct:   val / overall * 100,
			Color: color,
		})
	}
	return entries
}

func dailyBuckets(txns []db.Transaction) (expenses, revenues []float64) {
	days := map[string][2]float64{}
	var dates []string
	seen := map[string]bool{}
	for _, t := range txns {
		if !seen[t.Date] {
			dates = append(dates, t.Date)
			seen[t.Date] = true
		}
		v := days[t.Date]
		if t.Type == "expense" {
			v[0] += t.Val
		} else {
			v[1] += t.Val
		}
		days[t.Date] = v
	}
	// sort dates ascending
	sortStrings(dates)
	for _, d := range dates {
		expenses = append(expenses, days[d][0])
		revenues = append(revenues, days[d][1])
	}
	return
}

func sortStrings(s []string) {
	// simple insertion sort (avoids importing sort for small slices)
	for i := 1; i < len(s); i++ {
		key := s[i]
		j := i - 1
		for j >= 0 && s[j] > key {
			s[j+1] = s[j]
			j--
		}
		s[j+1] = key
	}
}

func accountNames(accounts []db.Account) []string {
	names := make([]string, len(accounts))
	for i, a := range accounts {
		names[i] = a.Name
	}
	return names
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
