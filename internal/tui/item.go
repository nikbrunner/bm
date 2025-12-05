package tui

import "github.com/nikbrunner/bm/internal/model"

// ItemKind distinguishes between folders and bookmarks in a list.
type ItemKind int

const (
	ItemFolder ItemKind = iota
	ItemBookmark
)

// Item represents either a folder or bookmark in the list.
type Item struct {
	Kind     ItemKind
	Folder   *model.Folder
	Bookmark *model.Bookmark
}

// ID returns the item's ID regardless of type.
func (i Item) ID() string {
	if i.Kind == ItemFolder {
		return i.Folder.ID
	}
	return i.Bookmark.ID
}

// Title returns a display title for the item.
func (i Item) Title() string {
	if i.Kind == ItemFolder {
		return i.Folder.Name
	}
	return i.Bookmark.Title
}

// IsFolder returns true if this item is a folder.
func (i Item) IsFolder() bool {
	return i.Kind == ItemFolder
}
