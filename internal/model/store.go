package model

// Store holds all bookmarks and folders.
type Store struct {
	Folders   []Folder   `json:"folders"`
	Bookmarks []Bookmark `json:"bookmarks"`
}

// NewStore creates an empty Store with initialized slices.
func NewStore() *Store {
	return &Store{
		Folders:   []Folder{},
		Bookmarks: []Bookmark{},
	}
}

// GetFoldersInFolder returns folders with the given parent ID.
// Pass nil for root level folders.
func (s *Store) GetFoldersInFolder(parentID *string) []Folder {
	var result []Folder
	for _, f := range s.Folders {
		if ptrEqual(f.ParentID, parentID) {
			result = append(result, f)
		}
	}
	return result
}

// GetBookmarksInFolder returns bookmarks in the given folder.
// Pass nil for root level bookmarks.
func (s *Store) GetBookmarksInFolder(folderID *string) []Bookmark {
	var result []Bookmark
	for _, b := range s.Bookmarks {
		if ptrEqual(b.FolderID, folderID) {
			result = append(result, b)
		}
	}
	return result
}

// GetFolderByID finds a folder by ID, returns nil if not found.
func (s *Store) GetFolderByID(id string) *Folder {
	for i := range s.Folders {
		if s.Folders[i].ID == id {
			return &s.Folders[i]
		}
	}
	return nil
}

// GetBookmarkByID finds a bookmark by ID, returns nil if not found.
func (s *Store) GetBookmarkByID(id string) *Bookmark {
	for i := range s.Bookmarks {
		if s.Bookmarks[i].ID == id {
			return &s.Bookmarks[i]
		}
	}
	return nil
}

// ptrEqual compares two string pointers for equality.
func ptrEqual(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}
