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

// AddFolder appends a folder to the store.
func (s *Store) AddFolder(f Folder) {
	s.Folders = append(s.Folders, f)
}

// AddBookmark appends a bookmark to the store.
func (s *Store) AddBookmark(b Bookmark) {
	s.Bookmarks = append(s.Bookmarks, b)
}

// RemoveFolderByID removes a folder by ID. Returns true if found and removed.
func (s *Store) RemoveFolderByID(id string) bool {
	for i, f := range s.Folders {
		if f.ID == id {
			s.Folders = append(s.Folders[:i], s.Folders[i+1:]...)
			return true
		}
	}
	return false
}

// RemoveBookmarkByID removes a bookmark by ID. Returns true if found and removed.
func (s *Store) RemoveBookmarkByID(id string) bool {
	for i, b := range s.Bookmarks {
		if b.ID == id {
			s.Bookmarks = append(s.Bookmarks[:i], s.Bookmarks[i+1:]...)
			return true
		}
	}
	return false
}

// InsertFolderAt inserts a folder at a specific index within folders of the same parent.
// The index is relative to folders with the same parentID.
func (s *Store) InsertFolderAt(f Folder, index int) {
	// Find all folders with same parent and their positions in the main slice
	var positions []int
	for i, folder := range s.Folders {
		if ptrEqual(folder.ParentID, f.ParentID) {
			positions = append(positions, i)
		}
	}

	if len(positions) == 0 || index >= len(positions) {
		// Just append
		s.Folders = append(s.Folders, f)
		return
	}

	// Insert at the global position corresponding to the local index
	globalIdx := positions[index]
	s.Folders = append(s.Folders[:globalIdx], append([]Folder{f}, s.Folders[globalIdx:]...)...)
}

// InsertBookmarkAt inserts a bookmark at a specific index within bookmarks of the same folder.
func (s *Store) InsertBookmarkAt(b Bookmark, index int) {
	// Find all bookmarks with same folder and their positions
	var positions []int
	for i, bookmark := range s.Bookmarks {
		if ptrEqual(bookmark.FolderID, b.FolderID) {
			positions = append(positions, i)
		}
	}

	if len(positions) == 0 || index >= len(positions) {
		// Just append
		s.Bookmarks = append(s.Bookmarks, b)
		return
	}

	// Insert at the global position
	globalIdx := positions[index]
	s.Bookmarks = append(s.Bookmarks[:globalIdx], append([]Bookmark{b}, s.Bookmarks[globalIdx:]...)...)
}
