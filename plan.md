Self Ledger TUI — Implementation Plan

 Context

 Reproduce the Self Ledger personal finance desktop app (Wails/Go + SvelteKit) as a terminal UI using Go + Bubble Tea. The TUI is
 a new, separate project — it replicates all functionality: transactions, accounts, transfers, installments, notes, backup, and
 export. Same SQLite database schema and format. No Wails, no frontend.

 ---
 Project Bootstrap

 mkdir self-ledger-tui && cd self-ledger-tui
 go mod init github.com/yourusername/self-ledger-tui

 Dependencies (go.mod)

 github.com/charmbracelet/bubbletea  v0.27+
 github.com/charmbracelet/bubbles    v0.20+
 github.com/charmbracelet/lipgloss   v1.0+
 github.com/guptarohit/asciigraph    v0.7+
 modernc.org/sqlite                  v1.x   (pure Go, no CGO)
 gopkg.in/yaml.v3                    v3.x

 bubbles components used: textinput, textarea, table, viewport, spinner, paginator

 ---
 File Structure

 self-ledger-tui/
 ├── main.go
 ├── internal/
 │   ├── db/
 │   │   ├── models.go      # all structs
 │   │   └── db.go          # DB connection + all queries
 │   ├── backup/
 │   │   └── backup.go      # git-based backup (same logic as original)
 │   ├── format/
 │   │   └── format.go      # currency, date, percent helpers
 │   ├── styles/
 │   │   └── styles.go      # lipgloss color constants + styles
 │   ├── components/
 │   │   ├── form.go        # reusable multi-field form component
 │   │   ├── barchart.go    # horizontal Unicode bar chart
 │   │   ├── linechart.go   # asciigraph wrapper
 │   │   └── confirm.go     # inline y/n confirmation
 │   └── views/
 │       ├── messages.go    # all tea.Msg types
 │       ├── cmds.go        # all DB calls wrapped as tea.Cmd
 │       ├── period.go      # shared period selector component
 │       ├── root.go        # root model, tab routing, status bar
 │       ├── dashboard.go   # Tab 0
 │       ├── ledger.go      # Tab 1
 │       ├── accounts.go    # Tab 2
 │       ├── transfers.go   # Tab 3
 │       ├── installments.go# Tab 4
 │       ├── notes.go       # Tab 5
 │       ├── backup.go      # Tab 6
 │       └── export.go      # Tab 7

 ---
 Database (internal/db/)

 Schema — identical to original app

 CREATE TABLE transactions (
     id INTEGER PRIMARY KEY AUTOINCREMENT,
     type TEXT NOT NULL,           -- "expense" | "revenue"
     desc TEXT NOT NULL,
     cat  TEXT NOT NULL,
     val  REAL NOT NULL,
     date TEXT NOT NULL,           -- YYYY-MM-DD
     account_id        INTEGER REFERENCES accounts(id),
     installment_id    INTEGER REFERENCES installments(id),
     installment_index INTEGER
 );
 CREATE TABLE accounts (
     id INTEGER PRIMARY KEY AUTOINCREMENT,
     name TEXT NOT NULL UNIQUE,
     initial_balance REAL NOT NULL DEFAULT 0
 );
 CREATE TABLE transfers (
     id INTEGER PRIMARY KEY AUTOINCREMENT,
     from_account_id INTEGER NOT NULL REFERENCES accounts(id),
     to_account_id   INTEGER NOT NULL REFERENCES accounts(id),
     amount REAL NOT NULL,
     date   TEXT NOT NULL,
     desc   TEXT NOT NULL DEFAULT ''
 );
 CREATE TABLE notes (
     section     TEXT NOT NULL,
     period_from TEXT NOT NULL,
     period_to   TEXT NOT NULL,
     content     TEXT NOT NULL DEFAULT '',
     PRIMARY KEY (section, period_from, period_to)
 );
 CREATE TABLE installments (
     id             INTEGER PRIMARY KEY AUTOINCREMENT,
     desc           TEXT NOT NULL,
     cat            TEXT NOT NULL,
     total_val      REAL NOT NULL,
     n_installments INTEGER NOT NULL,
     start_date     TEXT NOT NULL,
     account_id     INTEGER REFERENCES accounts(id)
 );

 Migrations on startup (silent if column exists):
 ALTER TABLE transactions ADD COLUMN account_id INTEGER;
 ALTER TABLE transactions ADD COLUMN installment_id INTEGER;
 ALTER TABLE transactions ADD COLUMN installment_index INTEGER;

 DB Open

 db, _ := sql.Open("sqlite", path+"?_journal=WAL&_busy_timeout=5000")
 db.SetMaxOpenConns(1)

 All DB Methods

 ┌─────────────────────────────────┬───────────────────────────────────────────────────────────────────────────────────────────┐
 │             Method              │                                        SQL / Logic                                        │
 ├─────────────────────────────────┼───────────────────────────────────────────────────────────────────────────────────────────┤
 │ QueryTransactions(from, to)     │ SELECT … WHERE date BETWEEN ? AND ? ORDER BY date DESC                                    │
 ├─────────────────────────────────┼───────────────────────────────────────────────────────────────────────────────────────────┤
 │ InsertTransaction(t)            │ INSERT INTO transactions …                                                                │
 ├─────────────────────────────────┼───────────────────────────────────────────────────────────────────────────────────────────┤
 │ UpdateTransaction(t)            │ UPDATE transactions SET … WHERE id=?                                                      │
 ├─────────────────────────────────┼───────────────────────────────────────────────────────────────────────────────────────────┤
 │ DeleteTransaction(id)           │ DELETE FROM transactions WHERE id=?                                                       │
 ├─────────────────────────────────┼───────────────────────────────────────────────────────────────────────────────────────────┤
 │ GetAccounts()                   │ SELECT id, name, initial_balance FROM accounts ORDER BY name                              │
 ├─────────────────────────────────┼───────────────────────────────────────────────────────────────────────────────────────────┤
 │ InsertAccount(a)                │ INSERT INTO accounts …                                                                    │
 ├─────────────────────────────────┼───────────────────────────────────────────────────────────────────────────────────────────┤
 │ DeleteAccount(id)               │ DELETE FROM accounts WHERE id=?                                                           │
 ├─────────────────────────────────┼───────────────────────────────────────────────────────────────────────────────────────────┤
 │ GetAccountBalances()            │ Complex SELECT: initial_balance + transfers_in - transfers_out + revenue - expenses (see  │
 │                                 │ formula below)                                                                            │
 ├─────────────────────────────────┼───────────────────────────────────────────────────────────────────────────────────────────┤
 │ GetTransfers()                  │ SELECT with JOIN on accounts for names, ORDER BY date DESC                                │
 ├─────────────────────────────────┼───────────────────────────────────────────────────────────────────────────────────────────┤
 │ InsertTransfer(t)               │ INSERT INTO transfers …                                                                   │
 ├─────────────────────────────────┼───────────────────────────────────────────────────────────────────────────────────────────┤
 │ DeleteTransfer(id)              │ DELETE FROM transfers WHERE id=?                                                          │
 ├─────────────────────────────────┼───────────────────────────────────────────────────────────────────────────────────────────┤
 │ GetNote(sec,from,to)            │ SELECT content … returns "" if not found                                                  │
 ├─────────────────────────────────┼───────────────────────────────────────────────────────────────────────────────────────────┤
 │ UpsertNote(sec,from,to,content) │ INSERT … ON CONFLICT DO UPDATE SET content=excluded.content                               │
 ├─────────────────────────────────┼───────────────────────────────────────────────────────────────────────────────────────────┤
 │ InsertInstallment(inst)         │ INSERT installment row + loop N times inserting monthly expense transactions              │
 ├─────────────────────────────────┼───────────────────────────────────────────────────────────────────────────────────────────┤
 │ GetInstallments()               │ SELECT with COUNT(transactions) as paid_count, compute monthly_val = total_val/n          │
 ├─────────────────────────────────┼───────────────────────────────────────────────────────────────────────────────────────────┤
 │ DeleteInstallment(id)           │ DELETE transactions WHERE installment_id=? then DELETE installments WHERE id=?            │
 ├─────────────────────────────────┼───────────────────────────────────────────────────────────────────────────────────────────┤
 │ ExportJSON()                    │ Serialize all tables to JSON string                                                       │
 ├─────────────────────────────────┼───────────────────────────────────────────────────────────────────────────────────────────┤
 │ ExportCSV()                     │ Serialize to CSV string                                                                   │
 ├─────────────────────────────────┼───────────────────────────────────────────────────────────────────────────────────────────┤
 │ ImportJSON(s)                   │ Full DB replace inside transaction (PRAGMA foreign_keys=OFF, DELETE all, re-insert, fix   │
 │                                 │ sequences)                                                                                │
 ├─────────────────────────────────┼───────────────────────────────────────────────────────────────────────────────────────────┤
 │ IsEmpty()                       │ true if no accounts AND no transactions                                                   │
 └─────────────────────────────────┴───────────────────────────────────────────────────────────────────────────────────────────┘

 Account balance formula:
 SELECT a.id, a.name,
   a.initial_balance
   + COALESCE((SELECT SUM(amount) FROM transfers WHERE to_account_id = a.id), 0)
   - COALESCE((SELECT SUM(amount) FROM transfers WHERE from_account_id = a.id), 0)
   + COALESCE((SELECT SUM(val) FROM transactions WHERE account_id = a.id AND type='revenue'), 0)
   - COALESCE((SELECT SUM(val) FROM transactions WHERE account_id = a.id AND type='expense'), 0)
   AS balance
 FROM accounts a ORDER BY a.name

 Structs (internal/db/models.go)

 type Transaction struct {
     ID               *int64
     Type             string   // "expense" | "revenue"
     Desc, Cat        string
     Val              float64
     Date             string   // YYYY-MM-DD
     AccountID        *int64
     InstallmentID    *int64
     InstallmentIndex *int64
 }
 type Installment struct {
     ID            *int64
     Desc, Cat     string
     TotalVal      float64
     NInstallments int64
     StartDate     string
     AccountID     *int64
     PaidCount     *int64   // computed
     MonthlyVal    *float64 // computed = TotalVal/NInstallments
 }
 type Account struct {
     ID             *int64
     Name           string
     InitialBalance float64
 }
 type AccountBalance struct{ ID int64; Name string; Balance float64 }
 type Transfer struct {
     ID              *int64
     FromAccountID   int64
     ToAccountID     int64
     FromAccountName *string
     ToAccountName   *string
     Amount          float64
     Date, Desc      string
 }
 type BackupConfig struct {
     Provider string `yaml:"provider"` // github/gitlab/forgejo/gitea/custom
     Host     string `yaml:"host"`
     Repo     string `yaml:"repo"`
     Token    string `yaml:"token"`
 }

 ---
 Backup (internal/backup/backup.go)

 Identical logic to original backup.go. Copy and adapt (remove Wails imports).

 Key functions:
 - RemoteURL(cfg BackupConfig) string — builds https://token@host/repo.git
 - NewBackupManager() *BackupManager — paths: ~/.config/self-ledger/backup.yaml, ~/.config/self-ledger/backup-repo/
 - LoadConfig() (BackupConfig, error)
 - SaveConfig(cfg BackupConfig) error
 - BackupNow(jsonData string) error — git init/add/commit/push
 - FetchBackup() (string, error) — git clone or fetch + checkout
 - AutoRestoreIfNeeded(db *db.DB) (bool, error) — if db.IsEmpty() + config exists, fetch + import

 Auto-backup on quit: call BackupNow inside a goroutine with context.WithTimeout(10s) when app receives quit signal.

 ---
 Format Helpers (internal/format/format.go)

 func Currency(v float64) string        // 1234567.89 → "R$ 1.234.567,89"
 func DateDisplay(d string) string      // "2026-04-19" → "19 Apr 2026"
 func ParseDate(s string) (string, error) // accepts YYYY-MM-DD
 func PctDelta(cur, prev float64) string  // "+12.3%" / "-5.6%" / "N/A"
 func TodayISO() string                  // current date as YYYY-MM-DD
 func PeriodDates(mode string) (from, to string) // "1m"/"3m"/"6m"/"1y" → dates

 ---
 Style Constants (internal/styles/styles.go)

 const (
     ColorExpense  = lipgloss.Color("#8c1f1f")
     ColorRevenue  = lipgloss.Color("#1a5c30")
     ColorFaint    = lipgloss.Color("#555555")
     ColorAccent   = lipgloss.Color("#4a90d9")
     ColorError    = lipgloss.Color("#b71c1c")
     ColorSuccess  = lipgloss.Color("#2e7d32")
     ColorNeutral  = lipgloss.Color("#cccccc")
 )

 Define var lipgloss styles for: TabActive, TabInactive, Box, Button, ButtonFocus, Input, InputFocus, Error, Success, Title,
 Faint.

 ---
 Shared Components

 Form (internal/components/form.go)

 type FieldType int  // FieldText, FieldNumber, FieldDate, FieldSelect, FieldToggle

 type Field struct {
     Label       string
     Type        FieldType
     Input       textinput.Model   // for text/number/date
     Options     []string          // for FieldSelect
     SelectedIdx int
     Required    bool
     Error       string
 }

 type FormModel struct {
     Fields      []Field
     FocusIdx    int
     Title       string
     SubmitLabel string
 }

 - Tab/Shift+Tab moves focus between fields
 - FieldSelect uses Up/Down to cycle options
 - FieldToggle uses Space/Enter to toggle
 - Validate() bool checks required + numeric parsing
 - Values() map[string]string extracts current values
 - Reset() clears all inputs

 Bar Chart (internal/components/barchart.go)

 type BarEntry struct{ Label string; Value, Pct float64; Color lipgloss.Color }

 func RenderBarChart(entries []BarEntry, maxWidth int, title string) string

 Each bar: LabelPadded  ████████░░  34.5%  R$ 1.234,56

 Unicode: █ filled, ░ empty. Max bar width = maxWidth - longestLabel - 22.

 Line Chart (internal/components/linechart.go)

 func RenderLineChart(expenses, revenues []float64, labels []string, width, height int) string

 Uses asciigraph.PlotMany with SeriesColors: []asciigraph.AnsiColor{asciigraph.Red, asciigraph.Green}. Wraps in a lipgloss box.
 Returns "No data for this period" placeholder if empty.

 Confirm (internal/components/confirm.go)

 Renders inline: Delete 'Groceries' on 19 Apr 2026? [y]es / [n]o
 Handles y/n keys, emits ConfirmResultMsg{ID int64; Confirmed bool}.

 ---
 Root Architecture (internal/views/root.go)

 type RootModel struct {
     db           *db.DB
     activeTab    int
     tabs         []string  // ["dashboard","ledger","accounts","transfers","installments","notes","backup","export"]
     width, height int
     period       PeriodModel
     dashboard    DashboardModel
     ledger       LedgerModel
     accounts     AccountsModel
     transfers    TransfersModel
     installments InstallmentsModel
     notes        NotesModel
     backup       BackupModel
     export       ExportModel
     statusMsg    string
     statusErr    bool
     statusTicks  int
     loading      bool
     spinner      spinner.Model
 }

 Tab switching: keys 1–8 or Tab/Shift+Tab. Tab bar renders as horizontal strip at top.

 Layout (top to bottom):
 [ tab bar                                    ] 1 line
 [ period selector                            ] 1 line
 [ tab content (viewport, height-4 lines)     ]
 [ status bar                                 ] 1 line

 Init(): Batch load: transactions, accounts, transfers, installments, backup config, spinner tick.

 tea.WindowSizeMsg: Distribute width/height to all sub-models and viewports.

 PeriodChangedMsg: Re-fire loadTransactions + loadSummary + loadVariation for dashboard and ledger.

 Auto-backup on quit: Catch tea.KeyMsg{Type: tea.KeyCtrlC} → run autoBackupCmd in goroutine → tea.Quit.

 ---
 Message Types (internal/views/messages.go)

 type TransactionsLoadedMsg  struct{ Txns []db.Transaction }
 type AccountsLoadedMsg      struct{ Balances []db.AccountBalance; Accounts []db.Account }
 type TransfersLoadedMsg     struct{ Transfers []db.Transfer }
 type InstallmentsLoadedMsg  struct{ Installments []db.Installment }
 type NoteLoadedMsg          struct{ Section, From, To, Content string }
 type PeriodChangedMsg       struct{ From, To string }
 type InsertDoneMsg          struct{ Err error; EntityType string }
 type UpdateDoneMsg          struct{ Err error }
 type DeleteDoneMsg          struct{ Err error }
 type BackupDoneMsg          struct{ Err error }
 type RestoreDoneMsg         struct{ Err error }
 type ExportDoneMsg          struct{ Path string; Err error }
 type StatusMsg              struct{ Text string; IsErr bool }
 type TickMsg                time.Time
 type ConfirmResultMsg       struct{ ID int64; Confirmed bool }

 ---
 Cmd Wrappers (internal/views/cmds.go)

 Pattern for all DB commands:
 func loadTransactions(d *db.DB, from, to string) tea.Cmd {
     return func() tea.Msg {
         txns, err := d.QueryTransactions(from, to)
         if err != nil { return StatusMsg{err.Error(), true} }
         return TransactionsLoadedMsg{txns}
     }
 }

 One tea.Cmd per DB operation. On success, mutation cmds trigger a reload cmd via tea.Batch.

 ---
 Tab Specs

 Tab 0 — Dashboard

 State: summary totals, last-4 records, catBreakdown, lineData, varExp, varRev, quickAddExpForm, quickAddRevForm, addMode,
 viewport

 Sections (scrollable via viewport):
 1. Summary row: Expenses: R$ X  |  Revenues: R$ X  |  Balance: R$ X
 2. Variation: Expenses: +12.3% vs prev period  |  Revenues: -5.6%
 3. Quick-add buttons: [e] Add Expense  [r] Add Revenue
 4. Inline form (when active): amount, desc, category, date, account, optional N installments
 5. Recent (last 4): compact rows R$ val  desc · cat  date
 6. Category bar charts: side by side — expenses left (red bars), revenues right (green bars)
 7. Line chart: daily bucketed expenses + revenues using asciigraph

 Key bindings: e/r open forms, Esc closes, Tab moves fields, Enter submits, j/k scrolls viewport.

 Tab 1 — Ledger

 State: allTxns, page (15/page), paginator, editID, editForm, deleteConfirmID, filterInput, table

 Rendering: Filter bar at top + bubbles table (Date | Type | Desc | Cat | Amount | Account) + pagination row.

 Key bindings: j/k navigate, e edit, d delete prompt, y/n confirm, / filter, n/p page, Esc cancel.

 Tab 2 — Accounts

 State: accounts ([]AccountBalance), addForm, showAdd, deleteConfirmID, cursor

 Rendering: List of account cards Name  Balance (green/red), total balance at bottom, [n] New Account.

 Key bindings: n add, d delete, j/k navigate, Esc cancel.

 Tab 3 — Transfers

 State: transfers, accounts (for dropdowns), addForm, page, paginator, deleteConfirmID, cursor

 Rendering: Transfer form at top + paginated list Date | From → To | Amount | Desc.
 From/To fields: FieldSelect cycling through account names.
 Validate: from ≠ to before insert.

 Tab 4 — Installments

 State: installments, addForm, showAdd, deleteConfirmID, cursor

 Rendering: List Desc | Cat | R$ monthly × N | ████░░ paid/total | date.
 Progress bar: strings.Repeat("█", paid) + strings.Repeat("░", total-paid).
 [n] New Installment button.

 Tab 5 — Notes

 State: expTextarea (textarea.Model), revTextarea (textarea.Model), activeNote, period, saved

 Rendering: Two labeled textareas. Active one has highlighted border. [Ctrl+S] Save.

 Key bindings: Tab switch between areas, Ctrl+S save both, Esc blur.

 Tab 6 — Backup

 State: config (BackupConfig), configForm, editingConfig, statusLine, loading, spinner

 Rendering: Current config display (token masked ****) + buttons [e] Edit Config  [b] Backup Now  [r] Restore + status line.

 Key bindings: e edit, b backup, r restore, Esc cancel edit, Enter save config.

 Tab 7 — Export

 State: pathInput (textinput.Model), format ("json"/"csv"), status

 Rendering: [j] Export JSON  [c] Export CSV → on select: Save to: [~/self-ledger-export-2026-04-19.json] path prompt.

 Key bindings: j/c select format, Enter confirm path, Esc cancel.

 Export Cmd:
 data := d.ExportJSON() // or ExportCSV()
 home, _ := os.UserHomeDir()
 fullPath := strings.Replace(path, "~", home, 1)
 os.WriteFile(fullPath, []byte(data), 0644)

 ---
 Period Selector (internal/views/period.go)

 State: mode (1m/3m/6m/1y/custom), fromInput, toInput, From/To strings

 Rendering: [1m] [3m] [6m] [1y] | 2026-01-01 → 2026-04-19

 Key bindings: 1–4 quick-select, c custom mode, Tab between date inputs, Enter apply.

 Emits PeriodChangedMsg{From, To} caught by root.

 ---
 main.go

 func main() {
     dbPath := resolveDBPath()    // ~/.config/self-ledger/self_ledger.db
     d, err := db.Open(dbPath)
     // ... error handling
     defer d.Close()

     bm, _ := backup.NewBackupManager()
     if restored, _ := backup.AutoRestoreIfNeeded(d, bm); restored {
         fmt.Println("Auto-restored from backup")
     }

     root := views.NewRootModel(d, bm)
     p := tea.NewProgram(root, tea.WithAltScreen(), tea.WithMouseCellMotion())
     if _, err := p.Run(); err != nil {
         log.Fatal(err)
     }
 }

 func resolveDBPath() string {
     cfg, _ := os.UserConfigDir()
     dir := filepath.Join(cfg, "self-ledger")
     os.MkdirAll(dir, 0750)
     return filepath.Join(dir, "self_ledger.db")
 }

 ---
 Implementation Order

 1. internal/db/models.go + internal/db/db.go — entire data layer
 2. internal/backup/backup.go — copy + adapt from original
 3. internal/format/format.go — formatting helpers
 4. internal/styles/styles.go — color/style constants
 5. internal/components/form.go — form component
 6. internal/components/barchart.go + linechart.go + confirm.go
 7. internal/views/messages.go + cmds.go — message/cmd contracts
 8. internal/views/period.go — period selector
 9. internal/views/root.go — skeleton with tab routing, status bar, window resize
 10. Views in order: dashboard → ledger → accounts → transfers → installments → notes → backup → export
 11. main.go — wire everything, launch program

 ---
 Edge Cases

 - Empty state: every list view shows a helpful prompt when empty
 - Loading: root spinner visible during initial load; per-tab spinners for backup/export ops
 - Errors: DB errors → red status bar (3s); form validation errors → inline below field
 - Window resize: all viewports and chart widths recalculate on tea.WindowSizeMsg
 - Delete guards: check for orphaned FK references before deleting accounts; show error if blocked
 - Period custom validation: both dates valid + from ≤ to before emitting PeriodChangedMsg
 - Transfer validation: from ≠ to before insert
 - Installment UX: toggle field in expense add form; when toggled, N + start_date fields appear; calls InsertInstallment not
 InsertTransaction
 - Auto-backup on quit: Ctrl+C triggers 10s-timeout backup goroutine before tea.Quit

 ---
 Verification

 1. go build ./... — compiles without errors
 2. Run app, confirm tab bar renders all 8 tabs
 3. Add expense → verify appears in Dashboard recent list and Ledger
 4. Add account → verify balance shows in Accounts and navbar
 5. Add transfer → verify balances update correctly
 6. Add installment → verify N expense transactions appear in Ledger
 7. Edit transaction in Ledger → verify update persists
 8. Delete transaction → verify removed
 9. Switch periods (1m/3m/6m/1y/custom) → verify data reloads and charts update
 10. Notes tab → type note, Ctrl+S, switch period and back → verify note persists
 11. Export JSON/CSV → verify file written at specified path
 12. Backup tab → configure repo, backup now → verify git push succeeds
 13. Restore → verify full DB replace
 14. Quit app → verify auto-backup fires (check git log on remote repo)
 15. Reopen app → verify data intact
