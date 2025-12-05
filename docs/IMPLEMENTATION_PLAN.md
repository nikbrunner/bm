# Plan: `bm` - Vim-style TUI Bookmark Manager (Phase 1-2)

## Scope
- **Phase 1**: Foundation (project setup, data model, JSON storage)
- **Phase 2**: Core TUI (Miller columns, vim navigation)
- **Approach**: TDD - tests first, then implementation

---

## Project Structure

```
/Users/nbr/repos/nikbrunner/bm/
├── cmd/bm/main.go                    # Entry point
├── internal/
│   ├── model/
│   │   ├── bookmark.go               # Bookmark struct
│   │   ├── folder.go                 # Folder struct
│   │   ├── store.go                  # Store (data container + queries)
│   │   ├── uuid.go                   # UUID generation
│   │   └── model_test.go             # Model tests
│   ├── storage/
│   │   ├── storage.go                # Storage interface + JSON impl
│   │   └── storage_test.go           # Storage tests
│   └── tui/
│       ├── app.go                    # Main bubbletea Model
│       ├── app_test.go               # Navigation tests
│       ├── keys.go                   # Key bindings
│       ├── styles.go                 # lipgloss styles
│       ├── item.go                   # Item type (folder/bookmark union)
│       └── view.go                   # View rendering
├── go.mod
└── go.sum
```

---

## Implementation Steps

### Phase 1: Foundation (COMPLETE)

| # | Task | Files | Status |
|---|------|-------|--------|
| 1 | Init Go module + deps | `go.mod` | :white_check_mark: |
| 2 | Bookmark struct + tests | `internal/model/bookmark.go`, `model_test.go` | :white_check_mark: |
| 3 | Folder struct + tests | `internal/model/folder.go` | :white_check_mark: |
| 4 | Store struct + query methods | `internal/model/store.go` | :white_check_mark: |
| 5 | UUID helper | `internal/model/uuid.go` | :white_check_mark: |
| 6 | JSON storage + tests | `internal/storage/storage.go`, `storage_test.go` | :white_check_mark: |

### Phase 2: Core TUI (COMPLETE)

| # | Task | Files | Status |
|---|------|-------|--------|
| 7 | Key bindings | `internal/tui/keys.go` | :white_check_mark: |
| 8 | Styles | `internal/tui/styles.go` | :white_check_mark: |
| 9 | Item type | `internal/tui/item.go` | :white_check_mark: |
| 10 | App model + navigation tests | `internal/tui/app.go`, `app_test.go` | :white_check_mark: |
| 11 | View rendering (Miller columns) | `internal/tui/view.go` | :white_check_mark: |
| 12 | Main entry point | `cmd/bm/main.go` | :white_check_mark: |

---

## Key Data Structures

### Bookmark
```go
type Bookmark struct {
    ID        string     `json:"id"`
    Title     string     `json:"title"`
    URL       string     `json:"url"`
    FolderID  *string    `json:"folderId"`  // nil = root
    Tags      []string   `json:"tags"`
    CreatedAt time.Time  `json:"createdAt"`
    VisitedAt *time.Time `json:"visitedAt"`
}
```

### Folder
```go
type Folder struct {
    ID       string  `json:"id"`
    Name     string  `json:"name"`
    ParentID *string `json:"parentId"` // nil = root
}
```

### Store
```go
type Store struct {
    Folders   []Folder   `json:"folders"`
    Bookmarks []Bookmark `json:"bookmarks"`
}

// Key methods:
// - GetFoldersInFolder(parentID *string) []Folder
// - GetBookmarksInFolder(folderID *string) []Bookmark
// - GetFolderByID(id string) *Folder
```

---

## Key TUI Components

### App State
```go
type App struct {
    store           *model.Store
    currentFolderID *string      // nil = root
    folderStack     []string     // for h navigation (breadcrumb)
    cursor          int
    items           []Item       // current folder contents
    lastKeyWasG     bool         // for gg detection
}
```

### Navigation
- `j/k` - move cursor up/down (bounded)
- `h` - pop folderStack, go to parent
- `l` - if folder selected, push to stack and enter
- `gg` - cursor to 0
- `G` - cursor to len(items)-1

### Miller Columns Layout
```
┌─────────────┬─────────────┬─────────────┐
│ Parent      │ Current     │ Preview     │
│ (context)   │ (active)    │ (details)   │
└─────────────┴─────────────┴─────────────┘
```

---

## Design Decisions

1. **`*string` for optional IDs** - `nil` = root level, clean JSON serialization
2. **Item union type** - struct with `Kind` discriminator, not interfaces
3. **folderStack** - maintains nav history without parent traversal
4. **Storage interface** - allows future SQLite swap
5. **Separate view.go** - keeps rendering isolated from state logic

---

## Dependencies

```
github.com/charmbracelet/bubbletea
github.com/charmbracelet/lipgloss
github.com/charmbracelet/bubbles
github.com/google/uuid
```

---

## MCP Usage Reminders

- **Ref MCP**: Check bubbletea/lipgloss docs before implementing TUI code
- **EXA MCP**: Search for Go idioms or real-world examples when uncertain
