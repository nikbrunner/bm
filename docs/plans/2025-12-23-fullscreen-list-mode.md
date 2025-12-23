# Fullscreen List Mode Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Refactor ModeSearch into a generic fullscreen list mode that supports multiple data sources, starting with a "Recent Bookmarks" view accessible via `R`.

**Architecture:** Add a `ListSource` enum to SearchState to switch between "all items" and "recent bookmarks". The rendering and input handling remain shared; only the data source and title change. Recent view shows bookmarks sorted by CreatedAt with folder path displayed.

**Tech Stack:** Go, bubbletea, lipgloss, bubbles/textinput

---

## Task 1: Add ListSource enum and update SearchState

**Files:**
- Modify: `internal/tui/state.go:92-104`

**Step 1: Add ListSource type after SearchState definition**

In `internal/tui/state.go`, add after line 104 (after the SearchState struct):

```go
// ListSource represents the data source for fullscreen list mode.
type ListSource int

const (
	SourceAll    ListSource = iota // All items (folders + bookmarks)
	SourceRecent                   // Bookmarks only, sorted by CreatedAt
)
```

**Step 2: Add Source field to SearchState**

Modify the SearchState struct (around line 92-104) to add a Source field:

```go
// SearchState holds state for global search and local filtering.
type SearchState struct {
	// Fullscreen list source
	Source ListSource // Which data source to use

	// Global search
	Input        textinput.Model // Search input
	FuzzyMatches []fuzzyMatch    // Current fuzzy match results
	FuzzyCursor  int             // Selected index in fuzzy results
	AllItems     []Item          // All items for global search

	// Local filter
	FilterInput   textinput.Model // Filter input for current folder
	FilterQuery   string          // Active filter query (persists after closing filter)
	FilteredItems []Item          // Items matching filter in current folder
}
```

**Step 3: Run build to verify syntax**

Run: `go build ./...`
Expected: Build succeeds with no errors

**Step 4: Commit**

```bash
git add internal/tui/state.go
git commit -m "feat(tui): add ListSource enum to SearchState"
```

---

## Task 2: Add Recent keybinding to KeyMap

**Files:**
- Modify: `internal/tui/keys.go:6-38` (KeyMap struct)
- Modify: `internal/tui/keys.go:41-167` (DefaultKeyMap function)

**Step 1: Add Recent field to KeyMap struct**

Add after line 26 (after `Filter`):

```go
	Recent        key.Binding
```

**Step 2: Add Recent binding to DefaultKeyMap**

Add after line 121 (after the Filter binding):

```go
		Recent: key.NewBinding(
			key.WithKeys("R"),
			key.WithHelp("R", "recent bookmarks"),
		),
```

**Step 3: Run build to verify**

Run: `go build ./...`
Expected: Build succeeds

**Step 4: Commit**

```bash
git add internal/tui/keys.go
git commit -m "feat(tui): add R keybinding for recent bookmarks"
```

---

## Task 3: Create helper to get items by source

**Files:**
- Modify: `internal/tui/app.go`

**Step 1: Add getItemsForSource function**

Add after `updateFuzzyMatches` function (around line 507):

```go
// getItemsForSource returns items based on the current list source.
func (a *App) getItemsForSource(source ListSource) []Item {
	switch source {
	case SourceRecent:
		// Bookmarks only, sorted by CreatedAt descending
		items := make([]Item, 0, len(a.store.Bookmarks))
		for i := range a.store.Bookmarks {
			items = append(items, Item{
				Kind:     ItemBookmark,
				Bookmark: &a.store.Bookmarks[i],
			})
		}
		// Sort by CreatedAt descending (newest first)
		sort.Slice(items, func(i, j int) bool {
			return items[i].Bookmark.CreatedAt.After(items[j].Bookmark.CreatedAt)
		})
		return items

	default: // SourceAll
		// All items (folders + bookmarks)
		items := make([]Item, 0, len(a.store.Folders)+len(a.store.Bookmarks))
		for i := range a.store.Folders {
			items = append(items, Item{
				Kind:   ItemFolder,
				Folder: &a.store.Folders[i],
			})
		}
		for i := range a.store.Bookmarks {
			items = append(items, Item{
				Kind:     ItemBookmark,
				Bookmark: &a.store.Bookmarks[i],
			})
		}
		return items
	}
}
```

**Step 2: Run build to verify**

Run: `go build ./...`
Expected: Build succeeds

**Step 3: Commit**

```bash
git add internal/tui/app.go
git commit -m "feat(tui): add getItemsForSource helper for list sources"
```

---

## Task 4: Refactor ModeSearch entry to use getItemsForSource

**Files:**
- Modify: `internal/tui/app.go:902-923`

**Step 1: Replace inline item gathering with helper call**

Find the ModeSearch entry code (around line 902-923) and replace:

```go
		case key.Matches(msg, a.keys.Search):
			// Open fuzzy finder mode with GLOBAL search
			a.mode = ModeSearch
			a.search.Source = SourceAll
			a.search.Input.Reset()
			a.search.Input.Focus()
			a.search.FuzzyCursor = 0
			a.search.AllItems = a.getItemsForSource(SourceAll)
			a.updateFuzzyMatches()
			return a, a.search.Input.Focus()
```

**Step 2: Run build to verify**

Run: `go build ./...`
Expected: Build succeeds

**Step 3: Run tests to verify no regression**

Run: `go test ./internal/tui/... -run TestSearch -v`
Expected: All search-related tests pass

**Step 4: Commit**

```bash
git add internal/tui/app.go
git commit -m "refactor(tui): use getItemsForSource for ModeSearch entry"
```

---

## Task 5: Add R keybinding handler for Recent mode

**Files:**
- Modify: `internal/tui/app.go` (in the normal mode key handling section)

**Step 1: Add handler for Recent key**

Find the switch block that handles `a.keys.Search` (around line 901) and add after the Filter case (around line 931):

```go
		case key.Matches(msg, a.keys.Recent):
			// Open fullscreen list with recent bookmarks
			a.mode = ModeSearch
			a.search.Source = SourceRecent
			a.search.Input.Reset()
			a.search.Input.Focus()
			a.search.FuzzyCursor = 0
			a.search.AllItems = a.getItemsForSource(SourceRecent)
			a.updateFuzzyMatches()
			return a, a.search.Input.Focus()
```

**Step 2: Run build to verify**

Run: `go build ./...`
Expected: Build succeeds

**Step 3: Manual test**

Run: `go run ./cmd/bm`
Press `R` to enter recent mode
Expected: Fullscreen list appears with bookmarks (may show "Find" title still - we'll fix that next)

**Step 4: Commit**

```bash
git add internal/tui/app.go
git commit -m "feat(tui): add R keybinding to open recent bookmarks view"
```

---

## Task 6: Update renderFuzzyFinder to show source-specific title

**Files:**
- Modify: `internal/tui/view.go:611-706` (renderFuzzyFinder function)

**Step 1: Add title based on source**

Find the content building section (around line 688-694) and update:

```go
	// Title based on source
	var title string
	switch a.search.Source {
	case SourceRecent:
		title = "Recent Bookmarks"
	default:
		title = "Find"
	}

	// Result count
	countStr := fmt.Sprintf("%d results", len(a.search.FuzzyMatches))
	if len(a.search.FuzzyMatches) == 1 {
		countStr = "1 result"
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		a.styles.Title.Render(title)+"  "+a.styles.Empty.Render(countStr),
		"",
		"> "+a.search.Input.Value()+"█",
		"",
		panes,
	)
```

**Step 2: Run build to verify**

Run: `go build ./...`
Expected: Build succeeds

**Step 3: Manual test**

Run: `go run ./cmd/bm`
Press `s` - should show "Find"
Press `Esc`, then `R` - should show "Recent Bookmarks"

**Step 4: Commit**

```bash
git add internal/tui/view.go
git commit -m "feat(tui): show source-specific title in fullscreen list"
```

---

## Task 7: Add folder path display for recent items

**Files:**
- Modify: `internal/tui/view.go:708-750` (renderFuzzyItem function)

**Step 1: Read the current renderFuzzyItem function**

First understand the current implementation (around line 708).

**Step 2: Add folder path for SourceRecent**

Modify `renderFuzzyItem` to accept source and show folder path. Update the function signature and add folder path rendering:

```go
// renderFuzzyItem renders a single item in the fuzzy results with highlighting.
func (a App) renderFuzzyItem(match fuzzyMatch, selected bool, maxWidth int, source ListSource) string {
	item := match.Item
	icon := "📁 "
	name := item.Title()

	if item.IsBookmark() {
		icon = "🔖 "
	}

	// For recent source, show folder path
	var folderPath string
	if source == SourceRecent && item.IsBookmark() {
		if item.Bookmark.FolderID != nil {
			folderPath = a.store.GetFolderPath(item.Bookmark.FolderID)
		} else {
			folderPath = "─"
		}
	}

	// Calculate available width for name
	pathWidth := 0
	if folderPath != "" {
		pathWidth = len(folderPath) + 2 // 2 for spacing
	}
	nameWidth := maxWidth - len(icon) - pathWidth

	// Truncate name if needed
	if len(name) > nameWidth {
		name = name[:nameWidth-1] + "…"
	}

	// Build the line
	var line string
	if selected {
		line = a.styles.SelectedItem.Render(icon + name)
	} else if len(match.MatchedIndexes) > 0 {
		// Highlight matched characters
		line = icon + a.highlightMatches(name, match.MatchedIndexes)
	} else {
		line = a.styles.Item.Render(icon + name)
	}

	// Add folder path for recent view
	if folderPath != "" {
		// Pad to align folder paths
		padding := maxWidth - len(icon) - len(name) - len(folderPath) - 2
		if padding < 1 {
			padding = 1
		}
		line += strings.Repeat(" ", padding) + a.styles.Empty.Render(folderPath)
	}

	return line
}
```

**Step 3: Update call site in renderFuzzyFinder**

Find where `renderFuzzyItem` is called (around line 633) and update:

```go
			line := a.renderFuzzyItem(match, isSelected, listItemWidth, a.search.Source)
```

**Step 4: Run build to verify**

Run: `go build ./...`
Expected: Build succeeds

**Step 5: Manual test**

Run: `go run ./cmd/bm`
Press `R` - bookmarks should show folder path on the right

**Step 6: Commit**

```bash
git add internal/tui/view.go
git commit -m "feat(tui): show folder path for items in recent view"
```

---

## Task 8: Update hints to include Recent keybinding

**Files:**
- Modify: `internal/tui/hints.go:64-76` (getGlobalHints function)

**Step 1: Add Recent hint**

Update `getGlobalHints` to include the R keybinding:

```go
// getGlobalHints returns hints for keys that work in any pane/mode.
func (a App) getGlobalHints() []Hint {
	return []Hint{
		{Key: "s", Desc: "search"},
		{Key: "R", Desc: "recent"},
		{Key: "/", Desc: "filter"},
		{Key: "i", Desc: "AI add"},
		{Key: "L", Desc: "read later"},
		{Key: "a/A", Desc: "add"},
		{Key: "C", Desc: "cull"},
		{Key: "?", Desc: "help"},
		{Key: "q", Desc: "quit"},
	}
}
```

**Step 2: Run build to verify**

Run: `go build ./...`
Expected: Build succeeds

**Step 3: Commit**

```bash
git add internal/tui/hints.go
git commit -m "feat(tui): add R hint for recent bookmarks"
```

---

## Task 9: Add test for recent bookmarks ordering

**Files:**
- Modify: `internal/tui/app_test.go`

**Step 1: Add test for getItemsForSource with SourceRecent**

Add a new test function:

```go
func TestGetItemsForSourceRecent(t *testing.T) {
	// Create store with bookmarks at different times
	store := &model.Store{}

	now := time.Now()

	// Add bookmarks with different creation times
	b1 := model.Bookmark{
		ID:        "b1",
		Title:     "Oldest",
		URL:       "https://oldest.com",
		CreatedAt: now.Add(-2 * time.Hour),
	}
	b2 := model.Bookmark{
		ID:        "b2",
		Title:     "Newest",
		URL:       "https://newest.com",
		CreatedAt: now,
	}
	b3 := model.Bookmark{
		ID:        "b3",
		Title:     "Middle",
		URL:       "https://middle.com",
		CreatedAt: now.Add(-1 * time.Hour),
	}

	store.Bookmarks = []model.Bookmark{b1, b2, b3}

	// Create minimal app
	app := &App{store: store}

	// Get items for recent source
	items := app.getItemsForSource(SourceRecent)

	// Verify order: newest first
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
	if items[0].Bookmark.Title != "Newest" {
		t.Errorf("expected first item to be 'Newest', got '%s'", items[0].Bookmark.Title)
	}
	if items[1].Bookmark.Title != "Middle" {
		t.Errorf("expected second item to be 'Middle', got '%s'", items[1].Bookmark.Title)
	}
	if items[2].Bookmark.Title != "Oldest" {
		t.Errorf("expected third item to be 'Oldest', got '%s'", items[2].Bookmark.Title)
	}

	// Verify no folders included
	for _, item := range items {
		if item.IsFolder() {
			t.Error("SourceRecent should not include folders")
		}
	}
}
```

**Step 2: Run the test**

Run: `go test ./internal/tui -run TestGetItemsForSourceRecent -v`
Expected: Test passes

**Step 3: Commit**

```bash
git add internal/tui/app_test.go
git commit -m "test(tui): add test for SourceRecent ordering"
```

---

## Task 10: Final integration test and cleanup

**Step 1: Run all tests**

Run: `go test ./... -v`
Expected: All tests pass

**Step 2: Run linter if available**

Run: `golangci-lint run` (if available)
Expected: No new issues

**Step 3: Manual integration test**

Run: `go run ./cmd/bm`

Test the following:
1. Press `s` - shows "Find" with all items
2. Type a query - fuzzy matches work
3. Press `Esc` to exit
4. Press `R` - shows "Recent Bookmarks" with bookmarks only
5. Verify bookmarks are sorted newest first
6. Verify folder path shows on the right for each bookmark
7. Type a query - filters the recent list
8. Press `l` or `Enter` on a bookmark - opens in browser
9. Press `Esc` to exit recent view

**Step 4: Commit any fixes and tag completion**

```bash
git add -A
git commit -m "feat(tui): complete fullscreen list mode with recent bookmarks

- Add ListSource enum (SourceAll, SourceRecent)
- Add R keybinding for recent bookmarks view
- Show folder path for items in recent view
- Refactor ModeSearch to use generic data source pattern
- Add test for recent ordering"
```

---

## Summary

After completing all tasks:
- `s` opens fullscreen search (all items, fuzzy filtered)
- `R` opens fullscreen recent (bookmarks only, sorted by CreatedAt)
- Both share the same UI (input, list, preview)
- Recent view shows folder path next to each bookmark
- All bookmark actions work in both views
