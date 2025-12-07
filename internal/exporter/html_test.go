package exporter

import (
	"strings"
	"testing"
	"time"

	"github.com/nikbrunner/bm/internal/model"
)

func TestExportHTML_EmptyStore(t *testing.T) {
	store := model.NewStore()

	html := ExportHTML(store)

	// Should have basic structure even when empty
	if !strings.Contains(html, "<!DOCTYPE NETSCAPE-Bookmark-file-1>") {
		t.Error("expected DOCTYPE declaration")
	}
	if !strings.Contains(html, "<TITLE>Bookmarks</TITLE>") {
		t.Error("expected TITLE element")
	}
	if !strings.Contains(html, "<H1>Bookmarks</H1>") {
		t.Error("expected H1 element")
	}
}

func TestExportHTML_SingleBookmark(t *testing.T) {
	store := model.NewStore()
	store.AddBookmark(model.Bookmark{
		ID:        "b1",
		Title:     "GitHub",
		URL:       "https://github.com",
		FolderID:  nil,
		Tags:      []string{},
		CreatedAt: time.Unix(1700000000, 0),
	})

	html := ExportHTML(store)

	if !strings.Contains(html, `<A HREF="https://github.com"`) {
		t.Error("expected bookmark URL")
	}
	if !strings.Contains(html, "GitHub</A>") {
		t.Error("expected bookmark title")
	}
	if !strings.Contains(html, `ADD_DATE="1700000000"`) {
		t.Error("expected ADD_DATE timestamp")
	}
}

func TestExportHTML_SingleFolder(t *testing.T) {
	store := model.NewStore()
	store.AddFolder(model.Folder{
		ID:       "f1",
		Name:     "Development",
		ParentID: nil,
	})

	html := ExportHTML(store)

	if !strings.Contains(html, "<H3") {
		t.Error("expected H3 for folder")
	}
	if !strings.Contains(html, "Development</H3>") {
		t.Error("expected folder name")
	}
}

func TestExportHTML_BookmarkInFolder(t *testing.T) {
	store := model.NewStore()

	folderID := "f1"
	store.AddFolder(model.Folder{
		ID:       folderID,
		Name:     "Development",
		ParentID: nil,
	})
	store.AddBookmark(model.Bookmark{
		ID:        "b1",
		Title:     "GitHub",
		URL:       "https://github.com",
		FolderID:  &folderID,
		Tags:      []string{},
		CreatedAt: time.Unix(1700000000, 0),
	})

	html := ExportHTML(store)

	// Folder should come before its bookmark
	folderIdx := strings.Index(html, "Development</H3>")
	bookmarkIdx := strings.Index(html, "GitHub</A>")

	if folderIdx == -1 {
		t.Fatal("folder not found in output")
	}
	if bookmarkIdx == -1 {
		t.Fatal("bookmark not found in output")
	}
	if folderIdx > bookmarkIdx {
		t.Error("expected folder to come before its bookmark")
	}
}

func TestExportHTML_NestedFolders(t *testing.T) {
	store := model.NewStore()

	parentID := "f1"
	childID := "f2"
	store.AddFolder(model.Folder{
		ID:       parentID,
		Name:     "Development",
		ParentID: nil,
	})
	store.AddFolder(model.Folder{
		ID:       childID,
		Name:     "React",
		ParentID: &parentID,
	})
	store.AddBookmark(model.Bookmark{
		ID:        "b1",
		Title:     "TanStack Router",
		URL:       "https://tanstack.com/router",
		FolderID:  &childID,
		Tags:      []string{},
		CreatedAt: time.Unix(1700000000, 0),
	})

	html := ExportHTML(store)

	// Check nested structure
	devIdx := strings.Index(html, "Development</H3>")
	reactIdx := strings.Index(html, "React</H3>")
	tanstackIdx := strings.Index(html, "TanStack Router</A>")

	if devIdx == -1 || reactIdx == -1 || tanstackIdx == -1 {
		t.Fatal("missing elements in output")
	}
	if devIdx >= reactIdx || reactIdx >= tanstackIdx {
		t.Error("expected proper nesting order: Development > React > TanStack Router")
	}
}

func TestExportHTML_EscapesSpecialCharacters(t *testing.T) {
	store := model.NewStore()
	store.AddBookmark(model.Bookmark{
		ID:        "b1",
		Title:     "Test <script>alert('xss')</script>",
		URL:       "https://example.com?foo=bar&baz=qux",
		FolderID:  nil,
		Tags:      []string{},
		CreatedAt: time.Now(),
	})

	html := ExportHTML(store)

	// Title should be escaped
	if strings.Contains(html, "<script>") {
		t.Error("script tag should be escaped")
	}
	if !strings.Contains(html, "&lt;script&gt;") {
		t.Error("expected escaped script tag")
	}

	// URL should be escaped
	if strings.Contains(html, "foo=bar&baz") {
		t.Error("ampersand should be escaped in URL")
	}
	if !strings.Contains(html, "foo=bar&amp;baz") {
		t.Error("expected escaped ampersand in URL")
	}
}

func TestExportHTML_MultipleRootItems(t *testing.T) {
	store := model.NewStore()
	store.AddFolder(model.Folder{
		ID:       "f1",
		Name:     "Folder A",
		ParentID: nil,
	})
	store.AddFolder(model.Folder{
		ID:       "f2",
		Name:     "Folder B",
		ParentID: nil,
	})
	store.AddBookmark(model.Bookmark{
		ID:        "b1",
		Title:     "Root Bookmark",
		URL:       "https://example.com",
		FolderID:  nil,
		Tags:      []string{},
		CreatedAt: time.Now(),
	})

	html := ExportHTML(store)

	if !strings.Contains(html, "Folder A</H3>") {
		t.Error("expected Folder A")
	}
	if !strings.Contains(html, "Folder B</H3>") {
		t.Error("expected Folder B")
	}
	if !strings.Contains(html, "Root Bookmark</A>") {
		t.Error("expected root bookmark")
	}
}
