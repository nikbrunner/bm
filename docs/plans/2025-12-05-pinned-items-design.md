# Design: Pinned Items Feature

## Overview

Add a pinned/favorites system that displays frequently-used bookmarks and folders in the leftmost pane for quick access.

---

## Requirements

- Pinned items appear in the leftmost pane below the "bm" header
- Items are shortcuts - the original bookmark/folder remains in its location
- Both bookmarks and folders can be pinned
- App starts with focus on the pinned pane
- Pinned items show `★` prefix in their original folder location
- `m` key toggles pin/unpin

---

## Data Model

### Bookmark (updated)

```go
type Bookmark struct {
    ID        string     `json:"id"`
    Title     string     `json:"title"`
    URL       string     `json:"url"`
    FolderID  *string    `json:"folderId"`
    Tags      []string   `json:"tags"`
    CreatedAt time.Time  `json:"createdAt"`
    VisitedAt *time.Time `json:"visitedAt"`
    Pinned    bool       `json:"pinned"`  // NEW
}
```

### Folder (updated)

```go
type Folder struct {
    ID       string  `json:"id"`
    Name     string  `json:"name"`
    ParentID *string `json:"parentId"`
    Pinned   bool    `json:"pinned"`  // NEW
}
```

### Store Methods (new)

```go
func (s *Store) GetPinnedBookmarks() []Bookmark
func (s *Store) GetPinnedFolders() []Folder
func (s *Store) TogglePinBookmark(id string) error
func (s *Store) TogglePinFolder(id string) error
```

---

## UI Layout

```
┌─ bm ────────────────────────────────────────────────────────────────────┐
│ bm                      │ 📁 Development         │ 📁 React             │
│ bookmarks               │ 📁 Design              │ 📁 Go                │
│                         │ 📁 Reading             │                      │
│ ── Pinned ──────────    │   Hacker News          │                      │
│ ★ TanStack Router       │   Lobsters             │                      │
│ ★ 📁 Development        │                        │                      │
│                         │                        │                      │
├─────────────────────────┴──────────────────────┴────────────────────────┤
│ j/k: move  h/l: navigate  o: open  m: pin/unpin  a: add  dd: cut       │
└─────────────────────────────────────────────────────────────────────────┘
```

- Left pane: app header + pinned section
- Middle pane: folder browser (current behavior)
- Right pane: preview or folder contents

---

## Navigation

### Focus States

App tracks focus between two panes:
- `pinnedPane` - leftmost column with pinned items
- `browserPane` - Miller column folder browser

Visual distinction: focused pane has highlighted border.

### Pinned Pane Keybindings

| Key | Action |
|-----|--------|
| `j/k` | Move cursor within pinned items |
| `l` | Move focus to browser pane |
| `o` / `Enter` | Bookmark: open URL. Folder: navigate to it in browser |
| `m` | Unpin selected item |
| `dd` | Unpin (does NOT delete original) |
| `gg/G` | Jump to top/bottom |

### Browser Pane Keybindings (updated)

| Key | Action |
|-----|--------|
| `h` | At root: move focus to pinned pane. Otherwise: parent folder |
| `m` | Toggle pin on selected item |

### Pinned Item Activation

- **Pinned bookmark** + `o`/`Enter`: Opens URL in default browser
- **Pinned folder** + `o`/`Enter`: Navigates browser pane to that folder

---

## Visual Indicators

- Pinned items in their original folder show `★` prefix
- In pinned pane, no star prefix needed (implied by location)

---

## Implementation Tasks

| # | Task | Files | Test |
|---|------|-------|------|
| 1 | Add `Pinned` field to Bookmark struct | `internal/model/bookmark.go` | - |
| 2 | Add `Pinned` field to Folder struct | `internal/model/folder.go` | - |
| 3 | Add `GetPinnedBookmarks()` method | `internal/model/store.go` | Yes |
| 4 | Add `GetPinnedFolders()` method | `internal/model/store.go` | Yes |
| 5 | Add `TogglePinBookmark(id)` method | `internal/model/store.go` | Yes |
| 6 | Add `TogglePinFolder(id)` method | `internal/model/store.go` | Yes |
| 7 | Add `FocusedPane` enum and state to App | `internal/tui/app.go` | - |
| 8 | Add `pinnedCursor` state to App | `internal/tui/app.go` | - |
| 9 | Add `m` key binding | `internal/tui/keys.go` | - |
| 10 | Implement `m` handler (toggle pin) | `internal/tui/app.go` | Yes |
| 11 | Update `h` handler: at root, switch to pinned pane | `internal/tui/app.go` | Yes |
| 12 | Add `l` handler in pinned pane: switch to browser | `internal/tui/app.go` | Yes |
| 13 | Render pinned pane in left column | `internal/tui/view.go` | - |
| 14 | Add `★` prefix for pinned items in browser | `internal/tui/view.go` | - |
| 15 | Handle `o`/`Enter` on pinned bookmark (open URL) | `internal/tui/app.go` | - |
| 16 | Handle `o`/`Enter` on pinned folder (navigate) | `internal/tui/app.go` | Yes |
| 17 | Set initial focus to pinned pane | `internal/tui/app.go` | - |
| 18 | Handle `dd` in pinned pane (unpin, not delete) | `internal/tui/app.go` | Yes |

---

## Edge Cases

- Empty pinned pane: show "(no pinned items)" placeholder
- Deleting a pinned item via browser: automatically removes from pinned
- `dd` in pinned pane only unpins, never deletes the original
- Pinning an already-pinned item: no-op (or unpin - toggle behavior)

---

## Future Considerations

- Smart add link with AI
- Autocomplete for tags
