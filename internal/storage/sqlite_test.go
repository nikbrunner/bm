package storage_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/nikbrunner/bm/internal/model"
	"github.com/nikbrunner/bm/internal/storage"
)

func TestSQLiteStorage_SaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "bookmarks.db")

	s, err := storage.NewSQLiteStorage(dbPath)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer s.Close()

	folderID := "f1"
	now := time.Now().Truncate(time.Second) // SQLite RFC3339 loses sub-second precision

	store := &model.Store{
		Folders: []model.Folder{
			{ID: folderID, Name: "Development", ParentID: nil, Pinned: true},
		},
		Bookmarks: []model.Bookmark{
			{
				ID:        "b1",
				Title:     "Test",
				URL:       "https://example.com",
				FolderID:  &folderID,
				Tags:      []string{"test", "example"},
				CreatedAt: now,
				VisitedAt: &now,
				Pinned:    false,
			},
		},
	}

	if err := s.Save(store); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

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
	if !loaded.Folders[0].Pinned {
		t.Error("expected folder to be pinned")
	}
	if len(loaded.Bookmarks[0].Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(loaded.Bookmarks[0].Tags))
	}
	if loaded.Bookmarks[0].FolderID == nil || *loaded.Bookmarks[0].FolderID != folderID {
		t.Error("expected bookmark folder_id to be preserved")
	}
}

func TestSQLiteStorage_EmptyDatabase(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "empty.db")

	s, err := storage.NewSQLiteStorage(dbPath)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer s.Close()

	store, err := s.Load()
	if err != nil {
		t.Fatalf("failed to load empty db: %v", err)
	}

	if len(store.Folders) != 0 || len(store.Bookmarks) != 0 {
		t.Error("expected empty store")
	}
}

func TestSQLiteStorage_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "nested", "dir", "bookmarks.db")

	s, err := storage.NewSQLiteStorage(dbPath)
	if err != nil {
		t.Fatalf("failed to create storage with nested dir: %v", err)
	}
	defer s.Close()

	store := &model.Store{
		Folders:   []model.Folder{},
		Bookmarks: []model.Bookmark{},
	}

	if err := s.Save(store); err != nil {
		t.Fatalf("failed to save: %v", err)
	}
}

func TestSQLiteStorage_NullableFields(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "nullable.db")

	s, err := storage.NewSQLiteStorage(dbPath)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer s.Close()

	// Test with nil parent_id, folder_id, visited_at
	store := &model.Store{
		Folders: []model.Folder{
			{ID: "f1", Name: "Root", ParentID: nil},
		},
		Bookmarks: []model.Bookmark{
			{
				ID:        "b1",
				Title:     "Orphan",
				URL:       "https://orphan.com",
				FolderID:  nil,
				Tags:      nil, // nil tags
				CreatedAt: time.Now(),
				VisitedAt: nil,
			},
		},
	}

	if err := s.Save(store); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	loaded, err := s.Load()
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	if loaded.Folders[0].ParentID != nil {
		t.Error("expected nil parent_id")
	}
	if loaded.Bookmarks[0].FolderID != nil {
		t.Error("expected nil folder_id")
	}
	if loaded.Bookmarks[0].VisitedAt != nil {
		t.Error("expected nil visited_at")
	}
	if loaded.Bookmarks[0].Tags == nil {
		t.Error("expected tags to be empty slice, not nil")
	}
}

func TestSQLiteStorage_TransactionRollback(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "rollback.db")

	s, err := storage.NewSQLiteStorage(dbPath)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer s.Close()

	// Save initial data
	store := &model.Store{
		Folders: []model.Folder{
			{ID: "f1", Name: "Original"},
		},
		Bookmarks: []model.Bookmark{},
	}
	if err := s.Save(store); err != nil {
		t.Fatalf("failed to save initial: %v", err)
	}

	// Save new data
	store2 := &model.Store{
		Folders: []model.Folder{
			{ID: "f2", Name: "Updated"},
		},
		Bookmarks: []model.Bookmark{},
	}
	if err := s.Save(store2); err != nil {
		t.Fatalf("failed to save updated: %v", err)
	}

	// Verify the update worked
	loaded, _ := s.Load()
	if len(loaded.Folders) != 1 || loaded.Folders[0].Name != "Updated" {
		t.Error("update did not work correctly")
	}
}

func TestSQLiteStorage_NestedFolders(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "nested.db")

	s, err := storage.NewSQLiteStorage(dbPath)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer s.Close()

	parentID := "f1"
	store := &model.Store{
		Folders: []model.Folder{
			{ID: "f1", Name: "Parent", ParentID: nil},
			{ID: "f2", Name: "Child", ParentID: &parentID},
		},
		Bookmarks: []model.Bookmark{},
	}

	if err := s.Save(store); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	loaded, err := s.Load()
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	if len(loaded.Folders) != 2 {
		t.Fatalf("expected 2 folders, got %d", len(loaded.Folders))
	}

	// Find child folder (order may vary due to ORDER BY name)
	var childFolder *model.Folder
	for i := range loaded.Folders {
		if loaded.Folders[i].ID == "f2" {
			childFolder = &loaded.Folders[i]
			break
		}
	}

	if childFolder == nil {
		t.Fatal("child folder not found")
	}
	if childFolder.ParentID == nil || *childFolder.ParentID != "f1" {
		t.Error("child folder parent_id not preserved")
	}
}

func TestMigrationFromJSON(t *testing.T) {
	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "bookmarks.json")
	sqlitePath := filepath.Join(tmpDir, "bookmarks.db")

	// Create JSON storage with data
	jsonStore := storage.NewJSONStorage(jsonPath)
	now := time.Now().Truncate(time.Second)
	folderID := "f1"

	store := &model.Store{
		Folders: []model.Folder{
			{ID: folderID, Name: "Test", ParentID: nil, Pinned: true},
		},
		Bookmarks: []model.Bookmark{
			{
				ID:        "b1",
				Title:     "Example",
				URL:       "https://example.com",
				FolderID:  &folderID,
				Tags:      []string{"tag1"},
				CreatedAt: now,
				Pinned:    true,
			},
		},
	}
	if err := jsonStore.Save(store); err != nil {
		t.Fatalf("failed to save JSON: %v", err)
	}

	// Load from JSON
	loaded, err := jsonStore.Load()
	if err != nil {
		t.Fatalf("failed to load JSON: %v", err)
	}

	// Save to SQLite
	sqliteStore, err := storage.NewSQLiteStorage(sqlitePath)
	if err != nil {
		t.Fatalf("failed to create SQLite: %v", err)
	}
	defer sqliteStore.Close()

	if err := sqliteStore.Save(loaded); err != nil {
		t.Fatalf("failed to save SQLite: %v", err)
	}

	// Verify
	reloaded, err := sqliteStore.Load()
	if err != nil {
		t.Fatalf("failed to load SQLite: %v", err)
	}

	if len(reloaded.Folders) != 1 || len(reloaded.Bookmarks) != 1 {
		t.Error("migration data mismatch")
	}
	if !reloaded.Folders[0].Pinned {
		t.Error("folder pinned status not preserved")
	}
	if !reloaded.Bookmarks[0].Pinned {
		t.Error("bookmark pinned status not preserved")
	}
	if len(reloaded.Bookmarks[0].Tags) != 1 || reloaded.Bookmarks[0].Tags[0] != "tag1" {
		t.Error("tags not preserved")
	}
}
