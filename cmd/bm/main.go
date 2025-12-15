package main

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/nikbrunner/bm/internal/ai"
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
		case "add":
			runAdd(os.Args[2:])
			return
		case "init":
			runInit()
			return
		case "reset":
			runReset()
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
  bm add                Quick add URL from clipboard to Read Later
  bm init               Create config with sample data
  bm reset              Clear all data (requires confirmation)
  bm import <file>      Import bookmarks from HTML
  bm export [path]      Export bookmarks to HTML
  bm help               Show this help

Quick Add Options:
  bm add                Read URL from clipboard
  bm add --url URL      Use specified URL
  bm add --title TITLE  Override AI-generated title

TUI Keybindings:
  Navigation:
    j/k         Move down/up
    h/l         Navigate back/forward (l opens bookmarks)
    gg/G        Jump to top/bottom

  Actions:
    l/Enter     Open bookmark / enter folder
    s           Global fuzzy search
    /           Filter current folder
    o           Cycle sort mode
    Y           Copy URL to clipboard
    *           Pin/unpin item
    c           Toggle delete confirmations

  Editing:
    a/A         Add bookmark/folder
    i           AI quick add (requires ANTHROPIC_API_KEY)
    L           Quick add to Read Later
    e           Edit selected item
    t           Edit tags
    m           Move to folder
    y           Yank (copy)
    d           Delete
    x           Cut (delete + buffer)
    p/P         Paste after/before

  Other:
    ?           Show help overlay
    q           Quit

Data Storage:
  ~/.config/bm/bookmarks.db     SQLite database
  ~/.config/bm/config.json      Settings (quick add folder, etc.)
`
	fmt.Print(help)
}

// runTUI runs the full interactive TUI.
func runTUI() {
	configPath, err := storage.DefaultConfigFilePath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting config path: %v\n", err)
		os.Exit(1)
	}

	store, dataStorage, closeStorage := loadStorage()

	config, err := storage.LoadConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	app := tui.NewApp(tui.AppParams{Store: store, Config: config})
	p := tea.NewProgram(app, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		closeStorage()
		fmt.Fprintf(os.Stderr, "Error running app: %v\n", err)
		os.Exit(1)
	}

	finalApp := finalModel.(tui.App)
	if err := dataStorage.Save(finalApp.Store()); err != nil {
		closeStorage()
		fmt.Fprintf(os.Stderr, "Error saving bookmarks: %v\n", err)
		os.Exit(1)
	}
	closeStorage()
}

// runQuickSearch performs a fuzzy search and opens the selected bookmark.
func runQuickSearch(query string) {
	store, dataStorage, closeStorage := loadStorage()
	defer closeStorage()

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
		if err := dataStorage.Save(store); err != nil {
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
	store, dataStorage, closeStorage := loadStorage()
	defer closeStorage()

	file, err := os.Open(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening file: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = file.Close() }()

	folders, bookmarks, err := importer.ParseHTMLBookmarks(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing HTML: %v\n", err)
		os.Exit(1)
	}

	added, skipped := store.ImportMerge(folders, bookmarks)

	if err := dataStorage.Save(store); err != nil {
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
	store, _, closeStorage := loadStorage()
	defer closeStorage()

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

// runAdd handles the quick add command.
func runAdd(args []string) {
	// Parse flags
	var urlFlag, titleFlag string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--url":
			if i+1 < len(args) {
				urlFlag = args[i+1]
				i++
			}
		case "--title":
			if i+1 < len(args) {
				titleFlag = args[i+1]
				i++
			}
		}
	}

	// Get URL from flag or clipboard
	bookmarkURL := urlFlag
	if bookmarkURL == "" {
		var err error
		bookmarkURL, err = clipboard.ReadAll()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading clipboard: %v\n", err)
			os.Exit(1)
		}
		bookmarkURL = strings.TrimSpace(bookmarkURL)
	}

	if bookmarkURL == "" {
		fmt.Fprintf(os.Stderr, "No URL found in clipboard\n")
		os.Exit(1)
	}

	// Validate URL
	parsedURL, err := url.Parse(bookmarkURL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		fmt.Fprintf(os.Stderr, "Invalid URL: %s\n", bookmarkURL)
		os.Exit(1)
	}

	// Load config
	configFilePath, err := storage.DefaultConfigFilePath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting config path: %v\n", err)
		os.Exit(1)
	}

	config, err := storage.LoadConfig(configFilePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Load store
	store, dataStorage, closeStorage := loadStorage()
	defer closeStorage()

	// Find or create the quick add folder
	folderID := findOrCreateFolder(store, config.QuickAddFolder)

	// Determine title and tags
	var title string
	var tags []string

	if titleFlag != "" {
		// Use provided title
		title = titleFlag
	} else {
		// Try to use AI
		aiClient, err := ai.NewClient()
		if err != nil {
			// AI unavailable - use URL as title
			fmt.Printf("AI unavailable (%v) - using URL as title\n", err)
			title = bookmarkURL
		} else {
			// Build context for AI
			context := ai.BuildContext(store)
			response, err := aiClient.SuggestBookmark(bookmarkURL, context)
			if err != nil {
				fmt.Printf("AI request failed (%v) - using URL as title\n", err)
				title = bookmarkURL
			} else {
				title = response.Title
				tags = response.Tags
			}
		}
	}

	// Create bookmark
	newBookmark := model.NewBookmark(model.NewBookmarkParams{
		Title:    title,
		URL:      bookmarkURL,
		FolderID: &folderID,
		Tags:     tags,
	})

	store.AddBookmark(newBookmark)

	// Save
	if err := dataStorage.Save(store); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving bookmarks: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Added to %s: %s\n", config.QuickAddFolder, title)
}

// findOrCreateFolder finds a folder by name or creates it at root level.
func findOrCreateFolder(store *model.Store, name string) string {
	// Look for existing folder at root level
	for _, f := range store.Folders {
		if f.Name == name && f.ParentID == nil {
			return f.ID
		}
	}

	// Create new folder
	newFolder := model.NewFolder(model.NewFolderParams{
		Name:     name,
		ParentID: nil,
	})
	store.AddFolder(newFolder)
	return newFolder.ID
}

// runInit creates the config and data files with sample data.
func runInit() {
	dataPath, err := storage.DefaultSQLitePath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting data path: %v\n", err)
		os.Exit(1)
	}

	configPath, err := storage.DefaultConfigFilePath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting config path: %v\n", err)
		os.Exit(1)
	}

	// Check if data file already exists
	if _, err := os.Stat(dataPath); err == nil {
		fmt.Fprintf(os.Stderr, "Data file already exists: %s\n", dataPath)
		fmt.Fprintf(os.Stderr, "Use 'bm reset' to clear existing data\n")
		os.Exit(1)
	}

	// Create sample data
	now := time.Now()
	devFolderID := "dev-folder"
	toolsFolderID := "tools-folder"
	readLaterFolderID := "read-later-folder"

	store := &model.Store{
		Folders: []model.Folder{
			{
				ID:       devFolderID,
				Name:     "Development",
				ParentID: nil,
			},
			{
				ID:       toolsFolderID,
				Name:     "Tools",
				ParentID: nil,
			},
			{
				ID:       readLaterFolderID,
				Name:     "Read Later",
				ParentID: nil,
			},
		},
		Bookmarks: []model.Bookmark{
			{
				ID:        "bm-github",
				Title:     "GitHub",
				URL:       "https://github.com",
				FolderID:  &devFolderID,
				Tags:      []string{"code", "git"},
				CreatedAt: now,
			},
			{
				ID:        "bm-go",
				Title:     "Go Documentation",
				URL:       "https://go.dev/doc",
				FolderID:  &devFolderID,
				Tags:      []string{"go", "docs"},
				CreatedAt: now,
			},
			{
				ID:        "bm-charm",
				Title:     "Charm - TUI Libraries",
				URL:       "https://charm.sh",
				FolderID:  &toolsFolderID,
				Tags:      []string{"tui", "go"},
				CreatedAt: now,
			},
			{
				ID:        "bm-hn",
				Title:     "Hacker News",
				URL:       "https://news.ycombinator.com",
				FolderID:  nil,
				Tags:      []string{"news", "tech"},
				CreatedAt: now,
			},
		},
	}

	// Save data file
	sqliteStorage, err := storage.NewSQLiteStorage(dataPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating database: %v\n", err)
		os.Exit(1)
	}
	defer sqliteStorage.Close()

	if err := sqliteStorage.Save(store); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving data: %v\n", err)
		os.Exit(1)
	}

	// Save config file
	config := storage.DefaultConfig()
	if err := storage.SaveConfig(configPath, &config); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Created:\n")
	fmt.Printf("  %s (%d folders, %d bookmarks)\n", dataPath, len(store.Folders), len(store.Bookmarks))
	fmt.Printf("  %s (quick add folder: %s)\n", configPath, config.QuickAddFolder)
	fmt.Println("\nRun 'bm' to open the TUI")
}

// runReset clears all bookmarks and folders.
func runReset() {
	_, dataStorage, closeStorage := loadStorage()
	defer closeStorage()

	// Confirm reset
	fmt.Print("This will delete all bookmarks and folders.\n\n")
	fmt.Print("Type 'yes' to confirm: ")
	var confirm string
	_, _ = fmt.Scanln(&confirm)
	if confirm != "yes" {
		fmt.Println("Aborted")
		os.Exit(0)
	}

	// Save empty store
	emptyStore := &model.Store{
		Folders:   []model.Folder{},
		Bookmarks: []model.Bookmark{},
	}

	if err := dataStorage.Save(emptyStore); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("All data cleared")
}

// loadStorage opens the appropriate storage backend and returns it with a cleanup function.
func loadStorage() (*model.Store, storage.Storage, func()) {
	dataStorage, err := storage.OpenStorage()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening storage: %v\n", err)
		os.Exit(1)
	}

	store, err := dataStorage.Load()
	if err != nil {
		if closer, ok := dataStorage.(interface{ Close() error }); ok {
			closer.Close()
		}
		fmt.Fprintf(os.Stderr, "Error loading bookmarks: %v\n", err)
		os.Exit(1)
	}

	closeFunc := func() {
		if closer, ok := dataStorage.(interface{ Close() error }); ok {
			closer.Close()
		}
	}

	return store, dataStorage, closeFunc
}

