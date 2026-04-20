package backup

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gavasc/tuidger/internal/db"
	"gopkg.in/yaml.v3"
)

type BackupManager struct {
	configPath string
	repoPath   string
}

func NewBackupManager() (*BackupManager, error) {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(cfgDir, "tuidger")
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, err
	}
	return &BackupManager{
		configPath: filepath.Join(dir, "backup.yaml"),
		repoPath:   filepath.Join(dir, "backup-repo"),
	}, nil
}

func (bm *BackupManager) LoadConfig() (db.BackupConfig, error) {
	var cfg db.BackupConfig
	data, err := os.ReadFile(bm.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	err = yaml.Unmarshal(data, &cfg)
	return cfg, err
}

func (bm *BackupManager) SaveConfig(cfg db.BackupConfig) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(bm.configPath, data, 0600)
}

func buildRemoteURL(cfg db.BackupConfig) string {
	if cfg.RemoteURL != "" {
		return cfg.RemoteURL
	}
	host := cfg.Host
	if host == "" {
		switch cfg.Provider {
		case "github":
			host = "github.com"
		case "gitlab":
			host = "gitlab.com"
		case "forgejo", "gitea":
			host = "codeberg.org"
		}
	}
	return fmt.Sprintf("https://%s@%s/%s.git", cfg.Token, host, cfg.Repo)
}

func (bm *BackupManager) BackupNow(jsonData string) error {
	cfg, err := bm.LoadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if cfg.RemoteURL == "" && (cfg.Token == "" || cfg.Repo == "") {
		return fmt.Errorf("backup not configured")
	}
	remoteURL := buildRemoteURL(cfg)

	// init repo if not exists
	if _, err := os.Stat(filepath.Join(bm.repoPath, ".git")); os.IsNotExist(err) {
		if err := bm.git("init", bm.repoPath); err != nil {
			return fmt.Errorf("git init: %w", err)
		}
	}

	dataFile := filepath.Join(bm.repoPath, "tuidger.json")
	if err := os.WriteFile(dataFile, []byte(jsonData), 0600); err != nil {
		return fmt.Errorf("write data file: %w", err)
	}

	cmds := [][]string{
		{"git", "-C", bm.repoPath, "add", "tuidger.json"},
		{"git", "-C", bm.repoPath, "commit", "--allow-empty", "-m",
			"backup: " + time.Now().Format("2006-01-02 15:04:05")},
		{"git", "-C", bm.repoPath, "push", remoteURL, "HEAD:main", "--force"},
	}
	for _, args := range cmds {
		if err := bm.runGit(args...); err != nil {
			return err
		}
	}
	return nil
}

func (bm *BackupManager) FetchBackup() (string, error) {
	cfg, err := bm.LoadConfig()
	if err != nil {
		return "", fmt.Errorf("load config: %w", err)
	}
	if cfg.RemoteURL == "" && (cfg.Token == "" || cfg.Repo == "") {
		return "", fmt.Errorf("backup not configured")
	}
	remoteURL := buildRemoteURL(cfg)

	if _, err := os.Stat(filepath.Join(bm.repoPath, ".git")); os.IsNotExist(err) {
		if err := bm.runGit("git", "clone", remoteURL, bm.repoPath); err != nil {
			return "", fmt.Errorf("git clone: %w", err)
		}
	} else {
		if err := bm.runGit("git", "-C", bm.repoPath, "fetch", remoteURL); err != nil {
			return "", fmt.Errorf("git fetch: %w", err)
		}
		if err := bm.runGit("git", "-C", bm.repoPath, "checkout", "FETCH_HEAD", "--", "tuidger.json"); err != nil {
			return "", fmt.Errorf("git checkout: %w", err)
		}
	}

	data, err := os.ReadFile(filepath.Join(bm.repoPath, "tuidger.json"))
	if err != nil {
		return "", fmt.Errorf("read backup: %w", err)
	}
	return string(data), nil
}

func AutoRestoreIfNeeded(d *db.DB, bm *BackupManager) (bool, error) {
	if !d.IsEmpty() {
		return false, nil
	}
	cfg, err := bm.LoadConfig()
	if err != nil || cfg.Token == "" {
		return false, nil
	}
	jsonData, err := bm.FetchBackup()
	if err != nil {
		return false, err
	}
	if err := d.ImportJSON(jsonData); err != nil {
		return false, err
	}
	return true, nil
}

func AutoBackupWithTimeout(d *db.DB, bm *BackupManager) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		jsonData, err := d.ExportJSON()
		if err != nil {
			done <- err
			return
		}
		done <- bm.BackupNow(jsonData)
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return fmt.Errorf("backup timeout")
	}
}

func (bm *BackupManager) git(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (bm *BackupManager) runGit(args ...string) error {
	// mask token in error messages
	cmd := exec.Command(args[0], args[1:]...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := string(out)
		// strip token from error output
		cfg, _ := bm.LoadConfig()
		if cfg.Token != "" {
			msg = strings.ReplaceAll(msg, cfg.Token, "****")
		}
		return fmt.Errorf("%s: %s", args[0], msg)
	}
	return nil
}
