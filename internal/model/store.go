package model

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// MaxPinnedItems is the maximum number of pinned items allowed.
const MaxPinnedItems = 9

// ErrMaxPinnedItems is returned when trying to pin more than MaxPinnedItems.
var ErrMaxPinnedItems = errors.New("maximum pinned items reached (9)")

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

// HasBookmarkURL checks if a bookmark with the given URL already exists.
func (s *Store) HasBookmarkURL(url string) bool {
	for _, b := range s.Bookmarks {
		if b.URL == url {
			return true
		}
	}
	return false
}

// ImportMerge imports folders and bookmarks, skipping duplicate URLs.
// Returns the count of bookmarks added and skipped.
func (s *Store) ImportMerge(folders []Folder, bookmarks []Bookmark) (added, skipped int) {
	// Build a map from imported folder IDs to actual IDs (may be remapped)
	folderIDMap := make(map[string]string)

	// Process folders - reuse existing folders with same name at same level
	for _, f := range folders {
		// Remap parent ID if it was imported
		var actualParentID *string
		if f.ParentID != nil {
			if remapped, ok := folderIDMap[*f.ParentID]; ok {
				actualParentID = &remapped
			} else {
				actualParentID = f.ParentID
			}
		}

		// Check if folder with same name exists at same parent level
		existingFolder := s.findFolderByNameAndParent(f.Name, actualParentID)
		if existingFolder != nil {
			// Reuse existing folder
			folderIDMap[f.ID] = existingFolder.ID
		} else {
			// Create new folder with remapped parent
			newFolder := NewFolder(NewFolderParams{
				Name:     f.Name,
				ParentID: actualParentID,
			})
			s.Folders = append(s.Folders, newFolder)
			folderIDMap[f.ID] = newFolder.ID
		}
	}

	// Process bookmarks - skip duplicates by URL
	for _, b := range bookmarks {
		if s.HasBookmarkURL(b.URL) {
			skipped++
			continue
		}

		// Remap folder ID if it was imported
		var actualFolderID *string
		if b.FolderID != nil {
			if remapped, ok := folderIDMap[*b.FolderID]; ok {
				actualFolderID = &remapped
			} else {
				actualFolderID = b.FolderID
			}
		}

		// Create new bookmark with remapped folder ID
		newBookmark := Bookmark{
			ID:        GenerateUUID(),
			Title:     b.Title,
			URL:       b.URL,
			FolderID:  actualFolderID,
			Tags:      b.Tags,
			CreatedAt: b.CreatedAt,
			VisitedAt: b.VisitedAt,
		}
		s.Bookmarks = append(s.Bookmarks, newBookmark)
		added++
	}

	return added, skipped
}

// findFolderByNameAndParent finds a folder by name and parent ID.
func (s *Store) findFolderByNameAndParent(name string, parentID *string) *Folder {
	for i := range s.Folders {
		if s.Folders[i].Name == name && ptrEqual(s.Folders[i].ParentID, parentID) {
			return &s.Folders[i]
		}
	}
	return nil
}

// GetPinnedBookmarks returns all bookmarks with Pinned=true, sorted by PinOrder.
func (s *Store) GetPinnedBookmarks() []Bookmark {
	var result []Bookmark
	for _, b := range s.Bookmarks {
		if b.Pinned {
			result = append(result, b)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].PinOrder < result[j].PinOrder
	})
	return result
}

// GetPinnedFolders returns all folders with Pinned=true, sorted by PinOrder.
func (s *Store) GetPinnedFolders() []Folder {
	var result []Folder
	for _, f := range s.Folders {
		if f.Pinned {
			result = append(result, f)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].PinOrder < result[j].PinOrder
	})
	return result
}

// CountPinnedItems returns the total count of pinned items (folders + bookmarks).
func (s *Store) CountPinnedItems() int {
	count := 0
	for _, f := range s.Folders {
		if f.Pinned {
			count++
		}
	}
	for _, b := range s.Bookmarks {
		if b.Pinned {
			count++
		}
	}
	return count
}

// nextPinOrder returns the next available pin order (1-9).
func (s *Store) nextPinOrder() int {
	maxOrder := 0
	for _, f := range s.Folders {
		if f.Pinned && f.PinOrder > maxOrder {
			maxOrder = f.PinOrder
		}
	}
	for _, b := range s.Bookmarks {
		if b.Pinned && b.PinOrder > maxOrder {
			maxOrder = b.PinOrder
		}
	}
	return maxOrder + 1
}

// TogglePinBookmark toggles the Pinned field of a bookmark by ID.
// Returns ErrMaxPinnedItems if already at limit when pinning.
// Returns an error if the bookmark is not found.
func (s *Store) TogglePinBookmark(id string) error {
	for i := range s.Bookmarks {
		if s.Bookmarks[i].ID == id {
			if s.Bookmarks[i].Pinned {
				// Unpinning - clear PinOrder and recompact
				oldOrder := s.Bookmarks[i].PinOrder
				s.Bookmarks[i].Pinned = false
				s.Bookmarks[i].PinOrder = 0
				s.recompactPinOrders(oldOrder)
			} else {
				// Pinning - check limit first
				if s.CountPinnedItems() >= MaxPinnedItems {
					return ErrMaxPinnedItems
				}
				s.Bookmarks[i].Pinned = true
				s.Bookmarks[i].PinOrder = s.nextPinOrder()
			}
			return nil
		}
	}
	return fmt.Errorf("bookmark not found: %s", id)
}

// TogglePinFolder toggles the Pinned field of a folder by ID.
// Returns ErrMaxPinnedItems if already at limit when pinning.
// Returns an error if the folder is not found.
func (s *Store) TogglePinFolder(id string) error {
	for i := range s.Folders {
		if s.Folders[i].ID == id {
			if s.Folders[i].Pinned {
				// Unpinning - clear PinOrder and recompact
				oldOrder := s.Folders[i].PinOrder
				s.Folders[i].Pinned = false
				s.Folders[i].PinOrder = 0
				s.recompactPinOrders(oldOrder)
			} else {
				// Pinning - check limit first
				if s.CountPinnedItems() >= MaxPinnedItems {
					return ErrMaxPinnedItems
				}
				s.Folders[i].Pinned = true
				s.Folders[i].PinOrder = s.nextPinOrder()
			}
			return nil
		}
	}
	return fmt.Errorf("folder not found: %s", id)
}

// recompactPinOrders decrements all PinOrders greater than removedOrder.
func (s *Store) recompactPinOrders(removedOrder int) {
	for i := range s.Folders {
		if s.Folders[i].Pinned && s.Folders[i].PinOrder > removedOrder {
			s.Folders[i].PinOrder--
		}
	}
	for i := range s.Bookmarks {
		if s.Bookmarks[i].Pinned && s.Bookmarks[i].PinOrder > removedOrder {
			s.Bookmarks[i].PinOrder--
		}
	}
}

// SwapPinOrders swaps the PinOrder of two pinned items by their current orders.
func (s *Store) SwapPinOrders(order1, order2 int) {
	// Find and swap folders
	for i := range s.Folders {
		if s.Folders[i].Pinned {
			if s.Folders[i].PinOrder == order1 {
				s.Folders[i].PinOrder = order2
			} else if s.Folders[i].PinOrder == order2 {
				s.Folders[i].PinOrder = order1
			}
		}
	}
	// Find and swap bookmarks
	for i := range s.Bookmarks {
		if s.Bookmarks[i].Pinned {
			if s.Bookmarks[i].PinOrder == order1 {
				s.Bookmarks[i].PinOrder = order2
			} else if s.Bookmarks[i].PinOrder == order2 {
				s.Bookmarks[i].PinOrder = order1
			}
		}
	}
}

// GetFolderByPath finds a folder by its full path (e.g., "/Dev/React").
// Returns nil if not found.
func (s *Store) GetFolderByPath(path string) *Folder {
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		return nil
	}

	parts := strings.Split(path, "/")
	var currentParentID *string

	for i, name := range parts {
		folder := s.findFolderByNameAndParent(name, currentParentID)
		if folder == nil {
			return nil
		}
		if i == len(parts)-1 {
			return folder
		}
		currentParentID = &folder.ID
	}

	return nil
}

// GetOrCreateFolderByPath finds or creates a folder by its full path.
// Creates any missing intermediate folders.
// Returns the folder and whether any folders were created.
func (s *Store) GetOrCreateFolderByPath(path string) (*Folder, bool) {
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		return nil, false
	}

	parts := strings.Split(path, "/")
	var currentParentID *string
	created := false

	for i, name := range parts {
		folder := s.findFolderByNameAndParent(name, currentParentID)
		if folder == nil {
			// Create the folder
			newFolder := NewFolder(NewFolderParams{
				Name:     name,
				ParentID: currentParentID,
			})
			s.AddFolder(newFolder)
			created = true

			// Get the newly added folder
			folder = s.findFolderByNameAndParent(name, currentParentID)
		}

		if i == len(parts)-1 {
			return folder, created
		}
		currentParentID = &folder.ID
	}

	return nil, false
}

// GetFolderPath returns the full path string for a folder (e.g., "/Dev/React").
func (s *Store) GetFolderPath(folderID *string) string {
	if folderID == nil {
		return "/"
	}

	var parts []string
	currentID := folderID

	for currentID != nil {
		folder := s.GetFolderByID(*currentID)
		if folder == nil {
			break
		}
		parts = append([]string{folder.Name}, parts...)
		currentID = folder.ParentID
	}

	if len(parts) == 0 {
		return "/"
	}
	return "/" + strings.Join(parts, "/")
}
