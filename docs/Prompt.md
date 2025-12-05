# Project: `bm` - A vim-style TUI bookmark manager

A ranger/yazi-inspired terminal bookmark manager written in Go. Keyboard-driven, fast, simple JSON storage.

---

## ⚠️ CRITICAL: MCP Usage

The following MCPs are set up and **must be used**:

| MCP | Purpose |
|-----|---------|
| **Ref MCP** | Use for all documentation lookups (bubbletea, lipgloss, bubbles, Go stdlib). Always check docs before implementing. |
| **EXA MCP** | Use for web searches when you need examples, patterns, or solutions not in the docs. |
| **Chrome MCP** | Use for testing browser-related functionality (opening URLs, HTML export verification). |
| **Survey Tool** | Use when asking clarifying questions about requirements, specifications, or decisions. Always clarify before implementing. |

**Do not skip these.** Check Ref MCP for bubbletea patterns before writing TUI code. Use EXA if you're unsure about Go idioms or need real-world examples. Use Survey Tool to ask questions.

---

## Philosophy

- Terminal-native, not "feels like terminal"
- Vim motions are muscle memory, not a feature
- Data is a simple, flat JSON file - version controllable, portable
- No browser sync - copy URL, `bm`, `a`, done
- Export to standard HTML format for browser import

---

## Tech Stack

- Go
- [bubbletea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [lipgloss](https://github.com/charmbracelet/lipgloss) - Styling
- [bubbles](https://github.com/charmbracelet/bubbles) - Components (text input, etc.)

---

## Data Model

Location: `~/.config/bm/bookmarks.json`

```json
{
  "folders": [
    { "id": "f1", "name": "Development", "parentId": null },
    { "id": "f2", "name": "React", "parentId": "f1" }
  ],
  "bookmarks": [
    {
      "id": "b1",
      "title": "TanStack Router",
      "url": "https://tanstack.com/router",
      "folderId": "f2",
      "tags": ["react", "routing"],
      "createdAt": "2025-01-15T10:30:00Z",
      "visitedAt": "2025-01-20T14:22:00Z"
    }
  ]
}
```

**Rules:**
- `folderId: null` = root level bookmark
- `parentId: null` = root level folder
- IDs: UUIDs
- Order: array position (preserved on save)

---

## UI Layout - Miller Columns + Preview

```
┌─ bm ────────────────────────────────────────────────────────────────────┐
│ 📁 Development          │ 📁 React             │ TanStack Router        │
│ 📁 Design               │ 📁 Node              │ https://tanstack.com/r │
│ 📁 Reading              │ 📁 Go                │                        │
│ 📁 Tools                │   Bubbletea Docs     │ Tags: #react #routing  │
│   Hacker News           │ ▶ TanStack Router    │ Created: 2025-01-15    │
│   Lobste.rs             │                      │ Visited: 2025-01-20    │
│                         │                      │                        │
├─────────────────────────┴──────────────────────┴────────────────────────┤
│ j/k: move  h/l: navigate  o: open  a: add  dd: cut  p: paste  /: search │
└─────────────────────────────────────────────────────────────────────────┘
```

- 3 columns: parent | current | preview
- Preview pane shows: title, full URL, tags, created/visited dates
- Folders shown with 📁 prefix, sorted before bookmarks

---

## Keybinds

| Key | Action |
|-----|--------|
| `j/k` | Move down/up |
| `h/l` | Navigate out/into folder |
| `gg` | Jump to top |
| `G` | Jump to bottom |
| `o` / `Enter` | Open URL in default browser |
| `a` | Add bookmark (modal) |
| `A` | Add folder (modal) |
| `e` | Edit selected item |
| `dd` | Cut (delete + yank) |
| `yy` | Yank (copy for move) |
| `p` | Paste after cursor |
| `P` | Paste before cursor |
| `Y` | Yank URL to system clipboard |
| `/` | Fuzzy filter current view |
| `t` | Edit tags |
| `s` | Cycle sort: manual → alpha → date created → date visited |
| `?` | Toggle help overlay |
| `q` / `Esc` | Quit (or close modal) |

---

## CLI Interface

```bash
bm              # Opens full TUI
bm <query>      # Fuzzy search → select → opens in browser (no TUI)
```

---

## HTML Export

Export bookmarks to standard Netscape bookmark HTML format (compatible with all browsers).

**Format:**
```html
<!DOCTYPE NETSCAPE-Bookmark-file-1>
<META HTTP-EQUIV="Content-Type" CONTENT="text/html; charset=UTF-8">
<TITLE>Bookmarks</TITLE>
<H1>Bookmarks</H1>
<DL><p>
    <DT><H3 ADD_DATE="timestamp">Development</H3>
    <DL><p>
        <DT><H3 ADD_DATE="timestamp">React</H3>
        <DL><p>
            <DT><A HREF="https://tanstack.com/router" ADD_DATE="timestamp">TanStack Router</A>
        </DL><p>
    </DL><p>
</DL><p>
```

**Keybind:** `E` (capital E) - exports to `~/Downloads/bookmarks-export-YYYY-MM-DD.html`

---

## File Locations

```
~/.config/bm/
├── bookmarks.json    # Data
└── config.json       # Future: settings (default sort, colors, etc.)
```

---

## Behavior Notes

- `dd` on a folder: confirm prompt, deletes folder + all contents
- Empty folder: still navigable, show "(empty)" state
- Auto-save on every mutation
- `visitedAt` updated when opening via `o`/`Enter`
- Fuzzy search (`/`) filters current folder only
- `bm <query>` searches all bookmarks globally

---

## Future / v1.5

- AI-assisted add: auto-suggest folder placement + tags (Haiku 4.5 structured output)
- Import from browser HTML export
- Auto-backup before writes
- Config file for theming/defaults
- `bm add <url>` CLI shortcut

---

## Progress Tracking

### Phase 1: Foundation ✅
- [x] Project setup (Go module, dependencies)
- [x] Data model structs
- [x] JSON read/write with auto-save
- [x] UUID generation for IDs

### Phase 2: Core TUI ✅
- [x] Basic bubbletea app structure
- [x] Miller column layout (3 panes)
- [x] Folder/bookmark rendering
- [x] `j/k` navigation within list
- [x] `h/l` navigation between folders
- [x] `gg/G` jump to top/bottom
- [x] Cursor/selection highlighting
- [x] Preview pane (title, URL, tags, dates)

### Phase 3: CRUD Operations ✅
- [x] `a` - Add bookmark modal (title, URL, tags input)
- [x] `A` - Add folder modal
- [x] `e` - Edit selected item
- [x] `dd` - Cut (delete + yank to buffer)
- [x] `yy` - Yank to buffer
- [x] `p/P` - Paste after/before
- [x] `t` - Edit tags
- [x] Delete folder confirmation prompt
- [x] Persist changes on exit

### Phase 4: Search & Sort ✅
- [x] `/` - Global fuzzy finder (searches all bookmarks/folders)
- [x] `s` - Cycle sort modes (manual → alpha → created → visited)
- [x] Sort indicator in UI
- [x] Fuzzy matching with sahilm/fuzzy library
- [x] Results + preview pane layout

### Phase 5: Actions & Polish
- [ ] `o`/`Enter` - Open URL in default browser
- [ ] `Y` - Yank URL to system clipboard
- [ ] Update `visitedAt` on open
- [ ] `?` - Help overlay
- [x] `q`/`Esc` - Quit handling
- [x] Empty state UI
- [ ] Error handling & user feedback

### Phase 6: CLI & Export
- [ ] `bm <query>` - Quick fuzzy open mode
- [ ] `E` - HTML export
- [ ] Export filename with date

### Phase 7: Final Polish
- [ ] Edge case testing
- [ ] README documentation
- [ ] Installation instructions
