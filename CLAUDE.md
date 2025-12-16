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
go test ./internal/storage -run TestSQLiteStorage

# Format code
go fmt ./...

# Run the app
./bm                          # Full TUI
./bm <query>                  # Quick search
./bm help                     # Show help
./bm init                     # Create config with sample data
./bm reset                    # Clear all data (requires confirmation)
./bm import bookmarks.html    # Import from browser HTML
./bm export                   # Export to browser HTML
./bm cull                     # Check all URLs for dead links (report only)
```

## Architecture

### Package Structure

```
cmd/bm/main.go          # CLI entry point with subcommands (import/export/search/TUI)
internal/
  ai/                   # Claude API for AI-powered bookmark analysis
  culler/               # URL health checker for dead link detection
  model/                # Core data types (Bookmark, Folder, Store)
  storage/              # SQLite persistence (~/.config/bm/bookmarks.db)
  tui/                  # Bubbletea TUI (App model, View, Styles, Keys)
  search/               # Fuzzy search for CLI quick-search mode
  picker/               # Simple TUI picker for CLI search results
  importer/             # HTML bookmark parser (browser format)
  exporter/             # HTML bookmark generator (browser format)
```

### Key Design Decisions

**Store Pattern**: `model.Store` holds flat slices of `Folders` and `Bookmarks`. Parent/folder relationships are via `ParentID`/`FolderID` pointer fields (`nil` = root level).

**TUI App Structure**: `tui.App` is the main bubbletea model with:
- Modal modes: `ModeNormal`, `ModeAddBookmark`, `ModeEditFolder`, `ModeSearch`, `ModeHelp`, `ModeConfirmDelete`, `ModeQuickAdd`, `ModeQuickAddLoading`, `ModeQuickAddConfirm`, `ModeMove`, `ModeReadLaterLoading`, `ModeCullMenu`, `ModeCullLoading`, `ModeCullResults`, `ModeCullInspect`
- Focus states: `PanePinned` (leftmost pinned items pane) and `PaneBrowser` (Miller columns)
- Fuzzy search over all items (not just current folder) via `allItems`/`fuzzyMatches`
- View renders 3-pane Miller columns (parent | current | preview), or 4-pane when pinned items exist in subfolders
- Pinned items shown in leftmost pane with `★` prefix; `*` toggles pin/unpin
- `confirmDelete` flag (toggled with `c`) controls whether delete/cut shows confirmation
- Searchable folder picker with smart ordering for quick add and move operations

**Item Union Type**: `tui.Item` wraps either a `Folder` or `Bookmark` for unified list handling.

**CLI Modes**: Subcommands are `help`, `init`, `reset`, `add`, `import`, `export`, `cull`. Anything else is treated as a fuzzy search query. No args opens the full TUI.

**Keybindings**: Single-key actions for editing (`y` yank, `d` delete, `x` cut), `gg` for top. `*` toggles pin/unpin on items. `m` moves items to a different folder. `h` at root switches to pinned pane, `l` returns to browser. `c` toggles delete confirmations (on by default). `?` shows help overlay. `l`/`Enter` opens bookmarks or enters folders. `i` triggers AI quick add, `L` adds to Read Later. `C` opens dead link cull mode.

### Dependencies

- `charmbracelet/bubbletea` - TUI framework
- `charmbracelet/lipgloss` - Styling
- `charmbracelet/bubbles` - Text inputs, key bindings
- `modernc.org/sqlite` - Pure Go SQLite database
- `sahilm/fuzzy` - Fuzzy matching
- `atotto/clipboard` - System clipboard
- `google/uuid` - UUID generation

### AI Integration

The `internal/ai/` package provides Claude API integration for bookmark analysis:
- Uses `claude-haiku-4-5` model for fast, cost-effective suggestions
- `SuggestBookmark(url, context)` returns title and tag suggestions
- `BuildContext(store)` creates context from existing folders/tags for smarter suggestions
- Requires `ANTHROPIC_API_KEY` environment variable

## Data Storage

Bookmarks stored in SQLite database at `~/.config/bm/bookmarks.db`. Schema includes `folders` and `bookmarks` tables with UUID primary keys. Settings stored in `~/.config/bm/config.json`. Cull results cached in `~/.config/bm/cull-cache.json`.
