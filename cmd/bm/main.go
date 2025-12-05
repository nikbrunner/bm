package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nikbrunner/bm/internal/storage"
	"github.com/nikbrunner/bm/internal/tui"
)

func main() {
	// Get config path
	configPath, err := storage.DefaultConfigPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting config path: %v\n", err)
		os.Exit(1)
	}

	// Load store
	jsonStorage := storage.NewJSONStorage(configPath)
	data, err := jsonStorage.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading bookmarks: %v\n", err)
		os.Exit(1)
	}

	// Create and run app
	app := tui.NewApp(tui.AppParams{Store: data})

	p := tea.NewProgram(app, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running app: %v\n", err)
		os.Exit(1)
	}

	// Save changes on exit
	finalApp := finalModel.(tui.App)
	if err := jsonStorage.Save(finalApp.Store()); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving bookmarks: %v\n", err)
		os.Exit(1)
	}
}
