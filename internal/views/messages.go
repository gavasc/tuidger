package views

import (
	"time"

	"github.com/gavasc/tuidger/internal/db"
)

type TransactionsLoadedMsg struct{ Txns []db.Transaction }
type AccountsLoadedMsg struct {
	Balances []db.AccountBalance
	Accounts []db.Account
}
type TransfersLoadedMsg struct{ Transfers []db.Transfer }
type InstallmentsLoadedMsg struct{ Installments []db.Installment }
type NoteLoadedMsg struct{ Section, From, To, Content string }
type PeriodChangedMsg struct{ From, To string }
type InsertDoneMsg struct {
	Err        error
	EntityType string
}
type UpdateDoneMsg struct{ Err error }
type DeleteDoneMsg struct{ Err error }
type BackupDoneMsg struct{ Err error }
type RestoreDoneMsg struct{ Err error }
type ExportDoneMsg struct {
	Path string
	Err  error
}
type StatusMsg struct {
	Text  string
	IsErr bool
}
type TickMsg time.Time
