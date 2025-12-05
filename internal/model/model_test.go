package model_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/nikbrunner/bm/internal/model"
)

// Helper functions for pointers
func stringPtr(s string) *string    { return &s }
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
