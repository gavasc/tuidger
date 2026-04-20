package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gavasc/tuidger/internal/backup"
	"github.com/gavasc/tuidger/internal/db"
	"github.com/gavasc/tuidger/internal/styles"
)

const statusDuration = 3

var tabNames = []string{"dashboard", "ledger", "accounts", "transfers", "installments", "notes", "backup", "export"}

type RootModel struct {
	db           *db.DB
	bm           *backup.BackupManager
	activeTab    int
	width        int
	height       int
	period       PeriodModel
	dashboard    DashboardModel
	ledger       LedgerModel
	accounts     AccountsModel
	transfers    TransfersModel
	installments InstallmentsModel
	notes        NotesModel
	backupTab    BackupTabModel
	export       ExportModel
	statusMsg    string
	statusErr    bool
	statusTicks  int
	loading      bool
	spinner      spinner.Model
}

func NewRootModel(d *db.DB, bm *backup.BackupManager) RootModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot

	period := NewPeriodModel()

	return RootModel{
		db:           d,
		bm:           bm,
		period:       period,
		dashboard:    NewDashboardModel(d),
		ledger:       NewLedgerModel(d),
		accounts:     NewAccountsModel(d),
		transfers:    NewTransfersModel(d),
		installments: NewInstallmentsModel(d),
		notes:        NewNotesModel(d),
		backupTab:    NewBackupTabModel(d, bm),
		export:       NewExportModel(d),
		spinner:      sp,
		loading:      true,
	}
}

func (m RootModel) Init() tea.Cmd {
	return tea.Batch(
		loadTransactions(m.db, m.period.From, m.period.To),
		loadAccounts(m.db),
		loadTransfers(m.db),
		loadInstallments(m.db),
		m.spinner.Tick,
		tickCmd(),
	)
}

func (m RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		contentH := msg.Height - 3 // tab bar + period + status
		m.dashboard.SetSize(msg.Width, contentH)
		m.ledger.SetSize(msg.Width, contentH)
		m.accounts.SetSize(msg.Width, contentH)
		m.transfers.SetSize(msg.Width, contentH)
		m.installments.SetSize(msg.Width, contentH)
		m.notes.SetSize(msg.Width, contentH)
		m.backupTab.SetSize(msg.Width, contentH)
		m.export.SetSize(msg.Width, contentH)
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case TickMsg:
		if m.statusTicks > 0 {
			m.statusTicks--
			if m.statusTicks == 0 {
				m.statusMsg = ""
				m.statusErr = false
			}
		}
		return m, tickCmd()

	case StatusMsg:
		m.statusMsg = msg.Text
		m.statusErr = msg.IsErr
		m.statusTicks = statusDuration
		if !msg.IsErr {
			m.loading = false
		}
		return m, nil

	case TransactionsLoadedMsg:
		m.loading = false
		var cmd tea.Cmd
		m.dashboard, cmd = m.dashboard.OnTransactionsLoaded(msg)
		var cmd2 tea.Cmd
		m.ledger, cmd2 = m.ledger.OnTransactionsLoaded(msg)
		return m, tea.Batch(cmd, cmd2)

	case AccountsLoadedMsg:
		m.dashboard.OnAccountsLoaded(msg)
		m.accounts.OnAccountsLoaded(msg)
		m.transfers.OnAccountsLoaded(msg)
		m.installments.Update(msg)
		// pass to ledger for edit-form account dropdown
		m.ledger.Update(msg)
		return m, nil

	case TransfersLoadedMsg:
		m.transfers.OnTransfersLoaded(msg)
		return m, nil

	case InstallmentsLoadedMsg:
		m.installments.OnInstallmentsLoaded(msg)
		return m, nil

	case NoteLoadedMsg:
		m.notes.OnNoteLoaded(msg)
		return m, nil

	case PeriodChangedMsg:
		m.period.From = msg.From
		m.period.To = msg.To
		m.loading = true
		return m, tea.Batch(
			loadTransactions(m.db, msg.From, msg.To),
		)

	case InsertDoneMsg:
		if msg.Err != nil {
			m.statusMsg = msg.Err.Error()
			m.statusErr = true
			m.statusTicks = statusDuration
			return m, nil
		}
		m.statusMsg = fmt.Sprintf("%s added", msg.EntityType)
		m.statusTicks = statusDuration
		return m, m.reloadAll()

	case UpdateDoneMsg:
		if msg.Err != nil {
			m.statusMsg = msg.Err.Error()
			m.statusErr = true
			m.statusTicks = statusDuration
			return m, nil
		}
		m.statusMsg = "updated"
		m.statusTicks = statusDuration
		return m, m.reloadAll()

	case DeleteDoneMsg:
		if msg.Err != nil {
			m.statusMsg = msg.Err.Error()
			m.statusErr = true
			m.statusTicks = statusDuration
			return m, nil
		}
		m.statusMsg = "deleted"
		m.statusTicks = statusDuration
		return m, m.reloadAll()

	case BackupDoneMsg:
		if msg.Err != nil {
			m.statusMsg = "Backup failed: " + msg.Err.Error()
			m.statusErr = true
		} else {
			m.statusMsg = "Backup successful"
		}
		m.statusTicks = statusDuration
		var cmd tea.Cmd
		m.backupTab, cmd = m.backupTab.OnBackupDone(msg)
		return m, cmd

	case RestoreDoneMsg:
		if msg.Err != nil {
			m.statusMsg = "Restore failed: " + msg.Err.Error()
			m.statusErr = true
		} else {
			m.statusMsg = "Restore successful"
		}
		m.statusTicks = statusDuration
		var cmd tea.Cmd
		m.backupTab, cmd = m.backupTab.OnRestoreDone(msg)
		if msg.Err == nil {
			return m, tea.Batch(cmd, m.reloadAll())
		}
		return m, cmd

	case ExportDoneMsg:
		if msg.Err != nil {
			m.statusMsg = "Export failed: " + msg.Err.Error()
			m.statusErr = true
		} else {
			m.statusMsg = "Exported to " + msg.Path
		}
		m.statusTicks = statusDuration
		var cmd tea.Cmd
		m.export, cmd = m.export.OnExportDone(msg)
		return m, cmd

	case tea.KeyMsg:
		// Global quit
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Batch(
				func() tea.Msg {
					backup.AutoBackupWithTimeout(m.db, m.bm)
					return nil
				},
				tea.Quit,
			)
		}

		// Tab switching via number keys (only when not in a form)
		if !m.activeTabCaptures() {
			switch msg.String() {
			case "1":
				m.activeTab = 0
				return m, m.onTabSwitch()
			case "2":
				m.activeTab = 1
				return m, m.onTabSwitch()
			case "3":
				m.activeTab = 2
				return m, m.onTabSwitch()
			case "4":
				m.activeTab = 3
				return m, m.onTabSwitch()
			case "5":
				m.activeTab = 4
				return m, m.onTabSwitch()
			case "6":
				m.activeTab = 5
				return m, m.onTabSwitch()
			case "7":
				m.activeTab = 6
				return m, m.onTabSwitch()
			case "8":
				m.activeTab = 7
				return m, m.onTabSwitch()
			}
		}

		// Period selector gets tab/shift+tab keys only when no active tab captures
		if !m.activeTabCaptures() {
			var pCmd tea.Cmd
			m.period, pCmd = m.period.Update(msg)
			if pCmd != nil {
				return m, pCmd
			}
		}
	}

	// Delegate to active tab
	var cmd tea.Cmd
	switch m.activeTab {
	case 0:
		m.dashboard, cmd = m.dashboard.Update(msg)
	case 1:
		m.ledger, cmd = m.ledger.Update(msg)
	case 2:
		m.accounts, cmd = m.accounts.Update(msg)
	case 3:
		m.transfers, cmd = m.transfers.Update(msg)
	case 4:
		m.installments, cmd = m.installments.Update(msg)
	case 5:
		m.notes, cmd = m.notes.Update(msg)
	case 6:
		m.backupTab, cmd = m.backupTab.Update(msg)
	case 7:
		m.export, cmd = m.export.Update(msg)
	}
	return m, cmd
}

func (m *RootModel) activeTabCaptures() bool {
	switch m.activeTab {
	case 0:
		return m.dashboard.Capturing()
	case 1:
		return m.ledger.Capturing()
	case 5:
		return m.notes.Capturing()
	case 6:
		return m.backupTab.Capturing()
	case 7:
		return m.export.Capturing()
	}
	return false
}

func (m RootModel) reloadAll() tea.Cmd {
	return tea.Batch(
		loadTransactions(m.db, m.period.From, m.period.To),
		loadAccounts(m.db),
		loadTransfers(m.db),
		loadInstallments(m.db),
	)
}

func (m RootModel) onTabSwitch() tea.Cmd {
	switch m.activeTab {
	case 5:
		return tea.Batch(
			loadNote(m.db, "expenses", m.period.From, m.period.To),
			loadNote(m.db, "revenues", m.period.From, m.period.To),
		)
	}
	return nil
}

func (m RootModel) View() string {
	if m.width == 0 {
		return "Loading…"
	}

	// Tab bar
	tabBar := m.renderTabBar()

	// Period selector
	periodBar := m.period.View()

	// Content
	var content string
	switch m.activeTab {
	case 0:
		content = m.dashboard.View()
	case 1:
		content = m.ledger.View()
	case 2:
		content = m.accounts.View()
	case 3:
		content = m.transfers.View()
	case 4:
		content = m.installments.View()
	case 5:
		content = m.notes.View()
	case 6:
		content = m.backupTab.View()
	case 7:
		content = m.export.View()
	}

	// Status bar
	status := m.renderStatus()

	return strings.Join([]string{tabBar, periodBar, content, status}, "\n")
}

func (m RootModel) renderTabBar() string {
	var parts []string
	for i, name := range tabNames {
		num := fmt.Sprintf("%d", i+1)
		label := fmt.Sprintf("[%s] %s", num, name)
		if i == m.activeTab {
			parts = append(parts, styles.TabActive.Render(label))
		} else {
			parts = append(parts, styles.TabInactive.Render(label))
		}
	}
	bar := strings.Join(parts, " ")
	if m.loading {
		bar += "  " + m.spinner.View()
	}
	return bar
}

func (m RootModel) renderStatus() string {
	if m.statusMsg == "" {
		return styles.Faint.Render("ctrl+c quit  1-8 tabs  1m/3m/6m/1y period")
	}
	if m.statusErr {
		return lipgloss.NewStyle().Foreground(styles.ColorError).Render("✗ " + m.statusMsg)
	}
	return lipgloss.NewStyle().Foreground(styles.ColorSuccess).Render("✓ " + m.statusMsg)
}

// PeriodFrom / PeriodTo expose the current period to external callers.
func (m *RootModel) PeriodFrom() string { return m.period.From }
func (m *RootModel) PeriodTo() string   { return m.period.To }
