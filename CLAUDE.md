# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working in this repository.

## Project

Self Ledger TUI — a terminal UI replication of the Self Ledger personal finance app (originally Wails/Go + SvelteKit). Implements transactions, accounts, transfers, installments, notes, backup, and export using the **same SQLite schema** as the desktop app.

Stack: Go + Bubble Tea (Charmbracelet) + pure-Go SQLite (`modernc.org/sqlite`, no CGO).
Module: `github.com/gavasc/tuidger`

## Commands

```bash
go mod tidy                      # sync dependencies
go build ./...                   # compile everything
go run .                         # run the app
go test ./...                    # run all tests
go test ./internal/db/...        # run a specific package's tests
go vet ./...                     # static analysis
```

## Implementation Checklist

### Foundation
- [x] **Step 1** — `internal/db/models.go` + `internal/db/db.go` — full DB layer, all queries, schema migrations, export/import
- [x] **Step 2** — `internal/backup/backup.go` — git-based backup, auto-restore, 10s-timeout quit backup
- [x] **Step 3** — `internal/format/format.go` — Currency (R$), DateDisplay, ParseDate, ParseFloat, PctDelta, TodayISO, PeriodDates
- [x] **Step 4** — `internal/styles/styles.go` — lipgloss color constants + shared styles

### Components
- [x] **Step 5** — `internal/components/form.go` — multi-field form (Text/Number/Date/Select/Toggle), Tab navigation, Validate, Values, Reset
- [x] **Step 6a** — `internal/components/barchart.go` — horizontal Unicode bar chart with percentages
- [x] **Step 6b** — `internal/components/linechart.go` — dual-series asciigraph line chart (expenses red, revenues green)
- [x] **Step 6c** — `internal/components/confirm.go` — inline y/n confirmation dialog, emits `ConfirmResultMsg`

### View Layer
- [x] **Step 7** — `internal/views/messages.go` + `internal/views/cmds.go` — all Msg types and DB-wrapped Cmds
- [x] **Step 8** — `internal/views/period.go` — period selector (1m/3m/6m/1y/custom), emits `PeriodChangedMsg`
- [x] **Step 9** — `internal/views/root.go` — root model: tab routing, status bar (3s auto-dismiss), window resize dispatch, spinner, auto-backup on Ctrl+C
- [x] **Step 10a** — `internal/views/dashboard.go` — Tab 0: summary totals, quick-add forms, recent 4, category bar charts, daily line chart
- [x] **Step 10b** — `internal/views/ledger.go` — Tab 1: paginated table (15/page), edit/delete with confirm
- [x] **Step 10c** — `internal/views/accounts.go` — Tab 2: account list with balances, add/delete
- [x] **Step 10d** — `internal/views/transfers.go` — Tab 3: transfer form + paginated list, from≠to validation
- [x] **Step 10e** — `internal/views/installments.go` — Tab 4: installment list with progress bars, add/delete cascade
- [x] **Step 10f** — `internal/views/notes.go` — Tab 5: dual textareas, Ctrl+S save, period-aware
- [x] **Step 10g** — `internal/views/backup_tab.go` — Tab 6: config display, backup now, restore, spinner
- [x] **Step 10h** — `internal/views/export.go` — Tab 7: JSON/CSV export with path prompt
- [x] **Step 11** — `main.go` — entry point: open DB, auto-restore, launch tea.Program with AltScreen

### Status
**All steps complete. `go build ./...` and `go vet ./...` pass clean.**

Next: run the app (`go run .`) and verify against the plan's verification checklist (plan.md lines 538–552).

---

## Architecture

### File Layout

```
main.go                          # entry point: open DB, init root model, run tea.Program
internal/
  db/
    models.go                    # all DB structs (Transaction, Account, Transfer, etc.)
    db.go                        # DB connection + every SQL query method
  backup/
    backup.go                    # git-based backup to GitHub/GitLab/Gitea/custom
  format/
    format.go                    # currency (R$), date, percent helpers
  styles/
    styles.go                    # lipgloss color constants + shared styles
  components/
    form.go                      # reusable multi-field form (Text/Number/Date/Select/Toggle)
    barchart.go                  # horizontal Unicode bar chart with percentages
    linechart.go                 # asciigraph dual-series line chart
    confirm.go                   # inline y/n confirmation dialog
  views/
    messages.go                  # all tea.Msg types
    cmds.go                      # all DB calls wrapped as tea.Cmd
    period.go                    # shared period selector component
    root.go                      # root model: tab routing, period state, status bar
    dashboard.go                 # tab 0
    ledger.go                    # tab 1
    accounts.go                  # tab 2
    transfers.go                 # tab 3
    installments.go              # tab 4
    notes.go                     # tab 5
    backup_tab.go                # tab 6
    export.go                    # tab 7
```

### Bubble Tea Patterns

- **Message/Command contract**: every DB operation lives in `cmds.go` as a `tea.Cmd` that returns a typed `tea.Msg` from `messages.go`. Tabs never call DB methods directly.
- **Mutation flow**: DB mutation Cmd → `InsertDoneMsg`/`UpdateDoneMsg`/`DeleteDoneMsg` → root dispatches `reloadAll()` → `*LoadedMsg` → tab re-renders.
- **Root model** owns: active tab index, period (`from`/`to` dates, resolution), status bar text, spinner, window dimensions. It delegates `Update`/`View` to the active tab.
- **Period changes** broadcast `PeriodChangedMsg` so every tab can re-fetch on next activation.
- **Status bar**: transient messages auto-dismiss after 3 s via `TickMsg`. Errors render in red.
- **`Capturing()` method**: each tab exposes `Capturing() bool`; root uses this to decide whether number keys 1–8 should switch tabs or be forwarded to the active tab.

### Key Behaviours

- **Config/data paths**: DB at `~/.config/self-ledger/self_ledger.db`; backup config at `~/.config/self-ledger/backup.yaml`.
- **Account balance**: `initial_balance + transfers_in − transfers_out + revenue − expenses` (computed by DB query, not in-memory).
- **Backup**: git clone/pull/commit/push to remote; auto-restore on startup when DB is empty; auto-backup on Ctrl+C with 10 s timeout.
- **Installments**: each installment generates N individual transactions linked via `installment_id`; deleting an installment cascades to all linked transactions.
- **Export**: JSON or CSV, written to `~/self-ledger-export-YYYY-MM-DD.{json|csv}` by default.

### Tab Keybindings (root level)

| Key | Action |
|-----|--------|
| `1`–`8` | Switch to tab by number (when active tab not capturing) |
| `Ctrl+C` | Quit (triggers auto-backup) |

### Known Limitations / Future Work

- Ledger filter (`/` key) clears the filter but doesn't open a filter-input mode yet — full filter-input UI can be added as a follow-up.
- `PctDelta` is imported in `format` but not yet wired to Dashboard variation row — can be added to `buildContent()`.
- Transfers pagination uses `left`/`right` arrow keys (plan said `n`/`p`); `n` was already used for "new transfer".
