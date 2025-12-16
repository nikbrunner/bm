# Auto-Sort Feature Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add AI-powered sorting that suggests better locations for bookmarks and folders.

**Architecture:** Press `S` on any item to analyze it (or its contents recursively if a folder). AI suggests relocations, user reviews each in a list UI with accept/skip/edit/delete actions. Follows existing cull feature pattern for async loading and list navigation.

**Tech Stack:** Go, bubbletea, lipgloss, Anthropic Claude API (haiku model)

---

## Task 1: Add SortState and SortSuggestion Types

**Files:**
- Modify: `internal/tui/state.go`

**Step 1: Add the new types after CullState**

Add after line ~320 in state.go (after CullState methods):

```go
// SortState holds state for the AI-powered auto-sort feature.
type SortState struct {
	SourceFolderID *string          // Folder being sorted (nil if single item)
	SourceItem     *Item            // Single item if sorting one bookmark/folder
	Suggestions    []SortSuggestion // Items that need relocation
	Cursor         int              // Current selection in suggestions list
	Progress       int              // Items analyzed so far
	Total          int              // Total items to analyze
}

// SortSuggestion represents a suggested relocation for an item.
type SortSuggestion struct {
	Item          Item   // The bookmark or folder to move
	CurrentPath   string // Where it is now
	SuggestedPath string // Where AI suggests moving it
	IsNewFolder   bool   // True if destination folder doesn't exist yet
	Processed     bool   // True after user accepts/skips/deletes
}

// NewSortState creates an empty SortState.
func NewSortState() SortState {
	return SortState{}
}

// Reset clears all sort state.
func (s *SortState) Reset() {
	s.SourceFolderID = nil
	s.SourceItem = nil
	s.Suggestions = nil
	s.Cursor = 0
	s.Progress = 0
	s.Total = 0
}

// CurrentSuggestion returns the currently selected suggestion, or nil if none.
func (s *SortState) CurrentSuggestion() *SortSuggestion {
	if len(s.Suggestions) == 0 || s.Cursor >= len(s.Suggestions) {
		return nil
	}
	return &s.Suggestions[s.Cursor]
}

// UnprocessedCount returns the number of suggestions not yet processed.
func (s *SortState) UnprocessedCount() int {
	count := 0
	for _, sug := range s.Suggestions {
		if !sug.Processed {
			count++
		}
	}
	return count
}
```

**Step 2: Verify it compiles**

Run: `go build ./...`
Expected: No errors

**Step 3: Commit**

```bash
git add internal/tui/state.go
git commit -m "feat(sort): add SortState and SortSuggestion types"
```

---

## Task 2: Add Sort Mode Constants

**Files:**
- Modify: `internal/tui/app.go`

**Step 1: Add mode constants after ModeCullInspect (around line 73)**

```go
	ModeCullInspect          // Bookmark list within a cull group
	ModeSortLoading          // Analyzing items for sort suggestions
	ModeSortResults          // List of suggested moves
```

**Step 2: Add sortProgressCounter after cullProgressCounter (around line 29)**

```go
var cullProgressCounter int64
var sortProgressCounter int64
```

**Step 3: Add sort field to App struct (around line 206, after cull)**

Find the `cull CullState` line and add after it:

```go
	cull CullState
	sort SortState
```

**Step 4: Initialize sort state in New() function**

Find `cull: NewCullState(),` and add after it:

```go
		cull:          NewCullState(),
		sort:          NewSortState(),
```

**Step 5: Verify it compiles**

Run: `go build ./...`
Expected: No errors

**Step 6: Commit**

```bash
git add internal/tui/app.go
git commit -m "feat(sort): add ModeSortLoading and ModeSortResults modes"
```

---

## Task 3: Add AI SuggestSort Function and Types

**Files:**
- Modify: `internal/ai/types.go`
- Modify: `internal/ai/client.go`

**Step 1: Add SortResponse type to types.go**

Add after the Response struct:

```go
// SortResponse represents the AI-suggested relocation for an item.
type SortResponse struct {
	FolderPath  string `json:"folderPath"`
	IsNewFolder bool   `json:"isNewFolder"`
	Confidence  string `json:"confidence"` // "high", "medium", "low"
}
```

**Step 2: Add SuggestSort function to client.go**

Add after the SuggestBookmark function:

```go
// SuggestSort calls the AI to suggest a better folder location for an item.
func (c *Client) SuggestSort(title, url, currentPath string, tags []string, isFolder bool, context string) (*SortResponse, error) {
	prompt := buildSortPrompt(title, url, currentPath, tags, isFolder, context)

	reqBody := apiRequest{
		Model:     haikuModel,
		MaxTokens: 256,
		Messages: []apiMessage{
			{Role: "user", Content: prompt},
		},
		OutputFormat: &outputFormat{
			Type: "json_schema",
			Schema: jsonSchema{
				Type: "object",
				Properties: map[string]schemaProp{
					"folderPath":  {Type: "string"},
					"isNewFolder": {Type: "boolean"},
					"confidence":  {Type: "string"},
				},
				Required:             []string{"folderPath", "isNewFolder", "confidence"},
				AdditionalProperties: false,
			},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", apiVersion)
	req.Header.Set("anthropic-beta", betaHeader)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAPIRequest, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status %d: %s", ErrAPIRequest, resp.StatusCode, string(body))
	}

	var apiResp apiResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if len(apiResp.Content) == 0 || apiResp.Content[0].Type != "text" {
		return nil, ErrInvalidResponse
	}

	var result SortResponse
	if err := json.Unmarshal([]byte(apiResp.Content[0].Text), &result); err != nil {
		return nil, fmt.Errorf("unmarshal AI response: %w", err)
	}

	return &result, nil
}

func buildSortPrompt(title, url, currentPath string, tags []string, isFolder bool, context string) string {
	itemType := "bookmark"
	if isFolder {
		itemType = "folder"
	}

	tagsStr := ""
	if len(tags) > 0 {
		tagsStr = fmt.Sprintf("\n- Tags: %s", strings.Join(tags, ", "))
	}

	urlStr := ""
	if url != "" {
		urlStr = fmt.Sprintf("\n- URL: %s", url)
	}

	return fmt.Sprintf(`Analyze this %s and suggest the best parent folder for it.

Item:
- Title: %s%s
- Current folder: %s%s

%s

Instructions:
- Prefer existing folders when they fit well
- Only suggest a new folder path if nothing existing is appropriate
- Set isNewFolder=true only when suggesting a folder that doesn't exist
- If current location is already optimal, return the current path exactly
- Confidence: "high" if clear match, "medium" if reasonable, "low" if uncertain`,
		itemType, title, urlStr, currentPath, tagsStr, context)
}
```

**Step 3: Add strings import to client.go if not present**

Check if `"strings"` is in the imports, add if missing.

**Step 4: Verify it compiles**

Run: `go build ./...`
Expected: No errors

**Step 5: Commit**

```bash
git add internal/ai/types.go internal/ai/client.go
git commit -m "feat(ai): add SuggestSort function for auto-sort feature"
```

---

## Task 4: Add Sort Message Types

**Files:**
- Modify: `internal/tui/app.go`

**Step 1: Add message types after aiResponseMsg (around line 48)**

```go
// sortResponseMsg is sent when an AI sort suggestion completes.
type sortResponseMsg struct {
	item     Item
	response *ai.SortResponse
	err      error
}

// sortCompleteMsg is sent when all items have been analyzed.
type sortCompleteMsg struct{}

// sortTickMsg is sent periodically to update progress display.
type sortTickMsg struct{}
```

**Step 2: Verify it compiles**

Run: `go build ./...`
Expected: No errors

**Step 3: Commit**

```bash
git add internal/tui/app.go
git commit -m "feat(sort): add sort message types"
```

---

## Task 5: Add Sort Keybinding Handler

**Files:**
- Modify: `internal/tui/app.go`

**Step 1: Find the key handling section for normal mode**

Look for where other keybindings like "C" for cull are handled. Add the "S" handler nearby (around line 880-890):

```go
		case "S":
			// Auto-sort: analyze current item or folder contents
			return a.startSort()
```

**Step 2: Add the startSort method**

Add after the cull-related methods (around line 3070):

```go
// startSort initiates the AI-powered sort analysis.
func (a *App) startSort() (tea.Model, tea.Cmd) {
	// Check for API key first
	client, err := ai.NewClient()
	if err != nil {
		a.setError("No API key: set ANTHROPIC_API_KEY")
		return a, nil
	}
	_ = client // Will be used in the command

	// Get the current item
	item := a.getCurrentItem()
	if item == nil {
		a.setError("No item selected")
		return a, nil
	}

	// Reset sort state
	a.sort.Reset()

	// Collect items to analyze
	var itemsToAnalyze []Item
	if item.IsFolder() {
		// Recursively collect all items in folder
		itemsToAnalyze = a.collectFolderItemsRecursive(item.Folder.ID)
		a.sort.SourceFolderID = &item.Folder.ID
	} else {
		itemsToAnalyze = []Item{*item}
		a.sort.SourceItem = item
	}

	if len(itemsToAnalyze) == 0 {
		a.setStatus("No items to sort")
		return a, nil
	}

	a.sort.Total = len(itemsToAnalyze)
	a.mode = ModeSortLoading

	// Reset progress counter
	atomic.StoreInt64(&sortProgressCounter, 0)

	// Start analysis
	return a, tea.Batch(
		a.analyzeSortItems(itemsToAnalyze),
		sortTickCmd(),
	)
}

// collectFolderItemsRecursive collects all bookmarks and folders recursively.
func (a *App) collectFolderItemsRecursive(folderID string) []Item {
	var items []Item

	// Get direct children
	bookmarks := a.store.GetBookmarksInFolder(&folderID)
	for i := range bookmarks {
		items = append(items, Item{Kind: ItemBookmark, Bookmark: &bookmarks[i]})
	}

	folders := a.store.GetFoldersInFolder(&folderID)
	for i := range folders {
		items = append(items, Item{Kind: ItemFolder, Folder: &folders[i]})
		// Recurse into subfolders
		items = append(items, a.collectFolderItemsRecursive(folders[i].ID)...)
	}

	return items
}

// analyzeSortItems starts the AI analysis for all items.
func (a *App) analyzeSortItems(items []Item) tea.Cmd {
	return func() tea.Msg {
		client, err := ai.NewClient()
		if err != nil {
			return sortCompleteMsg{}
		}

		context := ai.BuildContext(a.store)
		var suggestions []SortSuggestion

		for i, item := range items {
			atomic.StoreInt64(&sortProgressCounter, int64(i+1))

			var title, url, currentPath string
			var tags []string
			var isFolder bool

			if item.IsFolder() {
				title = item.Folder.Name
				currentPath = a.store.GetFolderPath(item.Folder.ParentID)
				isFolder = true
			} else {
				title = item.Bookmark.Title
				url = item.Bookmark.URL
				currentPath = a.store.GetFolderPath(item.Bookmark.FolderID)
				tags = item.Bookmark.Tags
				isFolder = false
			}

			resp, err := client.SuggestSort(title, url, currentPath, tags, isFolder, context)
			if err != nil {
				continue // Skip items that fail
			}

			// Filter out items already in the best place
			if resp.FolderPath == currentPath {
				continue
			}

			// Filter out low confidence suggestions
			if resp.Confidence == "low" {
				continue
			}

			suggestions = append(suggestions, SortSuggestion{
				Item:          item,
				CurrentPath:   currentPath,
				SuggestedPath: resp.FolderPath,
				IsNewFolder:   resp.IsNewFolder,
				Processed:     false,
			})
		}

		// Store suggestions and complete
		return sortResultsMsg{suggestions: suggestions}
	}
}

// sortResultsMsg carries the final suggestions.
type sortResultsMsg struct {
	suggestions []SortSuggestion
}

// sortTickCmd returns a command that ticks every 100ms to update progress.
func sortTickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return sortTickMsg{}
	})
}
```

**Step 3: Verify it compiles**

Run: `go build ./...`
Expected: No errors

**Step 4: Commit**

```bash
git add internal/tui/app.go
git commit -m "feat(sort): add S keybinding and startSort handler"
```

---

## Task 6: Add Sort Message Handlers in Update

**Files:**
- Modify: `internal/tui/app.go`

**Step 1: Add handlers in the Update switch statement**

Find the `case cullProgressMsg:` handler and add the sort handlers nearby:

```go
	case sortTickMsg:
		if a.mode != ModeSortLoading {
			return a, nil
		}
		a.sort.Progress = int(atomic.LoadInt64(&sortProgressCounter))
		if a.sort.Progress < a.sort.Total {
			return a, sortTickCmd()
		}
		return a, nil

	case sortResultsMsg:
		a.sort.Suggestions = msg.suggestions
		if len(msg.suggestions) == 0 {
			a.mode = ModeNormal
			a.setStatus("All items already well-organized!")
			return a, nil
		}
		a.sort.Cursor = 0
		a.mode = ModeSortResults
		return a, nil
```

**Step 2: Verify it compiles**

Run: `go build ./...`
Expected: No errors

**Step 3: Commit**

```bash
git add internal/tui/app.go
git commit -m "feat(sort): add sort message handlers in Update"
```

---

## Task 7: Add Sort Results Key Handlers

**Files:**
- Modify: `internal/tui/app.go`

**Step 1: Add ModeSortResults key handling**

Find where `ModeCullResults` is handled in the key switch and add similar handling for sort. Add in the Update function's key handling:

```go
	if a.mode == ModeSortResults {
		switch msg.String() {
		case "j", "down":
			a.sortNextUnprocessed()
			return a, nil
		case "k", "up":
			a.sortPrevUnprocessed()
			return a, nil
		case "enter":
			return a.sortAcceptCurrent()
		case "s":
			a.sortSkipCurrent()
			return a, nil
		case "o":
			return a.sortOpenCurrent()
		case "e":
			return a.sortEditCurrent()
		case "d":
			return a.sortDeleteCurrent()
		case "q":
			a.mode = ModeNormal
			a.sort.Reset()
			return a, nil
		case "esc":
			a.mode = ModeNormal
			a.sort.Reset()
			return a, nil
		}
		return a, nil
	}
```

**Step 2: Add the helper methods**

```go
// sortNextUnprocessed moves cursor to next unprocessed suggestion.
func (a *App) sortNextUnprocessed() {
	for i := a.sort.Cursor + 1; i < len(a.sort.Suggestions); i++ {
		if !a.sort.Suggestions[i].Processed {
			a.sort.Cursor = i
			return
		}
	}
}

// sortPrevUnprocessed moves cursor to previous unprocessed suggestion.
func (a *App) sortPrevUnprocessed() {
	for i := a.sort.Cursor - 1; i >= 0; i-- {
		if !a.sort.Suggestions[i].Processed {
			a.sort.Cursor = i
			return
		}
	}
}

// sortAcceptCurrent moves the item to the suggested folder.
func (a *App) sortAcceptCurrent() (tea.Model, tea.Cmd) {
	sug := a.sort.CurrentSuggestion()
	if sug == nil || sug.Processed {
		return a, nil
	}

	// Get or create the target folder
	targetFolder, created := a.store.GetOrCreateFolderByPath(sug.SuggestedPath)
	var targetFolderID *string
	if targetFolder != nil {
		targetFolderID = &targetFolder.ID
	}

	// Move the item
	if sug.Item.IsFolder() {
		folder := a.store.GetFolderByID(sug.Item.Folder.ID)
		if folder != nil {
			folder.ParentID = targetFolderID
		}
	} else {
		bookmark := a.store.GetBookmarkByID(sug.Item.Bookmark.ID)
		if bookmark != nil {
			bookmark.FolderID = targetFolderID
		}
	}

	sug.Processed = true

	// Save changes
	if err := a.storage.Save(a.store); err != nil {
		a.setError("Failed to save: " + err.Error())
	} else {
		action := "Moved"
		if created {
			action = "Moved (created folder)"
		}
		a.setStatus(action + ": " + sug.Item.Title())
	}

	// Move to next or exit if done
	if a.sort.UnprocessedCount() == 0 {
		a.mode = ModeNormal
		a.refreshItems()
		a.sort.Reset()
		return a, nil
	}
	a.sortNextUnprocessed()
	return a, nil
}

// sortSkipCurrent marks the current suggestion as processed without moving.
func (a *App) sortSkipCurrent() {
	sug := a.sort.CurrentSuggestion()
	if sug == nil || sug.Processed {
		return
	}

	sug.Processed = true

	if a.sort.UnprocessedCount() == 0 {
		a.mode = ModeNormal
		a.sort.Reset()
		return
	}
	a.sortNextUnprocessed()
}

// sortOpenCurrent opens the current item's URL in browser.
func (a *App) sortOpenCurrent() (tea.Model, tea.Cmd) {
	sug := a.sort.CurrentSuggestion()
	if sug == nil || sug.Item.IsFolder() {
		return a, nil
	}

	return a, a.openURL(sug.Item.Bookmark.URL)
}

// sortEditCurrent switches to move mode for manual folder selection.
func (a *App) sortEditCurrent() (tea.Model, tea.Cmd) {
	sug := a.sort.CurrentSuggestion()
	if sug == nil || sug.Processed {
		return a, nil
	}

	// Set up move state with this item
	a.move.Reset()
	a.move.ItemsToMove = []Item{sug.Item}
	a.move.Folders = a.getAllFolderPaths()
	a.move.FilteredFolders = a.move.Folders
	a.move.FolderIdx = a.findMoveFolderIndex(sug.SuggestedPath)
	a.move.FilterInput.Focus()

	a.mode = ModeMove
	return a, nil
}

// sortDeleteCurrent deletes the current item.
func (a *App) sortDeleteCurrent() (tea.Model, tea.Cmd) {
	sug := a.sort.CurrentSuggestion()
	if sug == nil || sug.Processed {
		return a, nil
	}

	// Delete the item
	if sug.Item.IsFolder() {
		a.store.RemoveFolderByID(sug.Item.Folder.ID)
	} else {
		a.store.RemoveBookmarkByID(sug.Item.Bookmark.ID)
	}

	sug.Processed = true

	// Save changes
	if err := a.storage.Save(a.store); err != nil {
		a.setError("Failed to save: " + err.Error())
	} else {
		a.setStatus("Deleted: " + sug.Item.Title())
	}

	// Move to next or exit if done
	if a.sort.UnprocessedCount() == 0 {
		a.mode = ModeNormal
		a.refreshItems()
		a.sort.Reset()
		return a, nil
	}
	a.sortNextUnprocessed()
	return a, nil
}
```

**Step 3: Verify it compiles**

Run: `go build ./...`
Expected: No errors

**Step 4: Commit**

```bash
git add internal/tui/app.go
git commit -m "feat(sort): add sort results key handlers and actions"
```

---

## Task 8: Add Sort View Rendering

**Files:**
- Modify: `internal/tui/view.go`

**Step 1: Add cases to renderModal switch**

Find `case ModeCullInspect:` and add after it:

```go
	case ModeSortLoading:
		return a.renderSortLoading()
	case ModeSortResults:
		return a.renderSortResults()
```

**Step 2: Add renderSortLoading function**

Add after the cull render functions:

```go
// renderSortLoading renders the loading screen during AI analysis.
func (a App) renderSortLoading() string {
	modalWidth := layout.CalculateModalWidth(a.width, a.layoutConfig.Modal.DefaultWidthPercent, a.layoutConfig.Modal)

	accent := lipgloss.AdaptiveColor{Light: "#6B8E23", Dark: "#9ACD32"}
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.ThickBorder()).
		BorderForeground(accent).
		Padding(1, 2).
		Width(modalWidth)

	var content strings.Builder
	content.WriteString(a.styles.Title.Render("Auto-Sort"))
	content.WriteString("\n\n")
	content.WriteString(fmt.Sprintf("Analyzing %d items...\n\n", a.sort.Total))

	// Progress bar
	if a.sort.Total > 0 {
		progress := float64(a.sort.Progress) / float64(a.sort.Total)
		barWidth := modalWidth - 10
		filled := int(progress * float64(barWidth))
		empty := barWidth - filled

		bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)
		content.WriteString(bar + "\n\n")
		content.WriteString(fmt.Sprintf("[%d/%d]", a.sort.Progress, a.sort.Total))
	}

	modal := lipgloss.Place(
		a.width,
		a.height-3,
		lipgloss.Center,
		lipgloss.Center,
		modalStyle.Render(content.String()),
	)

	return lipgloss.JoinVertical(lipgloss.Left, modal, a.renderHelpBar())
}
```

**Step 3: Add renderSortResults function**

```go
// renderSortResults renders the list of suggested moves.
func (a App) renderSortResults() string {
	modalWidth := layout.CalculateModalWidth(a.width, a.layoutConfig.Modal.LargeWidthPercent, a.layoutConfig.Modal)

	accent := lipgloss.AdaptiveColor{Light: "#6B8E23", Dark: "#9ACD32"}
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.ThickBorder()).
		BorderForeground(accent).
		Padding(1, 2).
		Width(modalWidth)

	var content strings.Builder

	unprocessed := a.sort.UnprocessedCount()
	title := fmt.Sprintf("Auto-Sort Results (%d to move)", unprocessed)
	content.WriteString(a.styles.Title.Render(title))
	content.WriteString("\n\n")

	if len(a.sort.Suggestions) == 0 {
		content.WriteString(a.styles.Empty.Render("All items already well-organized!"))
	} else {
		// Filter to unprocessed suggestions for display
		var visibleSuggestions []int
		for i, sug := range a.sort.Suggestions {
			if !sug.Processed {
				visibleSuggestions = append(visibleSuggestions, i)
			}
		}

		maxVisible := 8
		itemWidth := modalWidth - 6

		// Find cursor position in visible list
		cursorVisibleIdx := 0
		for i, idx := range visibleSuggestions {
			if idx == a.sort.Cursor {
				cursorVisibleIdx = i
				break
			}
		}

		start, end := layout.CalculateVisibleListItems(maxVisible, cursorVisibleIdx, len(visibleSuggestions))

		for vi := start; vi < end; vi++ {
			idx := visibleSuggestions[vi]
			sug := a.sort.Suggestions[idx]
			isSelected := idx == a.sort.Cursor

			// Title line
			titleLine := sug.Item.Title()
			if len(titleLine) > itemWidth-2 {
				titleLine = titleLine[:itemWidth-5] + "..."
			}

			// Path line: /Current -> /Suggested
			pathLine := sug.CurrentPath + " -> " + sug.SuggestedPath
			if sug.IsNewFolder {
				pathLine += " (new)"
			}
			if len(pathLine) > itemWidth-4 {
				pathLine = pathLine[:itemWidth-7] + "..."
			}

			if isSelected {
				content.WriteString(a.styles.ItemSelected.Render("> " + titleLine))
				content.WriteString("\n")
				content.WriteString(a.styles.URL.Render("  " + pathLine))
				content.WriteString("\n\n")
			} else {
				content.WriteString("  " + titleLine)
				content.WriteString("\n")
				content.WriteString(a.styles.Empty.Render("  " + pathLine))
				content.WriteString("\n\n")
			}
		}
	}

	modal := lipgloss.Place(
		a.width,
		a.height-3,
		lipgloss.Center,
		lipgloss.Center,
		modalStyle.Render(content.String()),
	)

	return lipgloss.JoinVertical(lipgloss.Left, modal, a.renderHelpBar())
}
```

**Step 4: Verify it compiles**

Run: `go build ./...`
Expected: No errors

**Step 5: Commit**

```bash
git add internal/tui/view.go
git commit -m "feat(sort): add sort loading and results view rendering"
```

---

## Task 9: Add Sort Hints

**Files:**
- Modify: `internal/tui/hints.go`

**Step 1: Add cases to getHints switch**

Find `case ModeCullInspect:` and add after it:

```go
	case ModeSortLoading:
		return a.getSortLoadingHints()
	case ModeSortResults:
		return a.getSortResultsHints()
```

**Step 2: Add hint functions**

Add after the cull hint functions:

```go
// getSortLoadingHints returns hints for ModeSortLoading.
func (a App) getSortLoadingHints() HintSet {
	return HintSet{
		System: []Hint{
			{Key: "Esc", Desc: "cancel"},
		},
	}
}

// getSortResultsHints returns hints for ModeSortResults.
func (a App) getSortResultsHints() HintSet {
	return HintSet{
		Nav: []Hint{
			{Key: "j/k", Desc: "move"},
		},
		Action: []Hint{
			{Key: "Enter", Desc: "accept"},
			{Key: "s", Desc: "skip"},
			{Key: "o", Desc: "open"},
			{Key: "e", Desc: "edit"},
			{Key: "d", Desc: "del"},
		},
		System: []Hint{
			{Key: "q/Esc", Desc: "quit"},
		},
	}
}
```

**Step 3: Verify it compiles**

Run: `go build ./...`
Expected: No errors

**Step 4: Commit**

```bash
git add internal/tui/hints.go
git commit -m "feat(sort): add sort mode keyboard hints"
```

---

## Task 10: Add S Key to Normal Mode Hints

**Files:**
- Modify: `internal/tui/hints.go`

**Step 1: Find getNormalHints and add S hint**

Look for the Action hints in getNormalHints and add:

```go
{Key: "S", Desc: "sort"},
```

Add it near other AI-related hints like `i` for quick add.

**Step 2: Verify it compiles**

Run: `go build ./...`
Expected: No errors

**Step 3: Commit**

```bash
git add internal/tui/hints.go
git commit -m "feat(sort): add S keybinding hint to normal mode"
```

---

## Task 11: Handle Cancel During Loading

**Files:**
- Modify: `internal/tui/app.go`

**Step 1: Add escape handling for ModeSortLoading**

Find where ModeCullLoading escape is handled and add similar for sort:

```go
	if a.mode == ModeSortLoading {
		switch msg.String() {
		case "esc", "q":
			a.mode = ModeNormal
			a.sort.Reset()
			return a, nil
		}
		return a, nil
	}
```

**Step 2: Verify it compiles**

Run: `go build ./...`
Expected: No errors

**Step 3: Commit**

```bash
git add internal/tui/app.go
git commit -m "feat(sort): add cancel handling during sort loading"
```

---

## Task 12: Integration Test - Manual Testing

**Step 1: Build and run**

```bash
go build -o bm ./cmd/bm && ./bm
```

**Step 2: Test scenarios**

1. Navigate to a folder with bookmarks, press `S`
2. Verify loading screen shows with progress
3. Verify results screen shows suggestions
4. Test `Enter` to accept a move
5. Test `s` to skip
6. Test `d` to delete
7. Test `e` to edit (should open move picker)
8. Test `o` to open URL
9. Test `Esc` and `q` to exit
10. Test on a single bookmark (not folder)
11. Test on empty folder

**Step 3: Fix any issues found**

If issues are found, fix and commit with descriptive message.

**Step 4: Final commit if all works**

```bash
git add -A
git commit -m "feat(sort): complete auto-sort feature implementation"
```

---

## Summary

The implementation adds:
- **2 new modes**: `ModeSortLoading`, `ModeSortResults`
- **New state**: `SortState` with `SortSuggestion` slice
- **New AI function**: `SuggestSort` for getting relocation suggestions
- **Keybinding**: `S` triggers sort on current item
- **Actions**: Accept, skip, open, edit, delete per suggestion
- **Views**: Loading progress bar, results list with current/suggested paths
