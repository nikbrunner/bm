package model_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/nikbrunner/bm/internal/model"
)

// Helper functions for pointers
func stringPtr(s string) *string     { return &s }
func timePtr(t time.Time) *time.Time { return &t }

func TestBookmark_JSONSerialization(t *testing.T) {
	tests := []struct {
		name     string
		bookmark model.Bookmark
	}{
		{
			name: "bookmark with all fields",
			bookmark: model.Bookmark{
				ID:        "b1",
				Title:     "TanStack Router",
				URL:       "https://tanstack.com/router",
				FolderID:  stringPtr("f1"),
				Tags:      []string{"react", "routing"},
				CreatedAt: time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
				VisitedAt: timePtr(time.Date(2025, 1, 20, 14, 22, 0, 0, time.UTC)),
			},
		},
		{
			name: "root level bookmark (no folder)",
			bookmark: model.Bookmark{
				ID:        "b2",
				Title:     "Hacker News",
				URL:       "https://news.ycombinator.com",
				FolderID:  nil,
				Tags:      []string{},
				CreatedAt: time.Date(2025, 1, 10, 8, 0, 0, 0, time.UTC),
				VisitedAt: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal
			data, err := json.Marshal(tt.bookmark)
			if err != nil {
				t.Fatalf("failed to marshal: %v", err)
			}

			// Unmarshal
			var got model.Bookmark
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("failed to unmarshal: %v", err)
			}

			// Compare key fields
			if got.ID != tt.bookmark.ID {
				t.Errorf("ID mismatch: got %q, want %q", got.ID, tt.bookmark.ID)
			}
			if got.Title != tt.bookmark.Title {
				t.Errorf("Title mismatch: got %q, want %q", got.Title, tt.bookmark.Title)
			}
			if got.URL != tt.bookmark.URL {
				t.Errorf("URL mismatch: got %q, want %q", got.URL, tt.bookmark.URL)
			}
		})
	}
}

func TestFolder_JSONSerialization(t *testing.T) {
	tests := []struct {
		name   string
		folder model.Folder
	}{
		{
			name: "root level folder",
			folder: model.Folder{
				ID:       "f1",
				Name:     "Development",
				ParentID: nil,
			},
		},
		{
			name: "nested folder",
			folder: model.Folder{
				ID:       "f2",
				Name:     "React",
				ParentID: stringPtr("f1"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.folder)
			if err != nil {
				t.Fatalf("failed to marshal: %v", err)
			}

			var got model.Folder
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("failed to unmarshal: %v", err)
			}

			if got.ID != tt.folder.ID {
				t.Errorf("ID mismatch: got %q, want %q", got.ID, tt.folder.ID)
			}
			if got.Name != tt.folder.Name {
				t.Errorf("Name mismatch: got %q, want %q", got.Name, tt.folder.Name)
			}
		})
	}
}

func TestStore_GetFoldersInFolder(t *testing.T) {
	store := model.Store{
		Folders: []model.Folder{
			{ID: "f1", Name: "Development", ParentID: nil},
			{ID: "f2", Name: "React", ParentID: stringPtr("f1")},
			{ID: "f3", Name: "Design", ParentID: nil},
			{ID: "f4", Name: "Node", ParentID: stringPtr("f1")},
		},
		Bookmarks: []model.Bookmark{},
	}

	// Test root level folders
	rootFolders := store.GetFoldersInFolder(nil)
	if len(rootFolders) != 2 {
		t.Errorf("expected 2 root folders, got %d", len(rootFolders))
	}

	// Test nested folders
	f1ID := "f1"
	nestedFolders := store.GetFoldersInFolder(&f1ID)
	if len(nestedFolders) != 2 {
		t.Errorf("expected 2 nested folders in f1, got %d", len(nestedFolders))
	}

	// Test empty folder
	f3ID := "f3"
	emptyFolders := store.GetFoldersInFolder(&f3ID)
	if len(emptyFolders) != 0 {
		t.Errorf("expected 0 folders in f3, got %d", len(emptyFolders))
	}
}

func TestStore_GetBookmarksInFolder(t *testing.T) {
	f1ID := "f1"
	store := model.Store{
		Folders: []model.Folder{},
		Bookmarks: []model.Bookmark{
			{ID: "b1", Title: "Root Bookmark", URL: "https://example.com", FolderID: nil},
			{ID: "b2", Title: "Nested Bookmark", URL: "https://example.org", FolderID: &f1ID},
			{ID: "b3", Title: "Another Root", URL: "https://example.net", FolderID: nil},
		},
	}

	rootBookmarks := store.GetBookmarksInFolder(nil)
	if len(rootBookmarks) != 2 {
		t.Errorf("expected 2 root bookmarks, got %d", len(rootBookmarks))
	}

	nestedBookmarks := store.GetBookmarksInFolder(&f1ID)
	if len(nestedBookmarks) != 1 {
		t.Errorf("expected 1 nested bookmark, got %d", len(nestedBookmarks))
	}
}

func TestStore_GetFolderByID(t *testing.T) {
	store := model.Store{
		Folders: []model.Folder{
			{ID: "f1", Name: "Development", ParentID: nil},
			{ID: "f2", Name: "React", ParentID: stringPtr("f1")},
		},
		Bookmarks: []model.Bookmark{},
	}

	// Test existing folder
	folder := store.GetFolderByID("f1")
	if folder == nil {
		t.Fatal("expected to find folder f1")
	}
	if folder.Name != "Development" {
		t.Errorf("expected name 'Development', got %q", folder.Name)
	}

	// Test non-existing folder
	notFound := store.GetFolderByID("nonexistent")
	if notFound != nil {
		t.Error("expected nil for nonexistent folder")
	}
}

// === Import Merge Tests ===

func TestStore_HasBookmarkURL(t *testing.T) {
	store := model.Store{
		Folders: []model.Folder{},
		Bookmarks: []model.Bookmark{
			{ID: "b1", Title: "Example", URL: "https://example.com", FolderID: nil},
		},
	}

	if !store.HasBookmarkURL("https://example.com") {
		t.Error("expected to find existing URL")
	}

	if store.HasBookmarkURL("https://notfound.com") {
		t.Error("should not find non-existing URL")
	}
}

func TestStore_ImportMerge_SkipsDuplicateURLs(t *testing.T) {
	store := model.Store{
		Folders: []model.Folder{},
		Bookmarks: []model.Bookmark{
			{ID: "existing", Title: "Existing", URL: "https://example.com", FolderID: nil},
		},
	}

	newFolders := []model.Folder{}
	newBookmarks := []model.Bookmark{
		{ID: "new1", Title: "Duplicate", URL: "https://example.com", FolderID: nil}, // should skip
		{ID: "new2", Title: "New Site", URL: "https://newsite.com", FolderID: nil},  // should add
	}

	added, skipped := store.ImportMerge(newFolders, newBookmarks)

	if added != 1 {
		t.Errorf("expected 1 added, got %d", added)
	}
	if skipped != 1 {
		t.Errorf("expected 1 skipped, got %d", skipped)
	}

	// Should have 2 bookmarks total (1 existing + 1 new)
	if len(store.Bookmarks) != 2 {
		t.Errorf("expected 2 bookmarks, got %d", len(store.Bookmarks))
	}
}

func TestStore_ImportMerge_ReusesFolderByName(t *testing.T) {
	existingFolderID := "existing-folder"
	store := model.Store{
		Folders: []model.Folder{
			{ID: existingFolderID, Name: "Development", ParentID: nil},
		},
		Bookmarks: []model.Bookmark{},
	}

	// Import a folder with same name at same level (root) - should reuse existing
	newFolders := []model.Folder{
		{ID: "imported-folder", Name: "Development", ParentID: nil},
	}
	// Bookmark pointing to the imported folder ID
	newBookmarks := []model.Bookmark{
		{ID: "b1", Title: "New Bookmark", URL: "https://new.com", FolderID: stringPtr("imported-folder")},
	}

	store.ImportMerge(newFolders, newBookmarks)

	// Should still have just 1 folder (reused existing)
	if len(store.Folders) != 1 {
		t.Errorf("expected 1 folder (reused), got %d", len(store.Folders))
	}

	// The imported bookmark should be remapped to existing folder ID
	if len(store.Bookmarks) != 1 {
		t.Fatalf("expected 1 bookmark, got %d", len(store.Bookmarks))
	}
	if store.Bookmarks[0].FolderID == nil || *store.Bookmarks[0].FolderID != existingFolderID {
		t.Errorf("bookmark should be in existing folder %s, got %v", existingFolderID, store.Bookmarks[0].FolderID)
	}
}

func TestStore_ImportMerge_CountsCorrectly(t *testing.T) {
	store := model.Store{
		Folders:   []model.Folder{},
		Bookmarks: []model.Bookmark{},
	}

	newFolders := []model.Folder{
		{ID: "f1", Name: "Folder1", ParentID: nil},
		{ID: "f2", Name: "Folder2", ParentID: nil},
	}
	newBookmarks := []model.Bookmark{
		{ID: "b1", Title: "Bookmark1", URL: "https://one.com", FolderID: nil},
		{ID: "b2", Title: "Bookmark2", URL: "https://two.com", FolderID: nil},
		{ID: "b3", Title: "Bookmark3", URL: "https://three.com", FolderID: nil},
	}

	added, skipped := store.ImportMerge(newFolders, newBookmarks)

	if added != 3 {
		t.Errorf("expected 3 bookmarks added, got %d", added)
	}
	if skipped != 0 {
		t.Errorf("expected 0 skipped, got %d", skipped)
	}
	if len(store.Folders) != 2 {
		t.Errorf("expected 2 folders, got %d", len(store.Folders))
	}
	if len(store.Bookmarks) != 3 {
		t.Errorf("expected 3 bookmarks, got %d", len(store.Bookmarks))
	}
}

// === Pinned Items Tests ===

func TestStore_GetPinnedBookmarks(t *testing.T) {
	store := model.Store{
		Folders: []model.Folder{},
		Bookmarks: []model.Bookmark{
			{ID: "b1", Title: "Pinned One", URL: "https://one.com", FolderID: nil, Pinned: true},
			{ID: "b2", Title: "Not Pinned", URL: "https://two.com", FolderID: nil, Pinned: false},
			{ID: "b3", Title: "Pinned Two", URL: "https://three.com", FolderID: nil, Pinned: true},
		},
	}

	pinned := store.GetPinnedBookmarks()
	if len(pinned) != 2 {
		t.Errorf("expected 2 pinned bookmarks, got %d", len(pinned))
	}

	// Verify correct bookmarks returned
	ids := make(map[string]bool)
	for _, b := range pinned {
		ids[b.ID] = true
	}
	if !ids["b1"] || !ids["b3"] {
		t.Error("expected pinned bookmarks b1 and b3")
	}
}

func TestStore_GetPinnedFolders(t *testing.T) {
	store := model.Store{
		Folders: []model.Folder{
			{ID: "f1", Name: "Pinned Folder", ParentID: nil, Pinned: true},
			{ID: "f2", Name: "Not Pinned", ParentID: nil, Pinned: false},
			{ID: "f3", Name: "Another Pinned", ParentID: nil, Pinned: true},
		},
		Bookmarks: []model.Bookmark{},
	}

	pinned := store.GetPinnedFolders()
	if len(pinned) != 2 {
		t.Errorf("expected 2 pinned folders, got %d", len(pinned))
	}

	ids := make(map[string]bool)
	for _, f := range pinned {
		ids[f.ID] = true
	}
	if !ids["f1"] || !ids["f3"] {
		t.Error("expected pinned folders f1 and f3")
	}
}

func TestStore_TogglePinBookmark(t *testing.T) {
	store := model.Store{
		Folders: []model.Folder{},
		Bookmarks: []model.Bookmark{
			{ID: "b1", Title: "Test", URL: "https://test.com", FolderID: nil, Pinned: false},
		},
	}

	// Toggle on
	err := store.TogglePinBookmark("b1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !store.Bookmarks[0].Pinned {
		t.Error("expected bookmark to be pinned after toggle")
	}

	// Toggle off
	err = store.TogglePinBookmark("b1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.Bookmarks[0].Pinned {
		t.Error("expected bookmark to be unpinned after second toggle")
	}

	// Non-existent bookmark
	err = store.TogglePinBookmark("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent bookmark")
	}
}

func TestStore_TogglePinFolder(t *testing.T) {
	store := model.Store{
		Folders: []model.Folder{
			{ID: "f1", Name: "Test", ParentID: nil, Pinned: false},
		},
		Bookmarks: []model.Bookmark{},
	}

	// Toggle on
	err := store.TogglePinFolder("f1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !store.Folders[0].Pinned {
		t.Error("expected folder to be pinned after toggle")
	}

	// Toggle off
	err = store.TogglePinFolder("f1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.Folders[0].Pinned {
		t.Error("expected folder to be unpinned after second toggle")
	}

	// Non-existent folder
	err = store.TogglePinFolder("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent folder")
	}
}
