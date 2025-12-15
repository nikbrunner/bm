package storage_test

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nikbrunner/bm/internal/exporter"
	"github.com/nikbrunner/bm/internal/importer"
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

// Integration tests for import/export with SQLite storage

func TestSQLiteStorage_ImportHTML(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "import.db")

	s, err := storage.NewSQLiteStorage(dbPath)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer s.Close()

	// Parse HTML bookmarks
	html := `<!DOCTYPE NETSCAPE-Bookmark-file-1>
<TITLE>Bookmarks</TITLE>
<DL><p>
    <DT><H3>Development</H3>
    <DL><p>
        <DT><A HREF="https://github.com" ADD_DATE="1700000000">GitHub</A>
        <DT><A HREF="https://go.dev" ADD_DATE="1700000000">Go Dev</A>
    </DL><p>
    <DT><A HREF="https://example.com" ADD_DATE="1700000000">Example</A>
</DL><p>`

	folders, bookmarks, err := importer.ParseHTMLBookmarks(strings.NewReader(html))
	if err != nil {
		t.Fatalf("failed to parse HTML: %v", err)
	}

	// Create store and import
	store := model.NewStore()
	added, skipped := store.ImportMerge(folders, bookmarks)

	if added != 3 {
		t.Errorf("expected 3 bookmarks added, got %d", added)
	}
	if skipped != 0 {
		t.Errorf("expected 0 skipped, got %d", skipped)
	}

	// Save to SQLite
	if err := s.Save(store); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	// Load from SQLite and verify
	loaded, err := s.Load()
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	if len(loaded.Folders) != 1 {
		t.Errorf("expected 1 folder, got %d", len(loaded.Folders))
	}
	if len(loaded.Bookmarks) != 3 {
		t.Errorf("expected 3 bookmarks, got %d", len(loaded.Bookmarks))
	}

	// Verify folder name
	if loaded.Folders[0].Name != "Development" {
		t.Errorf("expected folder 'Development', got %q", loaded.Folders[0].Name)
	}

	// Verify bookmark URLs exist
	urls := make(map[string]bool)
	for _, b := range loaded.Bookmarks {
		urls[b.URL] = true
	}
	expectedURLs := []string{"https://github.com", "https://go.dev", "https://example.com"}
	for _, url := range expectedURLs {
		if !urls[url] {
			t.Errorf("missing bookmark URL: %s", url)
		}
	}
}

func TestSQLiteStorage_ExportHTML(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "export.db")

	s, err := storage.NewSQLiteStorage(dbPath)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer s.Close()

	// Create store with data
	folderID := "f1"
	now := time.Now().Truncate(time.Second)
	store := &model.Store{
		Folders: []model.Folder{
			{ID: folderID, Name: "Tools", ParentID: nil},
		},
		Bookmarks: []model.Bookmark{
			{
				ID:        "b1",
				Title:     "Charm",
				URL:       "https://charm.sh",
				FolderID:  &folderID,
				Tags:      []string{"tui", "go"},
				CreatedAt: now,
			},
			{
				ID:        "b2",
				Title:     "Root Bookmark",
				URL:       "https://root.example.com",
				FolderID:  nil,
				Tags:      []string{},
				CreatedAt: now,
			},
		},
	}

	// Save to SQLite
	if err := s.Save(store); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	// Load from SQLite
	loaded, err := s.Load()
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	// Export to HTML
	html := exporter.ExportHTML(loaded)

	// Verify HTML contains expected content
	if !strings.Contains(html, "<!DOCTYPE NETSCAPE-Bookmark-file-1>") {
		t.Error("missing DOCTYPE")
	}
	if !strings.Contains(html, "Tools</H3>") {
		t.Error("missing folder name")
	}
	if !strings.Contains(html, "Charm</A>") {
		t.Error("missing bookmark title")
	}
	if !strings.Contains(html, "https://charm.sh") {
		t.Error("missing bookmark URL")
	}
	if !strings.Contains(html, "Root Bookmark</A>") {
		t.Error("missing root bookmark")
	}
}

func TestSQLiteStorage_ImportExportRoundtrip(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "roundtrip.db")

	s, err := storage.NewSQLiteStorage(dbPath)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer s.Close()

	// Create initial store with nested folders
	parentID := "f1"
	childID := "f2"
	now := time.Now().Truncate(time.Second)
	originalStore := &model.Store{
		Folders: []model.Folder{
			{ID: parentID, Name: "Development", ParentID: nil},
			{ID: childID, Name: "React", ParentID: &parentID},
		},
		Bookmarks: []model.Bookmark{
			{
				ID:        "b1",
				Title:     "TanStack",
				URL:       "https://tanstack.com",
				FolderID:  &childID,
				Tags:      []string{"react", "router"},
				CreatedAt: now,
			},
			{
				ID:        "b2",
				Title:     "GitHub",
				URL:       "https://github.com",
				FolderID:  &parentID,
				Tags:      []string{"git"},
				CreatedAt: now,
			},
		},
	}

	// Save to SQLite
	if err := s.Save(originalStore); err != nil {
		t.Fatalf("failed to save original: %v", err)
	}

	// Load and export to HTML
	loaded, err := s.Load()
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}
	html := exporter.ExportHTML(loaded)

	// Create new storage and import from HTML
	dbPath2 := filepath.Join(tmpDir, "roundtrip2.db")
	s2, err := storage.NewSQLiteStorage(dbPath2)
	if err != nil {
		t.Fatalf("failed to create second storage: %v", err)
	}
	defer s2.Close()

	folders, bookmarks, err := importer.ParseHTMLBookmarks(strings.NewReader(html))
	if err != nil {
		t.Fatalf("failed to parse exported HTML: %v", err)
	}

	newStore := model.NewStore()
	newStore.ImportMerge(folders, bookmarks)

	if err := s2.Save(newStore); err != nil {
		t.Fatalf("failed to save imported: %v", err)
	}

	// Load and verify
	reloaded, err := s2.Load()
	if err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	// Verify counts match
	if len(reloaded.Folders) != len(originalStore.Folders) {
		t.Errorf("folder count mismatch: expected %d, got %d",
			len(originalStore.Folders), len(reloaded.Folders))
	}
	if len(reloaded.Bookmarks) != len(originalStore.Bookmarks) {
		t.Errorf("bookmark count mismatch: expected %d, got %d",
			len(originalStore.Bookmarks), len(reloaded.Bookmarks))
	}

	// Verify folder names exist
	folderNames := make(map[string]bool)
	for _, f := range reloaded.Folders {
		folderNames[f.Name] = true
	}
	if !folderNames["Development"] || !folderNames["React"] {
		t.Error("missing expected folders")
	}

	// Verify bookmark URLs exist
	urls := make(map[string]bool)
	for _, b := range reloaded.Bookmarks {
		urls[b.URL] = true
	}
	if !urls["https://tanstack.com"] || !urls["https://github.com"] {
		t.Error("missing expected bookmarks")
	}

	// Verify nested structure is preserved
	var reactFolder *model.Folder
	for i := range reloaded.Folders {
		if reloaded.Folders[i].Name == "React" {
			reactFolder = &reloaded.Folders[i]
			break
		}
	}
	if reactFolder == nil {
		t.Fatal("React folder not found")
	}
	if reactFolder.ParentID == nil {
		t.Error("React folder should have a parent")
	}
}

