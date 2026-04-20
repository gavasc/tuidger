package views

import (
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/gavasc/tuidger/internal/backup"
	"github.com/gavasc/tuidger/internal/components"
	"github.com/gavasc/tuidger/internal/db"
	"github.com/gavasc/tuidger/internal/styles"
)

type backupTabMode int

const (
	backupTabModeView backupTabMode = iota
	backupTabModeEdit
)

type BackupTabModel struct {
	d          *db.DB
	bm         *backup.BackupManager
	cfg        db.BackupConfig
	configForm components.FormModel
	mode       backupTabMode
	loading    bool
	spinner    spinner.Model
	statusLine string
	isErr      bool
	width      int
	height     int
}

func NewBackupTabModel(d *db.DB, bm *backup.BackupManager) BackupTabModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot

	f := components.NewForm("Backup Config", "Enter → Save")
	f.AddTextField("Remote URL", false)
	f.AddSelectField("Provider", []string{"github", "gitlab", "forgejo", "gitea", "custom"}, false)
	f.AddTextField("Host (leave blank for defaults)", false)
	f.AddTextField("Repo (user/repo)", false)
	f.AddTextField("Token", false)

	m := BackupTabModel{d: d, bm: bm, configForm: f, spinner: sp}
	// Load config async on creation
	if bm != nil {
		cfg, _ := bm.LoadConfig()
		m.cfg = cfg
	}
	return m
}

func (m *BackupTabModel) SetSize(w, h int) { m.width = w; m.height = h }

func (m BackupTabModel) Capturing() bool { return m.mode == backupTabModeEdit }

func (m BackupTabModel) Hints() string {
	if m.mode == backupTabModeEdit {
		return "Tab next field  Enter save  Esc cancel"
	}
	return "[e] edit config  [b] backup now  [r] restore"
}

func (m BackupTabModel) OnBackupDone(msg BackupDoneMsg) (BackupTabModel, tea.Cmd) {
	m.loading = false
	if msg.Err != nil {
		m.statusLine = "Backup failed: " + msg.Err.Error()
		m.isErr = true
	} else {
		m.statusLine = "Backup successful"
		m.isErr = false
	}
	return m, nil
}

func (m BackupTabModel) OnRestoreDone(msg RestoreDoneMsg) (BackupTabModel, tea.Cmd) {
	m.loading = false
	if msg.Err != nil {
		m.statusLine = "Restore failed: " + msg.Err.Error()
		m.isErr = true
	} else {
		m.statusLine = "Restore successful"
		m.isErr = false
	}
	return m, nil
}

func (m BackupTabModel) Update(msg tea.Msg) (BackupTabModel, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil
	}

	if m.mode == backupTabModeEdit {
		return m.updateEdit(msg)
	}

	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "e":
			m.mode = backupTabModeEdit
			m.configForm.Reset()
			// pre-fill form
			m.configForm.Fields[0].Input.SetValue(m.cfg.RemoteURL)
			for i, opt := range m.configForm.Fields[1].Options {
				if opt == m.cfg.Provider {
					m.configForm.Fields[1].SelectedIdx = i
				}
			}
			m.configForm.Fields[2].Input.SetValue(m.cfg.Host)
			m.configForm.Fields[3].Input.SetValue(m.cfg.Repo)
			m.configForm.Fields[4].Input.SetValue(m.cfg.Token)
			m.configForm.FocusFirst()
			return m, nil
		case "b":
			m.loading = true
			m.statusLine = "Backing up…"
			return m, tea.Batch(m.spinner.Tick, backupNowCmd(m.d, m.bm))
		case "r":
			m.loading = true
			m.statusLine = "Restoring…"
			return m, tea.Batch(m.spinner.Tick, restoreCmd(m.d, m.bm))
		}
	}
	return m, nil
}

func (m BackupTabModel) updateEdit(msg tea.Msg) (BackupTabModel, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			m.mode = backupTabModeView
			return m, nil
		case "enter":
			return m.submitEdit()
		}
	}
	var cmd tea.Cmd
	m.configForm, cmd = m.configForm.Update(msg)
	return m, cmd
}

func (m BackupTabModel) submitEdit() (BackupTabModel, tea.Cmd) {
	if !m.configForm.Validate() {
		return m, nil
	}
	vals := m.configForm.Values()
	cfg := db.BackupConfig{
		RemoteURL: vals["Remote URL"],
		Provider:  vals["Provider"],
		Host:      vals["Host (leave blank for defaults)"],
		Repo:      vals["Repo (user/repo)"],
		Token:     vals["Token"],
	}
	if err := m.bm.SaveConfig(cfg); err != nil {
		return m, func() tea.Msg { return StatusMsg{Text: "Save config error: " + err.Error(), IsErr: true} }
	}
	m.cfg = cfg
	m.mode = backupTabModeView
	m.statusLine = "Config saved"
	return m, nil
}

func (m BackupTabModel) View() string {
	if m.mode == backupTabModeEdit {
		return m.configForm.View()
	}

	var sb strings.Builder

	// Config display
	sb.WriteString(styles.Title.Render("Backup Configuration") + "\n")
	if m.cfg.RemoteURL != "" {
		sb.WriteString("  Remote:   " + m.cfg.RemoteURL + "\n\n")
	} else {
		provider := m.cfg.Provider
		if provider == "" {
			provider = "(not configured)"
		}
		sb.WriteString("  Provider: " + provider + "\n")
		sb.WriteString("  Repo:     " + m.cfg.Repo + "\n")
		token := m.cfg.Token
		if len(token) > 4 {
			token = token[:4] + strings.Repeat("*", len(token)-4)
		}
		sb.WriteString("  Token:    " + token + "\n\n")
	}

	if m.loading {
		sb.WriteString(m.spinner.View() + " " + m.statusLine + "\n")
	} else if m.statusLine != "" {
		if m.isErr {
			sb.WriteString(styles.Error.Render(m.statusLine) + "\n")
		} else {
			sb.WriteString(styles.Success.Render(m.statusLine) + "\n")
		}
	}
	return sb.String()
}
