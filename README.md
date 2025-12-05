# bm

A vim-style TUI bookmark manager for the terminal. Ranger/yazi-inspired with Miller columns, keyboard-driven navigation, and simple JSON storage.

## Features

- **Miller columns layout** - Parent | Current | Preview panes
- **Pinned items** - Quick access to frequently used bookmarks/folders with `m` key
- **Vim keybindings** - `j/k`, `h/l`, `gg/G`, `y`, `d`, `x`, `p`
- **Fuzzy search** - Global search across all bookmarks
- **Browser integration** - Import/export standard HTML bookmark format
- **Simple storage** - Version-controllable JSON file

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
| `l` / `Enter` | Open URL in browser |
| `m` | Pin/unpin item (★ shown for pinned) |
| `Y` | Copy URL to clipboard |
| `s` | Global fuzzy search |
| `/` | Filter current folder |
| `o` | Cycle sort mode (manual → A-Z → created → visited) |

### Editing

| Key | Action |
|-----|--------|
| `a` | Add bookmark |
| `A` | Add folder |
| `i` | AI quick add (requires ANTHROPIC_API_KEY) |
| `e` | Edit selected item |
| `t` | Edit tags |
| `y` | Yank (copy) |
| `d` | Delete |
| `x` | Cut (delete + buffer) |
| `p/P` | Paste after/before |

### Other

| Key | Action |
|-----|--------|
| `?` | Toggle help overlay |
| `q` / `Esc` | Quit (or close modal) |

## Data Storage

Bookmarks are stored in `~/.config/bm/bookmarks.json`:

```json
{
  "folders": [
    { "id": "uuid", "name": "Development", "parentId": null, "pinned": false }
  ],
  "bookmarks": [
    {
      "id": "uuid",
      "title": "GitHub",
      "url": "https://github.com",
      "folderId": "uuid",
      "tags": ["code", "git"],
      "createdAt": "2025-01-15T10:30:00Z",
      "visitedAt": "2025-01-20T14:22:00Z",
      "pinned": true
    }
  ]
}
```

The file is human-readable and can be version-controlled.

## License

MIT
