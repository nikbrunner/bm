# Auto-Sort Feature Design

AI-powered bookmark and folder organization with interactive review.

## Overview

Press `S` on any item to get AI-powered sorting suggestions. On a folder, it recursively analyzes all items inside and suggests better locations. On a single item, it suggests a destination for just that one.

## User Flow

1. Navigate to a folder (e.g., `/Unsorted`) or item
2. Press `S` to trigger auto-sort
3. See loading screen with progress ("Analyzing 12/47...")
4. Results screen shows list of suggested moves:
   ```
   Auto-Sort Results (8 items to move)

   > React Hooks Guide
     /Unsorted -> /Dev/Frontend/React

     TypeScript Handbook
     /Unsorted -> /Dev/Frontend/TypeScript (new)

     PostgreSQL Cheatsheet
     /Unsorted -> /Dev/Databases
   ```
5. For each item: accept (`Enter`), skip (`s`), open (`o`), edit destination (`e`), or delete (`d`)
6. After processing all items (or pressing `q`), return to normal mode

## Key Behaviors

- Items already in their "best" folder are filtered out - only moves shown
- New folder suggestions marked with `(new)` suffix
- Accepting a "new folder" suggestion creates the folder automatically before moving
- Both bookmarks AND folders can receive relocation suggestions

## State & Modes

### New Modes

```
ModeSortLoading   // Analyzing items, showing progress
ModeSortResults   // List of suggested moves
```

### SortState Structure

```go
type SortState struct {
    SourceFolderID *string          // Folder being sorted (nil if single item)
    SourceItem     *Item            // Single item if sorting one bookmark/folder
    Suggestions    []SortSuggestion
    Cursor         int              // Current selection
    Progress       int              // Items analyzed so far
    Total          int              // Total items to analyze
}

type SortSuggestion struct {
    Item          Item     // Can be bookmark OR folder
    CurrentPath   string   // Where it is now
    SuggestedPath string   // Where AI suggests
    IsNewFolder   bool     // True if destination folder doesn't exist yet
    Processed     bool     // True after accept/skip/delete
}
```

### Keybindings in ModeSortResults

| Key | Action |
|-----|--------|
| `j/k` | Navigate suggestions |
| `Enter` | Accept - move to suggested folder |
| `s` | Skip - leave where it is, go to next |
| `o` | Open URL in browser (bookmarks only) |
| `e` | Edit - pick different destination manually |
| `d` | Delete item |
| `Esc` | Back (to previous screen / normal mode) |
| `q` | Quit (exit sort mode entirely) |

## AI Integration

### New AI Function

```go
func (c *Client) SuggestSort(item Item, context string) (*SortResponse, error)

type SortResponse struct {
    FolderPath  string `json:"folderPath"`
    IsNewFolder bool   `json:"isNewFolder"`
    Confidence  string `json:"confidence"` // "high", "medium", "low"
}
```

### Prompt Strategy

```
Analyze this bookmark/folder and suggest the best parent folder for it.

Item:
- Title: {title}
- URL: {url} (if bookmark)
- Current folder: {currentPath}
- Tags: {tags} (if bookmark)
- Contents: {summary of contents} (if folder)

{context from BuildContext - existing folders with samples}

Instructions:
- Prefer existing folders when they fit well
- Only suggest a new folder path if nothing existing is appropriate
- Set isNewFolder=true only when suggesting a folder that doesn't exist
- If current location is already optimal, return the current path
- Confidence: "high" if clear match, "medium" if reasonable, "low" if uncertain
```

### Filtering Logic

After getting response, filter out suggestions where:
- `SuggestedPath == CurrentPath` (already in best place)
- Optionally: show `Confidence == "low"` items dimmed

### Batching

Process items sequentially (not parallel) to avoid API rate limits. Show progress as each completes.

## UI Rendering

### Loading View (ModeSortLoading)

```
+-------------------------------------+
|  Auto-Sort                          |
|                                     |
|  Analyzing items...                 |
|  ████████░░░░░░░░░░  12/47          |
|                                     |
|  Current: React Hooks Guide         |
|                                     |
+-------------------------------------+
                          [Esc] cancel
```

### Results View (ModeSortResults)

```
+-------------------------------------+
|  Auto-Sort Results (8 to move)      |
|                                     |
|  > React Hooks Guide                |
|    /Unsorted -> /Dev/Frontend/React |
|                                     |
|    TypeScript Handbook              |
|    /Unsorted -> /Dev/TypeScript (new)|
|                                     |
|    PostgreSQL Cheatsheet            |
|    /Unsorted -> /Dev/Databases      |
|                                     |
+-------------------------------------+
 [j/k] move [Enter] accept [s] skip [o] open [e] edit [d] del [q] quit
```

### Visual Details

- Current item highlighted with `>` prefix and selection style
- New folders shown with `(new)` suffix in dimmed/accent color
- Processed items removed from list
- Empty state: "All items already well-organized!" when no suggestions

## Edge Cases

- **No API key**: Show error message, same as quick-add flow
- **Empty folder**: "No items to sort" message, return to normal mode
- **All items already optimal**: "All items already well-organized!" - immediate exit
- **API failure mid-sort**: Keep partial results, show error for failed item, continue with rest
- **Nested new folders**: If AI suggests `/Dev/Rust/Async` but `/Dev/Rust` doesn't exist, create the full path

## Folder Creation Logic

When accepting a suggestion with `IsNewFolder=true`:
1. Parse the path into segments
2. Walk down, creating any missing folders
3. Move item to final folder
4. New folders appear immediately in the browser

## Cancel Behavior

- `Esc`: Go back to previous screen (or normal mode if at top level)
- `q`: Quit sort mode entirely

Already-accepted moves are committed. Unprocessed items stay in their original locations.

## Persistence

No caching for sort results (unlike cull). Each sort is fresh since the AI suggestions depend on current folder structure.
