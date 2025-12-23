# Fullscreen List Mode Design

## Overview

Refactor the existing `ModeSearch` into a generic `ModeFullscreenList` that supports multiple data sources. The first new source will be "Recent Bookmarks" — a view showing all bookmarks sorted by creation date with fuzzy search/filter capability.

## Motivation

- Users want to see recently added bookmarks across all folders
- The search view (`s`) and proposed recent view (`R`) share identical UI patterns
- A generic abstraction enables future list views (pinned items, items by tag, etc.)

## Design

### List Sources

```go
type ListSource int

const (
    SourceAll    ListSource = iota // All items (folders + bookmarks), current search behavior
    SourceRecent                   // Bookmarks only, sorted by CreatedAt descending
)
```

### Keybindings

| Source | Key | Title |
|--------|-----|-------|
| `SourceAll` | `s` | "Find" |
| `SourceRecent` | `R` | "Recent Bookmarks" |

### Behavior by Source

| Aspect | SourceAll | SourceRecent |
|--------|-----------|--------------|
| **Items** | All folders + bookmarks | Bookmarks only |
| **Base sort** | None (fuzzy relevance) | CreatedAt descending |
| **Search** | Fuzzy match on title/URL | Fuzzy filter on pre-sorted list |
| **Show folder path** | No (items shown with icon) | Yes (right-aligned, dimmed) |

### State Changes

Rename/refactor search-related state to be generic:

```go
// Before (in App struct)
search struct {
    Input        textinput.Model
    FuzzyMatches []fuzzyMatch
    FuzzyCursor  int
}

// After
fullscreenList struct {
    Source       ListSource
    Input        textinput.Model
    Items        []Item           // Base items for current source
    Matches      []fuzzyMatch     // Filtered/matched items
    Cursor       int
}
```

### UI Layout

Fullscreen two-pane layout (matches current search view):

```
┌─────────────────────────────────────────────────────────────┐
│  Recent Bookmarks  42 results                               │
│                                                             │
│  > search query█                                            │
│                                                             │
│  ▸ Bubbletea Docs                Dev/Go   │  Bubbletea Docs │
│    TanStack Router               Work/Web │                 │
│    Claude API Ref                Dev/AI   │  https://...    │
│    Some Article                  ─        │                 │
│                                           │  #go #tui       │
│                                                             │
├─────────────────────────────────────────────────────────────┤
│  ↑↓ navigate · l open · y yank · q back                     │
└─────────────────────────────────────────────────────────────┘
```

- Left pane: List with cursor, folder path on right (for SourceRecent)
- Right pane: Preview of selected item (title, URL, tags)
- Bottom: Context-aware help bar

### Actions Available

All standard bookmark actions work in this view:

| Key | Action |
|-----|--------|
| `j/k` or `↑/↓` | Navigate list |
| `l` or `Enter` | Open bookmark in browser |
| `y` | Yank URL to clipboard |
| `d` | Delete (respects confirmDelete toggle) |
| `x` | Cut (delete + clipboard) |
| `*` | Toggle pin |
| `m` | Move to different folder |
| `e` | Edit bookmark |
| `q` or `Esc` | Return to normal mode |

### Folder Path Display

For `SourceRecent`, each bookmark shows its folder location:

- Nested folders show full path: `Dev/Go/Libraries`
- Root-level bookmarks show: `─` (em dash)
- Path is right-aligned and dimmed to not compete with title

### Implementation Approach

1. **Rename `ModeSearch` to `ModeFullscreenList`**
2. **Add `ListSource` enum and field to state**
3. **Extract `getItemsForSource(source ListSource) []Item` function**
   - `SourceAll`: Return `allItems` (existing behavior)
   - `SourceRecent`: Filter to bookmarks, sort by `CreatedAt` desc
4. **Update `renderFullscreenList()` to handle source-specific rendering**
   - Title based on source
   - Folder path column for `SourceRecent`
5. **Add `R` keybinding to enter `ModeFullscreenList` with `SourceRecent`**
6. **Update `s` keybinding to set `SourceAll`**

### Future Extensibility

This design supports adding more sources:

```go
const (
    SourceAll      ListSource = iota
    SourceRecent
    SourcePinned   // All pinned items
    SourceTag      // Items with specific tag (would need tag parameter)
    SourceVisited  // Recently visited (by VisitedAt)
)
```

## Files to Modify

- `internal/tui/app.go` — Mode enum, state struct, keybindings
- `internal/tui/view.go` — Rendering logic
- `internal/tui/hints.go` — Help bar hints for new source
- `internal/tui/keys.go` — Add `R` keybinding

## Testing

- Verify `s` still works as before (regression)
- Verify `R` shows bookmarks sorted by CreatedAt
- Verify search/filter works in both modes
- Verify all actions (open, yank, delete, etc.) work in recent view
- Verify folder path displays correctly
