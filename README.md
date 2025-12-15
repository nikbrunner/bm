# bm

A vim-style TUI bookmark manager for the terminal. Ranger/yazi-inspired with Miller columns, keyboard-driven navigation, and SQLite storage.

## Features

- **Miller columns layout** - Parent | Current | Preview panes
- **Pinned items** - Quick access to frequently used bookmarks/folders with `m` key
- **Vim keybindings** - `j/k`, `h/l`, `gg/G`, `y`, `d`, `x`, `p`
- **Fuzzy search** - Global search across all bookmarks
- **Browser integration** - Import/export standard HTML bookmark format
- **SQLite storage** - Fast and reliable local database

## Installation

### From source (requires Go 1.21+)

```bash
go install github.com/nikbrunner/bm/cmd/bm@latest
```

Or clone and build:

```bash
git clone https://github.com/nikbrunner/bm.git
cd bm
go install ./cmd/bm
```

Make sure `$HOME/go/bin` is in your PATH:

```bash
export PATH=$HOME/go/bin:$PATH
```

## Usage

### Interactive TUI

```bash
bm                        # Open full TUI
```

### Quick Search

```bash
bm <query>                # Fuzzy search → select → open in browser
bm github                 # Search for "github"
bm react router           # Search for "react router"
```

### Import/Export

```bash
bm import bookmarks.html              # Import from browser export
bm export                             # Export to ~/Downloads/bookmarks-export-YYYY-MM-DD.html
bm export ~/backup/bookmarks.html     # Export to custom path
```

### AI Features

If you set the `ANTHROPIC_API_KEY` environment variable, bm can use Claude to automatically generate titles and suggest tags for bookmarks:

```bash
export ANTHROPIC_API_KEY=sk-ant-...
bm add                                # AI analyzes URL and suggests title/tags
```

In the TUI, press `i` for AI quick add or `L` to add from clipboard to Read Later with AI analysis.

## Keybindings

### Navigation

| Key | Action |
|-----|--------|
| `j/k` | Move down/up |
| `h/l` | Navigate out/into folder (h at root → pinned pane) |
| `gg` | Jump to top |
| `G` | Jump to bottom |

### Actions

| Key | Action |
|-----|--------|
| `l` / `Enter` | Open bookmark in browser / enter folder |
| `s` | Global fuzzy search |
| `/` | Filter current folder |
| `o` | Cycle sort mode (manual → A-Z → created → visited) |
| `Y` | Copy URL to clipboard |
| `*` | Pin/unpin item (★ shown for pinned) |
| `c` | Toggle delete confirmations |

### Editing

| Key | Action |
|-----|--------|
| `a` | Add bookmark |
| `A` | Add folder |
| `i` | AI quick add (requires ANTHROPIC_API_KEY) |
| `L` | Quick add to Read Later (from clipboard) |
| `e` | Edit selected item |
| `t` | Edit tags (with autocomplete) |
| `y` | Yank (copy to buffer) |
| `d` | Delete |
| `x` | Cut (delete + copy to buffer) |
| `p/P` | Paste after/before |
| `m` | Move to different folder |

### Other

| Key | Action |
|-----|--------|
| `?` | Toggle help overlay |
| `q` / `Esc` | Quit (or close modal) |

## Data Storage

Bookmarks are stored in a SQLite database at `~/.config/bm/bookmarks.db`.

Settings are stored in `~/.config/bm/config.json`.

## Development

### Building & Testing

```bash
make build          # Build the binary
make test           # Run all tests (including visual snapshots)
make check          # Run fmt, lint, and test
make help           # Show all available targets
```

### Visual Snapshot Tests

The TUI has visual snapshot tests that capture rendered output as golden files. These tests verify both layout and interaction behavior.

```bash
make test                    # Runs all tests including snapshots
make test-update-golden      # Update golden files after intentional UI changes
```

Golden files are stored in `internal/tui/testdata/golden/` and are human-readable (ANSI codes stripped).

**When to update golden files:**
- After intentionally changing the UI layout or styling
- After adding new UI elements or modals
- After changing keybind displays or status bar content

## License

MIT
