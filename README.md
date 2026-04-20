# tuidger

A terminal-based personal finance manager. Track expenses, revenues, and account balances entirely from the keyboard. No cloud, no accounts, no internet. All data lives in a SQLite database on your machine.

## Features

- Add expenses and revenues with description, category, date, and account
- Manage multiple accounts with running balances
- Record transfers between accounts
- Split purchases into N monthly installments
- Dashboard with recent transactions, category bar charts, and a daily spending/revenue line chart
- Paginated ledger with inline edit and delete
- Period selector — 1m / 3m / 6m / 1y or a custom date range
- Export data to JSON or CSV
- Git-based backup (GitHub, GitLab, Codeberg, Gitea, or any SSH remote)

## Requirements

- [Go](https://go.dev/dl/) 1.21+

No CGO. No system libraries. The SQLite driver (`modernc.org/sqlite`) is pure Go.

## Install

```bash
git clone https://codeberg.org/gavasc/tuidger.git
cd tuidger
go build -o tuidger .
```

Or install directly:

```bash
go install github.com/gavasc/tuidger@latest
```

## Running

```bash
./tuidger
```

## Usage

### Global keys

| Key | Action |
|-----|--------|
| `1`–`6` | Switch tab |
| `←` / `→` or `h` / `l` | Cycle tabs |
| `[` / `]` | Cycle period presets |
| `p` | Open custom date range picker |
| `j` | Export JSON |
| `c` | Export CSV |
| `Ctrl+C` | Quit (triggers auto-backup) |

### Dashboard (tab 1)

| Key | Action |
|-----|--------|
| `e` | Add expense |
| `r` | Add revenue |
| `↑` / `↓` or `k` / `j` | Scroll |

### Ledger (tab 2)

| Key | Action |
|-----|--------|
| `j` / `k` | Move cursor |
| `e` | Edit selected transaction |
| `d` | Delete selected transaction |
| `n` / `p` | Next / previous page |

### Accounts (tab 3) · Transfers (tab 4) · Installments (tab 5)

Each tab follows the same pattern: `a` to add, `d` to delete, `j`/`k` to navigate.

### Backup (tab 6)

| Key | Action |
|-----|--------|
| `e` | Edit backup config |
| `b` | Backup now |
| `r` | Restore from backup |

## Data

The database is stored at:

| OS      | Path |
|---------|------|
| Linux   | `~/.config/tuidger/tuidger.db` |
| macOS   | `~/Library/Application Support/tuidger/tuidger.db` |
| Windows | `%APPDATA%\tuidger\tuidger.db` |

Backup configuration lives alongside it as `backup.yaml`.

## Backup

Tuidger exports the full database as JSON and commits it to a git remote. SSH remotes are supported directly — no token needed if your SSH key is already configured:

**`~/.config/tuidger/backup.yaml`**
```yaml
remote_url: ssh://git@codeberg.org/youruser/yourrepo.git
```

For HTTPS remotes (GitHub, GitLab, etc.):
```yaml
provider: github   # github | gitlab | forgejo | gitea | custom
repo: youruser/yourrepo
token: ghp_...
host: ""           # leave blank to use the provider default
```

On `Ctrl+C`, tuidger attempts an auto-backup with a 10-second timeout. On first launch with an empty database and a configured remote, it auto-restores from the latest backup.

## Stack

- **UI** — [Bubble Tea](https://github.com/charmbracelet/bubbletea) + [Bubbles](https://github.com/charmbracelet/bubbles) + [Lip Gloss](https://github.com/charmbracelet/lipgloss)
- **Database** — `modernc.org/sqlite` (pure-Go SQLite, no CGO)
- **Charts** — [asciigraph](https://github.com/guptarohit/asciigraph)
