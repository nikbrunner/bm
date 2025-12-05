package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nikbrunner/bm/internal/importer"
	"github.com/nikbrunner/bm/internal/storage"
	"github.com/nikbrunner/bm/internal/tui"
)

func main() {
	// Handle import subcommand: bm import <file.html>
	if len(os.Args) >= 3 && os.Args[1] == "import" {
		runImport(os.Args[2])
		return
	}

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

// runImport handles the import subcommand.
func runImport(filePath string) {
	// Get config path
	configPath, err := storage.DefaultConfigPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting config path: %v\n", err)
		os.Exit(1)
	}

	// Load existing store
	jsonStorage := storage.NewJSONStorage(configPath)
	store, err := jsonStorage.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading bookmarks: %v\n", err)
		os.Exit(1)
	}

	// Open and parse HTML file
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	folders, bookmarks, err := importer.ParseHTMLBookmarks(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing HTML: %v\n", err)
		os.Exit(1)
	}

	// Import and merge
	added, skipped := store.ImportMerge(folders, bookmarks)

	// Save
	if err := jsonStorage.Save(store); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving bookmarks: %v\n", err)
		os.Exit(1)
	}

	// Print summary
	fmt.Printf("Imported %d bookmarks, %d folders", added, len(folders))
	if skipped > 0 {
		fmt.Printf(" (%d duplicates skipped)", skipped)
	}
	fmt.Println()
}
