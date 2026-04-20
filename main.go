package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gavasc/tuidger/internal/backup"
	"github.com/gavasc/tuidger/internal/db"
	"github.com/gavasc/tuidger/internal/views"
)

func main() {
	dbPath := resolveDBPath()

	d, err := db.Open(dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer d.Close()

	bm, err := backup.NewBackupManager()
	if err != nil {
		log.Fatalf("backup manager: %v", err)
	}

	if restored, err := backup.AutoRestoreIfNeeded(d, bm); err != nil {
		fmt.Fprintf(os.Stderr, "auto-restore warning: %v\n", err)
	} else if restored {
		fmt.Println("Auto-restored from backup")
	}

	root := views.NewRootModel(d, bm)
	p := tea.NewProgram(root, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}

func resolveDBPath() string {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		cfgDir = os.Getenv("HOME") + "/.config"
	}
	dir := filepath.Join(cfgDir, "tuidger")
	os.MkdirAll(dir, 0750)
	return filepath.Join(dir, "tuidger.db")
}
