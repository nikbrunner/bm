package storage_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nikbrunner/bm/internal/model"
	"github.com/nikbrunner/bm/internal/storage"
)

func TestJSONStorage_SaveAndLoad(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "bookmarks.json")

	store := &model.Store{
		Folders: []model.Folder{
			{ID: "f1", Name: "Development", ParentID: nil},
		},
		Bookmarks: []model.Bookmark{
			{ID: "b1", Title: "Test", URL: "https://example.com"},
		},
	}

	// Test Save
	s := storage.NewJSONStorage(configPath)
	if err := s.Save(store); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("config file was not created")
	}

	// Test Load
	loaded, err := s.Load()
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	if len(loaded.Folders) != 1 {
		t.Errorf("expected 1 folder, got %d", len(loaded.Folders))
	}
	if len(loaded.Bookmarks) != 1 {
		t.Errorf("expected 1 bookmark, got %d", len(loaded.Bookmarks))
	}
	if loaded.Folders[0].Name != "Development" {
		t.Errorf("expected folder name 'Development', got %q", loaded.Folders[0].Name)
	}
}

func TestJSONStorage_LoadNonexistent(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nonexistent.json")

	s := storage.NewJSONStorage(configPath)
	store, err := s.Load()

	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}

	// Should return empty store
	if len(store.Folders) != 0 || len(store.Bookmarks) != 0 {
		t.Error("expected empty store for missing file")
	}
}

func TestJSONStorage_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	// Nested directory that doesn't exist
	configPath := filepath.Join(tmpDir, "nested", "dir", "bookmarks.json")

	store := &model.Store{
		Folders:   []model.Folder{},
		Bookmarks: []model.Bookmark{},
	}
	s := storage.NewJSONStorage(configPath)

	if err := s.Save(store); err != nil {
		t.Fatalf("failed to save with nested dir: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("config file was not created in nested directory")
	}
}

func TestJSONStorage_PreservesOrder(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "bookmarks.json")

	store := &model.Store{
		Folders: []model.Folder{
			{ID: "f1", Name: "First"},
			{ID: "f2", Name: "Second"},
			{ID: "f3", Name: "Third"},
		},
		Bookmarks: []model.Bookmark{},
	}

	s := storage.NewJSONStorage(configPath)
	if err := s.Save(store); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	loaded, err := s.Load()
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	// Verify order is preserved
	expectedNames := []string{"First", "Second", "Third"}
	for i, name := range expectedNames {
		if loaded.Folders[i].Name != name {
			t.Errorf("order not preserved: expected %q at position %d, got %q",
				name, i, loaded.Folders[i].Name)
		}
	}
}
