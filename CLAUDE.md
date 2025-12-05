# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`bm` is a vim-style TUI bookmark manager built with Go and the Charm stack (bubbletea, lipgloss, bubbles). It features Miller column navigation (ranger/yazi-inspired), fuzzy search, and browser-compatible HTML import/export.

## Build & Development Commands

```bash
# Build
go build -o bm ./cmd/bm

# Install to $GOPATH/bin
go install ./cmd/bm

# Run tests
go test ./...

# Run a single test
go test ./internal/storage -run TestJSONStorage

# Format code
go fmt ./...

# Run the app
./bm                          # Full TUI
./bm <query>                  # Quick search
./bm import bookmarks.html    # Import
./bm export                   # Export
```

## Architecture

### Package Structure

```
cmd/bm/main.go          # CLI entry point with subcommands (import/export/search/TUI)
internal/
  model/                # Core data types (Bookmark, Folder, Store)
  storage/              # JSON file persistence (~/.config/bm/bookmarks.json)
  tui/                  # Bubbletea TUI (App model, View, Styles, Keys)
  search/               # Fuzzy search for CLI quick-search mode
  picker/               # Simple TUI picker for CLI search results
  importer/             # HTML bookmark parser (browser format)
  exporter/             # HTML bookmark generator (browser format)
```

### Key Design Decisions

**Store Pattern**: `model.Store` holds flat slices of `Folders` and `Bookmarks`. Parent/folder relationships are via `ParentID`/`FolderID` pointer fields (`nil` = root level).

**TUI App Structure**: `tui.App` is the main bubbletea model with:
- Modal modes: `ModeNormal`, `ModeAddBookmark`, `ModeEditFolder`, `ModeSearch`, `ModeHelp`, etc.
- Vim-style keys: `gg` (top), `y` (yank), `d` (delete), `x` (cut) - single key actions except `gg`
- Fuzzy search over all items (not just current folder) via `allItems`/`fuzzyMatches`
- View renders 3-pane Miller columns (parent | current | preview)

**Item Union Type**: `tui.Item` wraps either a `Folder` or `Bookmark` for unified list handling.

**CLI Modes**: Based on first arg: `import`/`export` are subcommands, anything else is a fuzzy search query, no args opens full TUI.

### Dependencies

- `charmbracelet/bubbletea` - TUI framework
- `charmbracelet/lipgloss` - Styling
- `charmbracelet/bubbles` - Text inputs, key bindings
- `sahilm/fuzzy` - Fuzzy matching
- `atotto/clipboard` - System clipboard
- `google/uuid` - UUID generation

## Data Storage

Bookmarks stored at `~/.config/bm/bookmarks.json` as flat JSON with `folders[]` and `bookmarks[]` arrays. IDs are UUIDs. The file is designed to be human-readable and version-controllable.
