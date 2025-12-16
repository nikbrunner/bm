# Pinned Items Quick Access Design

## Overview

Add number key shortcuts (1-9) for instant access to pinned items, plus reorder capability with J/K.

## Constraints

- **Max 9 pinned items** - Enforced limit; attempting to pin a 10th item shows error message
- **Scoped to pinned pane only** - Number keys and reorder keys only work when `focusedPane == PanePinned`

## Keybindings

| Key | Action |
|-----|--------|
| `1`-`9` | Immediately open pinned item at that position |
| `J` | Move current item down |
| `K` | Move current item up |

## Behavior

### Number Keys (1-9)
- Folder item → navigate into it (switch to browser pane, set folder as current)
- Bookmark item → open URL in default browser
- Number exceeds item count → no-op or brief message

### Reorder Keys (J/K)
- Swap current item with adjacent item
- Update cursor to follow the moved item
- Persist new order to storage immediately

## Visual Display

Remove redundant star icon in pinned pane. Show bracketed numbers:

```
[1] GitHub
[2] Other Favourite
[3] Dev Tools
```

## Implementation Notes

### Storage
- Add `PinOrder int` field to both `Folder` and `Bookmark` models
- On reorder: update `PinOrder` values and save
- On pin: assign next available order (1-9)
- On unpin: clear `PinOrder`, recompact remaining items

### Enforcement
- `TogglePin` must check count before pinning
- Return error or show message if at limit
