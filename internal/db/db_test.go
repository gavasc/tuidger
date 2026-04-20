package db

import (
	"encoding/json"
	"strings"
	"testing"
)

// openMem returns a fresh in-memory DB for each test.
func openMem(t *testing.T) *DB {
	t.Helper()
	d, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func int64p(v int64) *int64 { return &v }

// ── IsEmpty ───────────────────────────────────────────────────────────────────

func TestIsEmpty_fresh(t *testing.T) {
	d := openMem(t)
	if !d.IsEmpty() {
		t.Fatal("expected empty DB to report IsEmpty=true")
	}
}

func TestIsEmpty_afterAccount(t *testing.T) {
	d := openMem(t)
	d.InsertAccount(Account{Name: "Checking", InitialBalance: 0})
	if d.IsEmpty() {
		t.Fatal("expected non-empty DB after inserting an account")
	}
}

// ── Accounts ──────────────────────────────────────────────────────────────────

func TestInsertAccount_and_GetAccounts(t *testing.T) {
	d := openMem(t)
	if err := d.InsertAccount(Account{Name: "Savings", InitialBalance: 1000}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	accounts, err := d.GetAccounts()
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(accounts))
	}
	if accounts[0].Name != "Savings" {
		t.Errorf("name: got %q, want %q", accounts[0].Name, "Savings")
	}
	if accounts[0].InitialBalance != 1000 {
		t.Errorf("initial_balance: got %f, want 1000", accounts[0].InitialBalance)
	}
}

func TestInsertAccount_duplicateName(t *testing.T) {
	d := openMem(t)
	d.InsertAccount(Account{Name: "Cash"})
	if err := d.InsertAccount(Account{Name: "Cash"}); err == nil {
		t.Fatal("expected error inserting duplicate account name")
	}
}

func TestDeleteAccount_noReferences(t *testing.T) {
	d := openMem(t)
	d.InsertAccount(Account{Name: "Temp"})
	accounts, _ := d.GetAccounts()
	id := *accounts[0].ID
	if err := d.DeleteAccount(id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	accounts, _ = d.GetAccounts()
	if len(accounts) != 0 {
		t.Fatal("expected account to be deleted")
	}
}

func TestDeleteAccount_blockedByTransaction(t *testing.T) {
	d := openMem(t)
	d.InsertAccount(Account{Name: "A"})
	accounts, _ := d.GetAccounts()
	id := *accounts[0].ID
	d.InsertTransaction(Transaction{
		Type:      "expense",
		Desc:      "lunch",
		Cat:       "food",
		Val:       10,
		Date:      "2026-01-01",
		AccountID: &id,
	})
	if err := d.DeleteAccount(id); err == nil {
		t.Fatal("expected error deleting account with linked transactions")
	}
}

func TestDeleteAccount_blockedByTransfer(t *testing.T) {
	d := openMem(t)
	d.InsertAccount(Account{Name: "A"})
	d.InsertAccount(Account{Name: "B"})
	accounts, _ := d.GetAccounts()
	idA, idB := *accounts[0].ID, *accounts[1].ID
	d.InsertTransfer(Transfer{FromAccountID: idA, ToAccountID: idB, Amount: 50, Date: "2026-01-01"})
	if err := d.DeleteAccount(idA); err == nil {
		t.Fatal("expected error deleting account with linked transfer")
	}
}

// ── Account Balance Calculation ───────────────────────────────────────────────

func TestGetAccountBalances_initialOnly(t *testing.T) {
	d := openMem(t)
	d.InsertAccount(Account{Name: "A", InitialBalance: 500})
	bals, err := d.GetAccountBalances()
	if err != nil {
		t.Fatal(err)
	}
	if len(bals) != 1 {
		t.Fatalf("expected 1, got %d", len(bals))
	}
	if bals[0].Balance != 500 {
		t.Errorf("balance: got %f, want 500", bals[0].Balance)
	}
}

func TestGetAccountBalances_withRevenueAndExpense(t *testing.T) {
	d := openMem(t)
	d.InsertAccount(Account{Name: "A", InitialBalance: 100})
	accounts, _ := d.GetAccounts()
	id := *accounts[0].ID

	d.InsertTransaction(Transaction{Type: "revenue", Desc: "salary", Cat: "income", Val: 300, Date: "2026-01-01", AccountID: &id})
	d.InsertTransaction(Transaction{Type: "expense", Desc: "rent", Cat: "housing", Val: 150, Date: "2026-01-02", AccountID: &id})

	bals, _ := d.GetAccountBalances()
	// 100 + 300 - 150 = 250
	want := 250.0
	if bals[0].Balance != want {
		t.Errorf("balance: got %f, want %f", bals[0].Balance, want)
	}
}

func TestGetAccountBalances_withTransfers(t *testing.T) {
	d := openMem(t)
	d.InsertAccount(Account{Name: "A", InitialBalance: 1000})
	d.InsertAccount(Account{Name: "B", InitialBalance: 0})
	accounts, _ := d.GetAccounts()
	// accounts ordered by name
	idA, idB := *accounts[0].ID, *accounts[1].ID

	d.InsertTransfer(Transfer{FromAccountID: idA, ToAccountID: idB, Amount: 400, Date: "2026-01-01"})

	bals, _ := d.GetAccountBalances()
	balA, balB := bals[0].Balance, bals[1].Balance
	if balA != 600 {
		t.Errorf("A balance: got %f, want 600", balA)
	}
	if balB != 400 {
		t.Errorf("B balance: got %f, want 400", balB)
	}
}

// ── Transactions ──────────────────────────────────────────────────────────────

func TestQueryTransactions_filtersByDateRange(t *testing.T) {
	d := openMem(t)
	d.InsertTransaction(Transaction{Type: "expense", Desc: "old", Cat: "misc", Val: 1, Date: "2025-01-01"})
	d.InsertTransaction(Transaction{Type: "expense", Desc: "recent", Cat: "misc", Val: 2, Date: "2026-03-15"})
	d.InsertTransaction(Transaction{Type: "expense", Desc: "future", Cat: "misc", Val: 3, Date: "2027-01-01"})

	txns, err := d.QueryTransactions("2026-01-01", "2026-12-31")
	if err != nil {
		t.Fatal(err)
	}
	if len(txns) != 1 {
		t.Fatalf("expected 1 transaction in range, got %d", len(txns))
	}
	if txns[0].Desc != "recent" {
		t.Errorf("got desc %q, want %q", txns[0].Desc, "recent")
	}
}

func TestQueryTransactions_orderedDescByDate(t *testing.T) {
	d := openMem(t)
	for _, date := range []string{"2026-01-01", "2026-03-01", "2026-02-01"} {
		d.InsertTransaction(Transaction{Type: "expense", Desc: date, Cat: "misc", Val: 1, Date: date})
	}
	txns, _ := d.QueryTransactions("2026-01-01", "2026-12-31")
	if txns[0].Date < txns[1].Date {
		t.Errorf("expected descending date order, got %s then %s", txns[0].Date, txns[1].Date)
	}
}

func TestUpdateTransaction(t *testing.T) {
	d := openMem(t)
	d.InsertTransaction(Transaction{Type: "expense", Desc: "old desc", Cat: "food", Val: 50, Date: "2026-01-01"})
	txns, _ := d.QueryTransactions("2026-01-01", "2026-01-01")
	id := *txns[0].ID

	updated := Transaction{ID: &id, Type: "revenue", Desc: "new desc", Cat: "income", Val: 99, Date: "2026-01-02"}
	if err := d.UpdateTransaction(updated); err != nil {
		t.Fatalf("update: %v", err)
	}
	txns, _ = d.QueryTransactions("2026-01-01", "2026-01-02")
	if txns[0].Desc != "new desc" || txns[0].Val != 99 {
		t.Errorf("update not persisted: %+v", txns[0])
	}
}

func TestDeleteTransaction(t *testing.T) {
	d := openMem(t)
	d.InsertTransaction(Transaction{Type: "expense", Desc: "to delete", Cat: "misc", Val: 10, Date: "2026-01-01"})
	txns, _ := d.QueryTransactions("2026-01-01", "2026-01-01")
	id := *txns[0].ID

	d.DeleteTransaction(id)
	txns, _ = d.QueryTransactions("2026-01-01", "2026-01-01")
	if len(txns) != 0 {
		t.Fatal("expected transaction to be deleted")
	}
}

// ── Transfers ─────────────────────────────────────────────────────────────────

func TestInsertAndGetTransfers(t *testing.T) {
	d := openMem(t)
	d.InsertAccount(Account{Name: "Wallet"})
	d.InsertAccount(Account{Name: "Savings"})
	accounts, _ := d.GetAccounts()
	idW, idS := *accounts[1].ID, *accounts[0].ID // alphabetical: Savings < Wallet

	d.InsertTransfer(Transfer{FromAccountID: idW, ToAccountID: idS, Amount: 200, Date: "2026-02-01", Desc: "monthly save"})
	transfers, err := d.GetTransfers()
	if err != nil {
		t.Fatal(err)
	}
	if len(transfers) != 1 {
		t.Fatalf("expected 1 transfer, got %d", len(transfers))
	}
	if transfers[0].Amount != 200 {
		t.Errorf("amount: got %f, want 200", transfers[0].Amount)
	}
}

func TestDeleteTransfer(t *testing.T) {
	d := openMem(t)
	d.InsertAccount(Account{Name: "A"})
	d.InsertAccount(Account{Name: "B"})
	accounts, _ := d.GetAccounts()
	d.InsertTransfer(Transfer{FromAccountID: *accounts[0].ID, ToAccountID: *accounts[1].ID, Amount: 10, Date: "2026-01-01"})
	transfers, _ := d.GetTransfers()
	d.DeleteTransfer(*transfers[0].ID)
	transfers, _ = d.GetTransfers()
	if len(transfers) != 0 {
		t.Fatal("expected transfer to be deleted")
	}
}

// ── Notes ─────────────────────────────────────────────────────────────────────

func TestGetNote_missingReturnsEmpty(t *testing.T) {
	d := openMem(t)
	content, err := d.GetNote("expenses", "2026-01-01", "2026-01-31")
	if err != nil {
		t.Fatal(err)
	}
	if content != "" {
		t.Errorf("expected empty string for missing note, got %q", content)
	}
}

func TestUpsertNote_insertAndUpdate(t *testing.T) {
	d := openMem(t)
	d.UpsertNote("expenses", "2026-01-01", "2026-01-31", "first content")
	content, _ := d.GetNote("expenses", "2026-01-01", "2026-01-31")
	if content != "first content" {
		t.Errorf("got %q, want %q", content, "first content")
	}

	// update same key
	d.UpsertNote("expenses", "2026-01-01", "2026-01-31", "updated content")
	content, _ = d.GetNote("expenses", "2026-01-01", "2026-01-31")
	if content != "updated content" {
		t.Errorf("got %q after update, want %q", content, "updated content")
	}
}

func TestUpsertNote_separateByPeriod(t *testing.T) {
	d := openMem(t)
	d.UpsertNote("expenses", "2026-01-01", "2026-01-31", "jan")
	d.UpsertNote("expenses", "2026-02-01", "2026-02-28", "feb")

	jan, _ := d.GetNote("expenses", "2026-01-01", "2026-01-31")
	feb, _ := d.GetNote("expenses", "2026-02-01", "2026-02-28")
	if jan != "jan" || feb != "feb" {
		t.Errorf("notes mixed up: jan=%q feb=%q", jan, feb)
	}
}

// ── Installments ──────────────────────────────────────────────────────────────

func TestInsertInstallment_createsNTransactions(t *testing.T) {
	d := openMem(t)
	inst := Installment{
		Desc:          "Laptop",
		Cat:           "tech",
		TotalVal:      1200,
		NInstallments: 6,
		StartDate:     "2026-01-01",
	}
	if err := d.InsertInstallment(inst); err != nil {
		t.Fatalf("insert installment: %v", err)
	}
	txns, _ := d.QueryTransactions("2026-01-01", "2026-12-31")
	if len(txns) != 6 {
		t.Fatalf("expected 6 transactions, got %d", len(txns))
	}
	// each installment is 1200/6 = 200
	for _, tx := range txns {
		if tx.Val != 200 {
			t.Errorf("expected monthly val 200, got %f", tx.Val)
		}
		if tx.Type != "expense" {
			t.Errorf("expected expense type, got %q", tx.Type)
		}
	}
}

func TestInsertInstallment_monthlyDateProgression(t *testing.T) {
	d := openMem(t)
	inst := Installment{Desc: "Sub", Cat: "service", TotalVal: 30, NInstallments: 3, StartDate: "2026-03-01"}
	d.InsertInstallment(inst)
	txns, _ := d.QueryTransactions("2026-01-01", "2026-12-31")

	dates := make([]string, len(txns))
	for i, tx := range txns {
		dates[i] = tx.Date
	}
	// QueryTransactions returns DESC; reverse to check progression
	want := []string{"2026-05-01", "2026-04-01", "2026-03-01"}
	for i, d := range dates {
		if d != want[i] {
			t.Errorf("date[%d]: got %q, want %q", i, d, want[i])
		}
	}
}

func TestGetInstallments_paidCount(t *testing.T) {
	d := openMem(t)
	inst := Installment{Desc: "Phone", Cat: "tech", TotalVal: 600, NInstallments: 6, StartDate: "2026-01-01"}
	d.InsertInstallment(inst)

	insts, err := d.GetInstallments()
	if err != nil {
		t.Fatal(err)
	}
	if len(insts) != 1 {
		t.Fatalf("expected 1 installment, got %d", len(insts))
	}
	if *insts[0].PaidCount != 6 {
		t.Errorf("paid_count: got %d, want 6", *insts[0].PaidCount)
	}
	if *insts[0].MonthlyVal != 100 {
		t.Errorf("monthly_val: got %f, want 100", *insts[0].MonthlyVal)
	}
}

func TestDeleteInstallment_cascadesTransactions(t *testing.T) {
	d := openMem(t)
	inst := Installment{Desc: "Car", Cat: "transport", TotalVal: 500, NInstallments: 5, StartDate: "2026-01-01"}
	d.InsertInstallment(inst)

	insts, _ := d.GetInstallments()
	instID := *insts[0].ID

	if err := d.DeleteInstallment(instID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	txns, _ := d.QueryTransactions("2026-01-01", "2027-12-31")
	if len(txns) != 0 {
		t.Fatalf("expected 0 transactions after cascade delete, got %d", len(txns))
	}
	insts, _ = d.GetInstallments()
	if len(insts) != 0 {
		t.Fatal("expected installment to be deleted")
	}
}

func TestInsertInstallment_invalidStartDate(t *testing.T) {
	d := openMem(t)
	inst := Installment{Desc: "Bad", Cat: "x", TotalVal: 100, NInstallments: 3, StartDate: "not-a-date"}
	if err := d.InsertInstallment(inst); err == nil {
		t.Fatal("expected error for invalid start_date")
	}
}

// ── Export / Import ───────────────────────────────────────────────────────────

func TestExportJSON_roundtrip(t *testing.T) {
	d := openMem(t)
	d.InsertAccount(Account{Name: "Main", InitialBalance: 500})
	accounts, _ := d.GetAccounts()
	id := *accounts[0].ID
	d.InsertTransaction(Transaction{Type: "expense", Desc: "lunch", Cat: "food", Val: 12.5, Date: "2026-01-15", AccountID: &id})

	jsonStr, err := d.ExportJSON()
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if !strings.Contains(jsonStr, "Main") || !strings.Contains(jsonStr, "lunch") {
		t.Error("JSON output missing expected data")
	}

	// Import into a fresh DB
	d2 := openMem(t)
	if err := d2.ImportJSON(jsonStr); err != nil {
		t.Fatalf("import: %v", err)
	}
	accs, _ := d2.GetAccounts()
	if len(accs) != 1 || accs[0].Name != "Main" {
		t.Errorf("after import: accounts = %+v", accs)
	}
	txns, _ := d2.QueryTransactions("2026-01-01", "2026-12-31")
	if len(txns) != 1 || txns[0].Desc != "lunch" {
		t.Errorf("after import: transactions = %+v", txns)
	}
}

func TestExportCSV_header(t *testing.T) {
	d := openMem(t)
	csv, err := d.ExportCSV()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(csv, "id,type,desc,cat,val,date") {
		t.Errorf("CSV missing header: %q", csv[:50])
	}
}

func TestImportJSON_replacesExistingData(t *testing.T) {
	d := openMem(t)
	d.InsertAccount(Account{Name: "Old"})

	// build a JSON payload with different data
	newData := `{"accounts":[{"ID":1,"Name":"New","InitialBalance":0}],"transactions":null,"transfers":null,"installments":null}`
	if err := d.ImportJSON(newData); err != nil {
		t.Fatalf("import: %v", err)
	}
	accs, _ := d.GetAccounts()
	if len(accs) != 1 || accs[0].Name != "New" {
		t.Errorf("expected account 'New' after import, got %+v", accs)
	}
}

func TestExportJSON_isValidJSON(t *testing.T) {
	d := openMem(t)
	d.InsertAccount(Account{Name: "A"})
	jsonStr, _ := d.ExportJSON()
	var v interface{}
	if err := json.Unmarshal([]byte(jsonStr), &v); err != nil {
		t.Errorf("exported JSON is not valid: %v", err)
	}
}
