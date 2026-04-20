package views

import (
	"fmt"
	"sort"
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
	balances []db.AccountBalance
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
	m.balances = msg.Balances
	m.updateAccounts(msg.Accounts)
}

func (m DashboardModel) Capturing() bool { return m.mode != addModeNone }

func (m DashboardModel) Hints() string {
	if m.mode != addModeNone {
		return "Tab next field  Enter submit  Esc cancel"
	}
	return "[e] add expense  [r] add revenue  [↑/↓] scroll"
}

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
			// Installment fields start hidden; updateForm reveals them when toggle is on.
			m.expForm.Fields[6].Hidden = true
			m.expForm.Fields[7].Hidden = true
			m.expForm.FocusFirst()
			return m, nil
		case "r":
			m.mode = addModeRevenue
			m.revForm.Reset()
			m.updateAccounts(m.accounts)
			if len(m.accounts) > 0 {
				m.revForm.Fields[4].Options = accountNames(m.accounts)
			}
			// Installments don't apply to revenue — hide all three installment fields.
			m.revForm.Fields[5].Hidden = true
			m.revForm.Fields[6].Hidden = true
			m.revForm.Fields[7].Hidden = true
			m.revForm.FocusFirst()
			return m, nil
		case "j", "down":
			m.vp.LineDown(1)
		case "k", "up":
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
	m.vp.SetContent(m.buildContent())
	bg := m.vp.View()
	if m.mode != addModeNone {
		var formContent string
		if m.mode == addModeExpense {
			formContent = m.expForm.View()
		} else {
			formContent = m.revForm.View()
		}
		return components.RenderModal(formContent, m.width, m.height)
	}
	return bg
}

func (m DashboardModel) buildContent() string {
	var sb strings.Builder

	// ── Summary row + hint on the same line ──────────────────────────────────
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

	// ── Recent 5 transactions + category charts side by side ─────────────────
	halfW := m.width/2 - 2

	// left: recent transactions
	var recentSB strings.Builder
	recentSB.WriteString(styles.Title.Render("Recent") + "\n")
	if len(m.txns) == 0 {
		recentSB.WriteString(styles.Faint.Render("No transactions in this period.") + "\n")
	} else {
		n := 5
		if len(m.txns) < n {
			n = len(m.txns)
		}
		for _, t := range m.txns[:n] {
			var valStr string
			if t.Type == "expense" {
				valStr = styles.ExpenseText.Render(fmt.Sprintf("%-16s", format.Currency(t.Val)))
			} else {
				valStr = styles.RevenueText.Render(fmt.Sprintf("%-16s", format.Currency(t.Val)))
			}
			recentSB.WriteString(fmt.Sprintf("%s %s %s\n",
				valStr,
				truncate(t.Desc+" · "+t.Cat, 22),
				styles.Faint.Render(format.DateDisplay(t.Date))))
		}
	}

	// right: bar charts stacked
	expCats := catBreakdown(m.txns, "expense")
	revCats := catBreakdown(m.txns, "revenue")
	expChart := components.RenderBarChart(expCats, halfW, "Expenses by Category")
	revChart := components.RenderBarChart(revCats, halfW, "Revenues by Category")
	rightCol := expChart + revChart

	// merge left and right column line by line
	leftLines := strings.Split(recentSB.String(), "\n")
	rightLines := strings.Split(rightCol, "\n")
	maxLines := len(leftLines)
	if len(rightLines) > maxLines {
		maxLines = len(rightLines)
	}
	for i := 0; i < maxLines; i++ {
		ll, rl := "", ""
		if i < len(leftLines) {
			ll = leftLines[i]
		}
		if i < len(rightLines) {
			rl = rightLines[i]
		}
		sb.WriteString(lipgloss.NewStyle().Width(halfW).Render(ll) + "  " + rl + "\n")
	}

	// ── Line chart ────────────────────────────────────────────────────────────
	expenses, revenues := dailyBuckets(m.txns)
	chart := components.RenderLineChart(expenses, revenues, m.width-4, 4)
	sb.WriteString(chart + "\n\n")

	// ── Ledger preview ────────────────────────────────────────────────────────
	sb.WriteString(styles.Title.Render("Ledger") + "\n")
	if len(m.txns) == 0 {
		sb.WriteString(styles.Faint.Render("No transactions in this period.") + "\n")
	} else {
		header := fmt.Sprintf("  %-12s %-10s %-28s %-16s %s",
			"Date", "Type", "Description", "Category", "Amount")
		sb.WriteString(styles.Faint.Render(header) + "\n")
		if m.width > 2 {
			sb.WriteString(styles.Faint.Render(strings.Repeat("─", m.width-2)) + "\n")
		}
		previewTxns := m.txns
		if len(previewTxns) > 10 {
			previewTxns = previewTxns[:10]
		}
		for _, t := range previewTxns {
			var valStr string
			if t.Type == "expense" {
				valStr = styles.ExpenseText.Render(fmt.Sprintf("%-14s", "-"+format.Currency(t.Val)))
			} else {
				valStr = styles.RevenueText.Render(fmt.Sprintf("%-14s", "+"+format.Currency(t.Val)))
			}
			typeLabel := styles.Faint.Render(fmt.Sprintf("%-10s", t.Type))
			sb.WriteString(fmt.Sprintf("  %s %s%-28s %-16s %s\n",
				styles.Faint.Render(fmt.Sprintf("%-12s", format.DateDisplay(t.Date))),
				typeLabel,
				truncate(t.Desc, 28),
				truncate(t.Cat, 16),
				valStr,
			))
		}
	}
	sb.WriteString("\n")

	// ── Accounts ─────────────────────────────────────────────────────────────
	sb.WriteString(styles.Title.Render("Accounts") + "\n")
	if len(m.balances) == 0 {
		sb.WriteString(styles.Faint.Render("No accounts yet.") + "\n")
	} else {
		var total float64
		for _, ab := range m.balances {
			balStr := format.Currency(ab.Balance)
			if ab.Balance >= 0 {
				balStr = styles.RevenueText.Render(balStr)
			} else {
				balStr = styles.ExpenseText.Render(balStr)
			}
			sb.WriteString(fmt.Sprintf("  %-30s %s\n", ab.Name, balStr))
			total += ab.Balance
		}
		sb.WriteString(styles.Faint.Render(strings.Repeat("─", 44)) + "\n")
		totalStr := format.Currency(total)
		if total >= 0 {
			totalStr = styles.RevenueText.Render("Total  " + totalStr)
		} else {
			totalStr = styles.ExpenseText.Render("Total  " + totalStr)
		}
		sb.WriteString("  " + totalStr + "\n")
	}

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
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Value > entries[j].Value
	})
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
