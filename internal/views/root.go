package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gavasc/tuidger/internal/backup"
	"github.com/gavasc/tuidger/internal/components"
	"github.com/gavasc/tuidger/internal/db"
	"github.com/gavasc/tuidger/internal/styles"
)

const statusDuration = 3

var tabNames = []string{"dashboard", "ledger", "accounts", "transfers", "installments", "backup"}

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
	backupTab    BackupTabModel
	export       ExportModel // not a tab — lives in the bottom bar
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
		contentH := msg.Height - 3 // tab bar + period + bottom bar
		m.dashboard.SetSize(msg.Width, contentH)
		m.ledger.SetSize(msg.Width, contentH)
		m.accounts.SetSize(msg.Width, contentH)
		m.transfers.SetSize(msg.Width, contentH)
		m.installments.SetSize(msg.Width, contentH)
		m.backupTab.SetSize(msg.Width, contentH)
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
		m.installments, _ = m.installments.Update(msg)
		m.ledger, _ = m.ledger.Update(msg)
		return m, nil

	case TransfersLoadedMsg:
		m.transfers.OnTransfersLoaded(msg)
		return m, nil

	case InstallmentsLoadedMsg:
		m.installments.OnInstallmentsLoaded(msg)
		return m, nil

	case PeriodChangedMsg:
		m.period.From = msg.From
		m.period.To = msg.To
		m.loading = true
		return m, loadTransactions(m.db, msg.From, msg.To)

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
		var cmd tea.Cmd
		m.export, cmd = m.export.OnExportDone(msg)
		if msg.Err != nil {
			m.statusMsg = "Export failed: " + msg.Err.Error()
			m.statusErr = true
		} else {
			m.statusMsg = "Exported to " + msg.Path
		}
		m.statusTicks = statusDuration
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

		// Export modal captures all keys when active
		if m.export.Capturing() {
			var cmd tea.Cmd
			m.export, cmd = m.export.Update(msg)
			return m, cmd
		}

		// Period modal captures all keys when active (digits 1-6 must reach the
		// date inputs, not switch tabs)
		if m.period.ModalActive() {
			var cmd tea.Cmd
			m.period, cmd = m.period.Update(msg)
			return m, cmd
		}

		// Tab switching + export trigger when nothing else captures
		if !m.activeTabCaptures() {
			switched := true
			switch msg.String() {
			case "1":
				m.activeTab = 0
			case "2":
				m.activeTab = 1
			case "3":
				m.activeTab = 2
			case "4":
				m.activeTab = 3
			case "5":
				m.activeTab = 4
			case "6":
				m.activeTab = 5
			case "left", "h":
				if m.activeTab > 0 {
					m.activeTab--
				}
			case "right", "l":
				if m.activeTab < len(tabNames)-1 {
					m.activeTab++
				}
			// Export shortcuts live in the bottom bar
			case "j":
				var cmd tea.Cmd
				m.export, cmd = m.export.Update(msg)
				return m, cmd
			case "c":
				var cmd tea.Cmd
				m.export, cmd = m.export.Update(msg)
				return m, cmd
			default:
				switched = false
			}
			if switched {
				return m, nil
			}

			// Forward remaining keys to period selector
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
		m.backupTab, cmd = m.backupTab.Update(msg)
	}
	return m, cmd
}

func (m *RootModel) activeTabCaptures() bool {
	switch m.activeTab {
	case 0:
		return m.dashboard.Capturing()
	case 1:
		return m.ledger.Capturing()
	case 2:
		return m.accounts.Capturing()
	case 3:
		return m.transfers.Capturing()
	case 4:
		return m.installments.Capturing()
	case 5:
		return m.backupTab.Capturing()
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

func (m RootModel) View() string {
	if m.width == 0 {
		return "Loading…"
	}

	tabBar := m.renderTabBar()
	periodBar := m.period.View()

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
		content = m.backupTab.View()
	}

	bottom := m.renderBottom()
	base := strings.Join([]string{tabBar, periodBar, content, bottom}, "\n")

	// Root-level modal overlays (cover full screen)
	if m.period.ModalActive() {
		return components.RenderModal(m.period.ModalView(), m.width, m.height)
	}
	if m.export.Capturing() {
		return components.RenderModal(m.export.ModalView(), m.width, m.height)
	}
	return base
}

func (m RootModel) renderTabBar() string {
	var parts []string
	for i, name := range tabNames {
		label := fmt.Sprintf("[%d] %s", i+1, name)
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

func (m RootModel) activeHints() string {
	switch m.activeTab {
	case 0:
		return m.dashboard.Hints()
	case 1:
		return m.ledger.Hints()
	case 2:
		return m.accounts.Hints()
	case 3:
		return m.transfers.Hints()
	case 4:
		return m.installments.Hints()
	case 5:
		return m.backupTab.Hints()
	}
	return ""
}

func (m RootModel) renderBottom() string {
	sep := styles.Faint.Render("  |  ")
	nav := styles.Faint.Render("←/→ tabs  ↑/↓ navigate  Ctrl+C quit")
	exportHints := styles.Faint.Render("[j] export JSON  [c] export CSV")

	left := styles.Faint.Render(m.activeHints())
	if m.statusMsg != "" {
		if m.statusErr {
			left = lipgloss.NewStyle().Foreground(styles.ColorError).Render("✗ " + m.statusMsg)
		} else {
			left = lipgloss.NewStyle().Foreground(styles.ColorSuccess).Render("✓ " + m.statusMsg)
		}
	}

	return left + sep + exportHints + sep + nav
}

func (m *RootModel) PeriodFrom() string { return m.period.From }
func (m *RootModel) PeriodTo() string   { return m.period.To }
