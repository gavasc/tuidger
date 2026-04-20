package views

import (
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gavasc/tuidger/internal/backup"
	"github.com/gavasc/tuidger/internal/db"
)

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return TickMsg(t) })
}

// ── Load ──────────────────────────────────────────────────────────────────────

func loadTransactions(d *db.DB, from, to string) tea.Cmd {
	return func() tea.Msg {
		txns, err := d.QueryTransactions(from, to)
		if err != nil {
			return StatusMsg{Text: "DB error: " + err.Error(), IsErr: true}
		}
		return TransactionsLoadedMsg{Txns: txns}
	}
}

func loadAccounts(d *db.DB) tea.Cmd {
	return func() tea.Msg {
		balances, err := d.GetAccountBalances()
		if err != nil {
			return StatusMsg{Text: "DB error: " + err.Error(), IsErr: true}
		}
		accounts, err := d.GetAccounts()
		if err != nil {
			return StatusMsg{Text: "DB error: " + err.Error(), IsErr: true}
		}
		return AccountsLoadedMsg{Balances: balances, Accounts: accounts}
	}
}

func loadTransfers(d *db.DB) tea.Cmd {
	return func() tea.Msg {
		transfers, err := d.GetTransfers()
		if err != nil {
			return StatusMsg{Text: "DB error: " + err.Error(), IsErr: true}
		}
		return TransfersLoadedMsg{Transfers: transfers}
	}
}

func loadInstallments(d *db.DB) tea.Cmd {
	return func() tea.Msg {
		insts, err := d.GetInstallments()
		if err != nil {
			return StatusMsg{Text: "DB error: " + err.Error(), IsErr: true}
		}
		return InstallmentsLoadedMsg{Installments: insts}
	}
}

func loadNote(d *db.DB, section, from, to string) tea.Cmd {
	return func() tea.Msg {
		content, err := d.GetNote(section, from, to)
		if err != nil {
			return StatusMsg{Text: "DB error: " + err.Error(), IsErr: true}
		}
		return NoteLoadedMsg{Section: section, From: from, To: to, Content: content}
	}
}

// ── Mutations ─────────────────────────────────────────────────────────────────

func insertTransaction(d *db.DB, t db.Transaction, from, to string) tea.Cmd {
	return func() tea.Msg {
		if err := d.InsertTransaction(t); err != nil {
			return StatusMsg{Text: "Insert error: " + err.Error(), IsErr: true}
		}
		return InsertDoneMsg{EntityType: "transaction"}
	}
}

func updateTransaction(d *db.DB, t db.Transaction) tea.Cmd {
	return func() tea.Msg {
		if err := d.UpdateTransaction(t); err != nil {
			return StatusMsg{Text: "Update error: " + err.Error(), IsErr: true}
		}
		return UpdateDoneMsg{}
	}
}

func deleteTransaction(d *db.DB, id int64) tea.Cmd {
	return func() tea.Msg {
		if err := d.DeleteTransaction(id); err != nil {
			return StatusMsg{Text: "Delete error: " + err.Error(), IsErr: true}
		}
		return DeleteDoneMsg{}
	}
}

func insertAccount(d *db.DB, a db.Account) tea.Cmd {
	return func() tea.Msg {
		if err := d.InsertAccount(a); err != nil {
			return StatusMsg{Text: "Insert error: " + err.Error(), IsErr: true}
		}
		return InsertDoneMsg{EntityType: "account"}
	}
}

func deleteAccount(d *db.DB, id int64) tea.Cmd {
	return func() tea.Msg {
		if err := d.DeleteAccount(id); err != nil {
			return StatusMsg{Text: err.Error(), IsErr: true}
		}
		return DeleteDoneMsg{}
	}
}

func insertTransfer(d *db.DB, t db.Transfer) tea.Cmd {
	return func() tea.Msg {
		if err := d.InsertTransfer(t); err != nil {
			return StatusMsg{Text: "Insert error: " + err.Error(), IsErr: true}
		}
		return InsertDoneMsg{EntityType: "transfer"}
	}
}

func deleteTransfer(d *db.DB, id int64) tea.Cmd {
	return func() tea.Msg {
		if err := d.DeleteTransfer(id); err != nil {
			return StatusMsg{Text: "Delete error: " + err.Error(), IsErr: true}
		}
		return DeleteDoneMsg{}
	}
}

func insertInstallment(d *db.DB, inst db.Installment) tea.Cmd {
	return func() tea.Msg {
		if err := d.InsertInstallment(inst); err != nil {
			return StatusMsg{Text: "Insert error: " + err.Error(), IsErr: true}
		}
		return InsertDoneMsg{EntityType: "installment"}
	}
}

func deleteInstallment(d *db.DB, id int64) tea.Cmd {
	return func() tea.Msg {
		if err := d.DeleteInstallment(id); err != nil {
			return StatusMsg{Text: "Delete error: " + err.Error(), IsErr: true}
		}
		return DeleteDoneMsg{}
	}
}

func upsertNote(d *db.DB, section, from, to, content string) tea.Cmd {
	return func() tea.Msg {
		if err := d.UpsertNote(section, from, to, content); err != nil {
			return StatusMsg{Text: "Save error: " + err.Error(), IsErr: true}
		}
		return StatusMsg{Text: "Note saved", IsErr: false}
	}
}

func exportCmd(d *db.DB, format, path string) tea.Cmd {
	return func() tea.Msg {
		home, _ := os.UserHomeDir()
		fullPath := strings.Replace(path, "~", home, 1)
		var data string
		var err error
		if format == "json" {
			data, err = d.ExportJSON()
		} else {
			data, err = d.ExportCSV()
		}
		if err != nil {
			return ExportDoneMsg{Err: err}
		}
		if err := os.WriteFile(fullPath, []byte(data), 0644); err != nil {
			return ExportDoneMsg{Err: err}
		}
		return ExportDoneMsg{Path: fullPath}
	}
}

func backupNowCmd(d *db.DB, bm *backup.BackupManager) tea.Cmd {
	return func() tea.Msg {
		jsonData, err := d.ExportJSON()
		if err != nil {
			return BackupDoneMsg{Err: err}
		}
		if err := bm.BackupNow(jsonData); err != nil {
			return BackupDoneMsg{Err: err}
		}
		return BackupDoneMsg{}
	}
}

func restoreCmd(d *db.DB, bm *backup.BackupManager) tea.Cmd {
	return func() tea.Msg {
		jsonData, err := bm.FetchBackup()
		if err != nil {
			return RestoreDoneMsg{Err: err}
		}
		if err := d.ImportJSON(jsonData); err != nil {
			return RestoreDoneMsg{Err: err}
		}
		return RestoreDoneMsg{}
	}
}
