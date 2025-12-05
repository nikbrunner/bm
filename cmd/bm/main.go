package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nikbrunner/bm/internal/exporter"
	"github.com/nikbrunner/bm/internal/importer"
	"github.com/nikbrunner/bm/internal/model"
	"github.com/nikbrunner/bm/internal/picker"
	"github.com/nikbrunner/bm/internal/search"
	"github.com/nikbrunner/bm/internal/storage"
	"github.com/nikbrunner/bm/internal/tui"
)

func main() {
	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case "help", "--help", "-h":
			printHelp()
			return
		case "import":
			if len(os.Args) < 3 {
				fmt.Fprintf(os.Stderr, "Usage: bm import <file.html>\n")
				os.Exit(1)
			}
			runImport(os.Args[2])
			return
		case "export":
			// Export with optional path
			var outputPath string
			if len(os.Args) >= 3 {
				outputPath = os.Args[2]
			}
			runExport(outputPath)
			return
		default:
			// Treat as search query (join all remaining args)
			query := strings.Join(os.Args[1:], " ")
			runQuickSearch(query)
			return
		}
	}

	// No args - run full TUI
	runTUI()
}

func printHelp() {
	help := `bm - vim-style bookmark manager

Usage:
  bm                    Open interactive TUI
  bm <query>            Quick search → select → open
  bm import <file>      Import bookmarks from HTML
  bm export [path]      Export bookmarks to HTML
  bm help               Show this help

TUI Keybindings:
  Navigation:
    j/k         Move down/up
    h/l         Navigate back/forward
    gg/G        Jump to top/bottom

  Actions:
    l/Enter     Open bookmark / enter folder
    o           Open bookmark in browser
    Y           Copy URL to clipboard
    /           Global fuzzy search
    s           Cycle sort mode
    c           Toggle delete confirmations

  Editing:
    a/A         Add bookmark/folder
    e           Edit selected item
    t           Edit tags (bookmarks only)
    y           Yank (copy)
    d           Delete
    x           Cut (delete + buffer)
    p/P         Paste after/before

  Other:
    ?           Show help overlay
    q           Quit

Data Storage:
  ~/.config/bm/bookmarks.json
`
	fmt.Print(help)
}

// runTUI runs the full interactive TUI.
func runTUI() {
	configPath, err := storage.DefaultConfigPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting config path: %v\n", err)
		os.Exit(1)
	}

	jsonStorage := storage.NewJSONStorage(configPath)
	data, err := jsonStorage.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading bookmarks: %v\n", err)
		os.Exit(1)
	}

	app := tui.NewApp(tui.AppParams{Store: data})
	p := tea.NewProgram(app, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running app: %v\n", err)
		os.Exit(1)
	}

	finalApp := finalModel.(tui.App)
	if err := jsonStorage.Save(finalApp.Store()); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving bookmarks: %v\n", err)
		os.Exit(1)
	}
}

// runQuickSearch performs a fuzzy search and opens the selected bookmark.
func runQuickSearch(query string) {
	configPath, err := storage.DefaultConfigPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting config path: %v\n", err)
		os.Exit(1)
	}

	jsonStorage := storage.NewJSONStorage(configPath)
	store, err := jsonStorage.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading bookmarks: %v\n", err)
		os.Exit(1)
	}

	// Search
	results := search.FuzzySearchBookmarks(store, query)

	if len(results) == 0 {
		fmt.Printf("No bookmarks found for '%s'\n", query)
		os.Exit(0)
	}

	var selectedBookmark *model.Bookmark

	if len(results) == 1 {
		// Single result - select it directly
		selectedBookmark = results[0].Bookmark
		fmt.Printf("Opening: %s\n", selectedBookmark.Title)
	} else {
		// Multiple results - show picker
		p := picker.New(results, query)
		program := tea.NewProgram(p)
		finalModel, err := program.Run()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error running picker: %v\n", err)
			os.Exit(1)
		}

		finalPicker := finalModel.(picker.Picker)
		if finalPicker.Cancelled() {
			os.Exit(0)
		}
		selectedBookmark = finalPicker.SelectedBookmark()
	}

	if selectedBookmark == nil {
		os.Exit(0)
	}

	// Update visitedAt
	bookmark := store.GetBookmarkByID(selectedBookmark.ID)
	if bookmark != nil {
		now := time.Now()
		bookmark.VisitedAt = &now
		if err := jsonStorage.Save(store); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving bookmarks: %v\n", err)
		}
	}

	// Open in browser
	openURL(selectedBookmark.URL)
}

// openURL opens a URL in the default browser.
func openURL(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	}
	if cmd != nil {
		_ = cmd.Start()
	}
}

// runImport handles the import subcommand.
func runImport(filePath string) {
	configPath, err := storage.DefaultConfigPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting config path: %v\n", err)
		os.Exit(1)
	}

	jsonStorage := storage.NewJSONStorage(configPath)
	store, err := jsonStorage.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading bookmarks: %v\n", err)
		os.Exit(1)
	}

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

	added, skipped := store.ImportMerge(folders, bookmarks)

	if err := jsonStorage.Save(store); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving bookmarks: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Imported %d bookmarks, %d folders", added, len(folders))
	if skipped > 0 {
		fmt.Printf(" (%d duplicates skipped)", skipped)
	}
	fmt.Println()
}

// runExport handles the export subcommand.
func runExport(outputPath string) {
	// Determine output path
	if outputPath == "" {
		var err error
		outputPath, err = exporter.DefaultExportPath()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting default export path: %v\n", err)
			os.Exit(1)
		}
	}

	// Load store
	configPath, err := storage.DefaultConfigPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting config path: %v\n", err)
		os.Exit(1)
	}

	jsonStorage := storage.NewJSONStorage(configPath)
	store, err := jsonStorage.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading bookmarks: %v\n", err)
		os.Exit(1)
	}

	// Generate HTML
	html := exporter.ExportHTML(store)

	// Write to file
	if err := os.WriteFile(outputPath, []byte(html), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Exported %d bookmarks, %d folders to %s\n",
		len(store.Bookmarks), len(store.Folders), outputPath)
}
