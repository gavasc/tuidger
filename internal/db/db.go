package db

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct {
	conn *sql.DB
}

func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path+"?_journal=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}
	conn.SetMaxOpenConns(1)
	d := &DB{conn: conn}
	if err := d.migrate(); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *DB) Close() error { return d.conn.Close() }

func (d *DB) migrate() error {
	schema := `
CREATE TABLE IF NOT EXISTS accounts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    initial_balance REAL NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS installments (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    desc           TEXT NOT NULL,
    cat            TEXT NOT NULL,
    total_val      REAL NOT NULL,
    n_installments INTEGER NOT NULL,
    start_date     TEXT NOT NULL,
    account_id     INTEGER REFERENCES accounts(id)
);
CREATE TABLE IF NOT EXISTS transactions (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    type              TEXT NOT NULL,
    desc              TEXT NOT NULL,
    cat               TEXT NOT NULL,
    val               REAL NOT NULL,
    date              TEXT NOT NULL,
    account_id        INTEGER REFERENCES accounts(id),
    installment_id    INTEGER REFERENCES installments(id),
    installment_index INTEGER
);
CREATE TABLE IF NOT EXISTS transfers (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    from_account_id INTEGER NOT NULL REFERENCES accounts(id),
    to_account_id   INTEGER NOT NULL REFERENCES accounts(id),
    amount REAL NOT NULL,
    date   TEXT NOT NULL,
    desc   TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS notes (
    section     TEXT NOT NULL,
    period_from TEXT NOT NULL,
    period_to   TEXT NOT NULL,
    content     TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (section, period_from, period_to)
);`
	if _, err := d.conn.Exec(schema); err != nil {
		return err
	}
	// silent migrations
	for _, stmt := range []string{
		`ALTER TABLE transactions ADD COLUMN account_id INTEGER`,
		`ALTER TABLE transactions ADD COLUMN installment_id INTEGER`,
		`ALTER TABLE transactions ADD COLUMN installment_index INTEGER`,
	} {
		d.conn.Exec(stmt) // ignore errors (column already exists)
	}
	return nil
}

// ── Transactions ──────────────────────────────────────────────────────────────

func (d *DB) QueryTransactions(from, to string) ([]Transaction, error) {
	rows, err := d.conn.Query(`
		SELECT id, type, desc, cat, val, date,
		       account_id, installment_id, installment_index
		FROM transactions
		WHERE date BETWEEN ? AND ?
		ORDER BY date DESC`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Transaction
	for rows.Next() {
		var t Transaction
		if err := rows.Scan(&t.ID, &t.Type, &t.Desc, &t.Cat, &t.Val, &t.Date,
			&t.AccountID, &t.InstallmentID, &t.InstallmentIndex); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (d *DB) InsertTransaction(t Transaction) error {
	_, err := d.conn.Exec(`INSERT INTO transactions (type,desc,cat,val,date,account_id,installment_id,installment_index)
		VALUES (?,?,?,?,?,?,?,?)`,
		t.Type, t.Desc, t.Cat, t.Val, t.Date, t.AccountID, t.InstallmentID, t.InstallmentIndex)
	return err
}

func (d *DB) UpdateTransaction(t Transaction) error {
	_, err := d.conn.Exec(`UPDATE transactions SET type=?,desc=?,cat=?,val=?,date=?,account_id=? WHERE id=?`,
		t.Type, t.Desc, t.Cat, t.Val, t.Date, t.AccountID, *t.ID)
	return err
}

func (d *DB) DeleteTransaction(id int64) error {
	_, err := d.conn.Exec(`DELETE FROM transactions WHERE id=?`, id)
	return err
}

// ── Accounts ──────────────────────────────────────────────────────────────────

func (d *DB) GetAccounts() ([]Account, error) {
	rows, err := d.conn.Query(`SELECT id, name, initial_balance FROM accounts ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Account
	for rows.Next() {
		var a Account
		if err := rows.Scan(&a.ID, &a.Name, &a.InitialBalance); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (d *DB) InsertAccount(a Account) error {
	_, err := d.conn.Exec(`INSERT INTO accounts (name, initial_balance) VALUES (?,?)`, a.Name, a.InitialBalance)
	return err
}

func (d *DB) DeleteAccount(id int64) error {
	// Guard: check for referencing transactions
	var count int
	d.conn.QueryRow(`SELECT COUNT(*) FROM transactions WHERE account_id=?`, id).Scan(&count)
	if count > 0 {
		return fmt.Errorf("account has %d transaction(s); delete them first", count)
	}
	var tcount int
	d.conn.QueryRow(`SELECT COUNT(*) FROM transfers WHERE from_account_id=? OR to_account_id=?`, id, id).Scan(&tcount)
	if tcount > 0 {
		return fmt.Errorf("account has %d transfer(s); delete them first", tcount)
	}
	_, err := d.conn.Exec(`DELETE FROM accounts WHERE id=?`, id)
	return err
}

func (d *DB) GetAccountBalances() ([]AccountBalance, error) {
	rows, err := d.conn.Query(`
		SELECT a.id, a.name,
		  a.initial_balance
		  + COALESCE((SELECT SUM(amount) FROM transfers WHERE to_account_id   = a.id), 0)
		  - COALESCE((SELECT SUM(amount) FROM transfers WHERE from_account_id = a.id), 0)
		  + COALESCE((SELECT SUM(val) FROM transactions WHERE account_id = a.id AND type='revenue'), 0)
		  - COALESCE((SELECT SUM(val) FROM transactions WHERE account_id = a.id AND type='expense'), 0)
		  AS balance
		FROM accounts a ORDER BY a.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AccountBalance
	for rows.Next() {
		var ab AccountBalance
		if err := rows.Scan(&ab.ID, &ab.Name, &ab.Balance); err != nil {
			return nil, err
		}
		out = append(out, ab)
	}
	return out, rows.Err()
}

// ── Transfers ─────────────────────────────────────────────────────────────────

func (d *DB) GetTransfers() ([]Transfer, error) {
	rows, err := d.conn.Query(`
		SELECT t.id, t.from_account_id, t.to_account_id,
		       fa.name, ta.name, t.amount, t.date, t.desc
		FROM transfers t
		JOIN accounts fa ON fa.id = t.from_account_id
		JOIN accounts ta ON ta.id = t.to_account_id
		ORDER BY t.date DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Transfer
	for rows.Next() {
		var tr Transfer
		if err := rows.Scan(&tr.ID, &tr.FromAccountID, &tr.ToAccountID,
			&tr.FromAccountName, &tr.ToAccountName, &tr.Amount, &tr.Date, &tr.Desc); err != nil {
			return nil, err
		}
		out = append(out, tr)
	}
	return out, rows.Err()
}

func (d *DB) InsertTransfer(t Transfer) error {
	_, err := d.conn.Exec(`INSERT INTO transfers (from_account_id,to_account_id,amount,date,desc) VALUES (?,?,?,?,?)`,
		t.FromAccountID, t.ToAccountID, t.Amount, t.Date, t.Desc)
	return err
}

func (d *DB) DeleteTransfer(id int64) error {
	_, err := d.conn.Exec(`DELETE FROM transfers WHERE id=?`, id)
	return err
}

// ── Notes ─────────────────────────────────────────────────────────────────────

func (d *DB) GetNote(section, from, to string) (string, error) {
	var content string
	err := d.conn.QueryRow(`SELECT content FROM notes WHERE section=? AND period_from=? AND period_to=?`,
		section, from, to).Scan(&content)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return content, err
}

func (d *DB) UpsertNote(section, from, to, content string) error {
	_, err := d.conn.Exec(`
		INSERT INTO notes (section, period_from, period_to, content) VALUES (?,?,?,?)
		ON CONFLICT(section, period_from, period_to) DO UPDATE SET content=excluded.content`,
		section, from, to, content)
	return err
}

// ── Installments ──────────────────────────────────────────────────────────────

func (d *DB) GetInstallments() ([]Installment, error) {
	rows, err := d.conn.Query(`
		SELECT i.id, i.desc, i.cat, i.total_val, i.n_installments, i.start_date, i.account_id,
		       COUNT(t.id) as paid_count,
		       CAST(i.total_val AS REAL) / i.n_installments as monthly_val
		FROM installments i
		LEFT JOIN transactions t ON t.installment_id = i.id
		GROUP BY i.id
		ORDER BY i.start_date DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Installment
	for rows.Next() {
		var inst Installment
		if err := rows.Scan(&inst.ID, &inst.Desc, &inst.Cat, &inst.TotalVal, &inst.NInstallments,
			&inst.StartDate, &inst.AccountID, &inst.PaidCount, &inst.MonthlyVal); err != nil {
			return nil, err
		}
		out = append(out, inst)
	}
	return out, rows.Err()
}

func (d *DB) InsertInstallment(inst Installment) error {
	tx, err := d.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.Exec(`INSERT INTO installments (desc,cat,total_val,n_installments,start_date,account_id)
		VALUES (?,?,?,?,?,?)`,
		inst.Desc, inst.Cat, inst.TotalVal, inst.NInstallments, inst.StartDate, inst.AccountID)
	if err != nil {
		return err
	}
	instID, _ := res.LastInsertId()
	monthlyVal := inst.TotalVal / float64(inst.NInstallments)

	startDate, err := time.Parse("2006-01-02", inst.StartDate)
	if err != nil {
		return fmt.Errorf("invalid start_date: %w", err)
	}

	for i := int64(0); i < inst.NInstallments; i++ {
		txDate := startDate.AddDate(0, int(i), 0).Format("2006-01-02")
		_, err = tx.Exec(`INSERT INTO transactions (type,desc,cat,val,date,account_id,installment_id,installment_index)
			VALUES ('expense',?,?,?,?,?,?,?)`,
			inst.Desc, inst.Cat, monthlyVal, txDate, inst.AccountID, instID, i+1)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (d *DB) DeleteInstallment(id int64) error {
	tx, err := d.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM transactions WHERE installment_id=?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM installments WHERE id=?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

// ── Export / Import ───────────────────────────────────────────────────────────

type exportData struct {
	Transactions []Transaction `json:"transactions"`
	Accounts     []Account     `json:"accounts"`
	Transfers    []Transfer    `json:"transfers"`
	Installments []Installment `json:"installments"`
}

func (d *DB) ExportJSON() (string, error) {
	data, err := d.collectAll()
	if err != nil {
		return "", err
	}
	b, err := json.MarshalIndent(data, "", "  ")
	return string(b), err
}

func (d *DB) ExportCSV() (string, error) {
	data, err := d.collectAll()
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	w := csv.NewWriter(&sb)
	w.Write([]string{"id", "type", "desc", "cat", "val", "date", "account_id", "installment_id", "installment_index"})
	for _, t := range data.Transactions {
		w.Write([]string{
			nullInt64Str(t.ID), t.Type, t.Desc, t.Cat,
			fmt.Sprintf("%.2f", t.Val), t.Date,
			nullInt64Str(t.AccountID), nullInt64Str(t.InstallmentID), nullInt64Str(t.InstallmentIndex),
		})
	}
	w.Flush()
	return sb.String(), w.Error()
}

func nullInt64Str(v *int64) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%d", *v)
}

func (d *DB) ImportJSON(s string) error {
	var data exportData
	if err := json.Unmarshal([]byte(s), &data); err != nil {
		return err
	}
	tx, err := d.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	tx.Exec(`PRAGMA foreign_keys=OFF`)
	for _, tbl := range []string{"transactions", "transfers", "installments", "accounts", "notes"} {
		tx.Exec(`DELETE FROM ` + tbl)
	}

	for _, a := range data.Accounts {
		tx.Exec(`INSERT INTO accounts (id,name,initial_balance) VALUES (?,?,?)`, a.ID, a.Name, a.InitialBalance)
	}
	for _, inst := range data.Installments {
		tx.Exec(`INSERT INTO installments (id,desc,cat,total_val,n_installments,start_date,account_id) VALUES (?,?,?,?,?,?,?)`,
			inst.ID, inst.Desc, inst.Cat, inst.TotalVal, inst.NInstallments, inst.StartDate, inst.AccountID)
	}
	for _, t := range data.Transactions {
		tx.Exec(`INSERT INTO transactions (id,type,desc,cat,val,date,account_id,installment_id,installment_index) VALUES (?,?,?,?,?,?,?,?,?)`,
			t.ID, t.Type, t.Desc, t.Cat, t.Val, t.Date, t.AccountID, t.InstallmentID, t.InstallmentIndex)
	}
	for _, tr := range data.Transfers {
		tx.Exec(`INSERT INTO transfers (id,from_account_id,to_account_id,amount,date,desc) VALUES (?,?,?,?,?,?)`,
			tr.ID, tr.FromAccountID, tr.ToAccountID, tr.Amount, tr.Date, tr.Desc)
	}

	tx.Exec(`PRAGMA foreign_keys=ON`)
	return tx.Commit()
}

func (d *DB) collectAll() (exportData, error) {
	var data exportData
	txns, err := d.conn.Query(`SELECT id,type,desc,cat,val,date,account_id,installment_id,installment_index FROM transactions ORDER BY date`)
	if err != nil {
		return data, err
	}
	defer txns.Close()
	for txns.Next() {
		var t Transaction
		txns.Scan(&t.ID, &t.Type, &t.Desc, &t.Cat, &t.Val, &t.Date, &t.AccountID, &t.InstallmentID, &t.InstallmentIndex)
		data.Transactions = append(data.Transactions, t)
	}

	accs, err := d.conn.Query(`
		SELECT a.id, a.name, a.initial_balance,
		  a.initial_balance
		  + COALESCE((SELECT SUM(amount) FROM transfers WHERE to_account_id   = a.id), 0)
		  - COALESCE((SELECT SUM(amount) FROM transfers WHERE from_account_id = a.id), 0)
		  + COALESCE((SELECT SUM(val)    FROM transactions WHERE account_id = a.id AND type='revenue'), 0)
		  - COALESCE((SELECT SUM(val)    FROM transactions WHERE account_id = a.id AND type='expense'), 0)
		  AS current_balance
		FROM accounts a ORDER BY a.name`)
	if err != nil {
		return data, err
	}
	defer accs.Close()
	for accs.Next() {
		var a Account
		accs.Scan(&a.ID, &a.Name, &a.InitialBalance, &a.CurrentBalance)
		data.Accounts = append(data.Accounts, a)
	}

	insts, err := d.conn.Query(`SELECT id,desc,cat,total_val,n_installments,start_date,account_id FROM installments ORDER BY start_date`)
	if err != nil {
		return data, err
	}
	defer insts.Close()
	for insts.Next() {
		var i Installment
		insts.Scan(&i.ID, &i.Desc, &i.Cat, &i.TotalVal, &i.NInstallments, &i.StartDate, &i.AccountID)
		data.Installments = append(data.Installments, i)
	}

	trs, err := d.conn.Query(`SELECT id,from_account_id,to_account_id,amount,date,desc FROM transfers ORDER BY date`)
	if err != nil {
		return data, err
	}
	defer trs.Close()
	for trs.Next() {
		var t Transfer
		trs.Scan(&t.ID, &t.FromAccountID, &t.ToAccountID, &t.Amount, &t.Date, &t.Desc)
		data.Transfers = append(data.Transfers, t)
	}

	return data, nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func (d *DB) IsEmpty() bool {
	var accCount, txnCount int
	d.conn.QueryRow(`SELECT COUNT(*) FROM accounts`).Scan(&accCount)
	d.conn.QueryRow(`SELECT COUNT(*) FROM transactions`).Scan(&txnCount)
	return accCount == 0 && txnCount == 0
}
