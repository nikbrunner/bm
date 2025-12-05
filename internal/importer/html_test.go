package importer_test

import (
	"strings"
	"testing"
	"time"

	"github.com/nikbrunner/bm/internal/importer"
)

func TestParseHTML_SingleBookmark(t *testing.T) {
	html := `<!DOCTYPE NETSCAPE-Bookmark-file-1>
<TITLE>Bookmarks</TITLE>
<H1>Bookmarks</H1>
<DL><p>
    <DT><A HREF="https://example.com" ADD_DATE="1234567890">Example Site</A>
</DL><p>`

	folders, bookmarks, err := importer.ParseHTMLBookmarks(strings.NewReader(html))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(folders) != 0 {
		t.Errorf("expected 0 folders, got %d", len(folders))
	}

	if len(bookmarks) != 1 {
		t.Fatalf("expected 1 bookmark, got %d", len(bookmarks))
	}

	b := bookmarks[0]
	if b.Title != "Example Site" {
		t.Errorf("expected title 'Example Site', got %q", b.Title)
	}
	if b.URL != "https://example.com" {
		t.Errorf("expected URL 'https://example.com', got %q", b.URL)
	}
	if b.FolderID != nil {
		t.Errorf("expected FolderID nil (root), got %v", b.FolderID)
	}
	if b.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestParseHTML_NestedFolders(t *testing.T) {
	html := `<!DOCTYPE NETSCAPE-Bookmark-file-1>
<DL><p>
    <DT><H3 ADD_DATE="1234567890">Development</H3>
    <DL><p>
        <DT><H3 ADD_DATE="1234567890">React</H3>
        <DL><p>
            <DT><A HREF="https://react.dev" ADD_DATE="1234567890">React Docs</A>
        </DL><p>
        <DT><A HREF="https://github.com" ADD_DATE="1234567890">GitHub</A>
    </DL><p>
    <DT><A HREF="https://google.com" ADD_DATE="1234567890">Google</A>
</DL><p>`

	folders, bookmarks, err := importer.ParseHTMLBookmarks(strings.NewReader(html))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 2 folders: Development and React
	if len(folders) != 2 {
		t.Fatalf("expected 2 folders, got %d", len(folders))
	}

	// Find Development folder (root level)
	var devFolder, reactFolder *struct {
		ID       string
		Name     string
		ParentID *string
	}
	for i := range folders {
		if folders[i].Name == "Development" {
			devFolder = &struct {
				ID       string
				Name     string
				ParentID *string
			}{folders[i].ID, folders[i].Name, folders[i].ParentID}
		}
		if folders[i].Name == "React" {
			reactFolder = &struct {
				ID       string
				Name     string
				ParentID *string
			}{folders[i].ID, folders[i].Name, folders[i].ParentID}
		}
	}

	if devFolder == nil {
		t.Fatal("Development folder not found")
	}
	if devFolder.ParentID != nil {
		t.Error("Development should be at root (ParentID nil)")
	}

	if reactFolder == nil {
		t.Fatal("React folder not found")
	}
	if reactFolder.ParentID == nil || *reactFolder.ParentID != devFolder.ID {
		t.Error("React should be child of Development")
	}

	// Should have 3 bookmarks
	if len(bookmarks) != 3 {
		t.Fatalf("expected 3 bookmarks, got %d", len(bookmarks))
	}

	// Check bookmark hierarchy
	var reactDocs, github, google *struct {
		Title    string
		FolderID *string
	}
	for i := range bookmarks {
		switch bookmarks[i].Title {
		case "React Docs":
			reactDocs = &struct {
				Title    string
				FolderID *string
			}{bookmarks[i].Title, bookmarks[i].FolderID}
		case "GitHub":
			github = &struct {
				Title    string
				FolderID *string
			}{bookmarks[i].Title, bookmarks[i].FolderID}
		case "Google":
			google = &struct {
				Title    string
				FolderID *string
			}{bookmarks[i].Title, bookmarks[i].FolderID}
		}
	}

	if reactDocs == nil || reactDocs.FolderID == nil || *reactDocs.FolderID != reactFolder.ID {
		t.Error("React Docs should be in React folder")
	}
	if github == nil || github.FolderID == nil || *github.FolderID != devFolder.ID {
		t.Error("GitHub should be in Development folder")
	}
	if google == nil || google.FolderID != nil {
		t.Error("Google should be at root level (FolderID nil)")
	}
}

func TestParseHTML_EmptyFile(t *testing.T) {
	html := `<!DOCTYPE NETSCAPE-Bookmark-file-1>
<TITLE>Bookmarks</TITLE>
<H1>Bookmarks</H1>
<DL><p>
</DL><p>`

	folders, bookmarks, err := importer.ParseHTMLBookmarks(strings.NewReader(html))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(folders) != 0 {
		t.Errorf("expected 0 folders, got %d", len(folders))
	}
	if len(bookmarks) != 0 {
		t.Errorf("expected 0 bookmarks, got %d", len(bookmarks))
	}
}

func TestParseHTML_Timestamps(t *testing.T) {
	// 1234567890 = Fri Feb 13 2009 23:31:30 UTC
	html := `<!DOCTYPE NETSCAPE-Bookmark-file-1>
<DL><p>
    <DT><A HREF="https://example.com" ADD_DATE="1234567890">Test</A>
</DL><p>`

	_, bookmarks, err := importer.ParseHTMLBookmarks(strings.NewReader(html))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(bookmarks) != 1 {
		t.Fatalf("expected 1 bookmark, got %d", len(bookmarks))
	}

	expected := time.Unix(1234567890, 0)
	if !bookmarks[0].CreatedAt.Equal(expected) {
		t.Errorf("expected CreatedAt %v, got %v", expected, bookmarks[0].CreatedAt)
	}
}

func TestParseHTML_MissingHref(t *testing.T) {
	html := `<!DOCTYPE NETSCAPE-Bookmark-file-1>
<DL><p>
    <DT><A ADD_DATE="1234567890">No URL</A>
    <DT><A HREF="https://valid.com" ADD_DATE="1234567890">Valid</A>
</DL><p>`

	_, bookmarks, err := importer.ParseHTMLBookmarks(strings.NewReader(html))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should skip bookmark without HREF, keep valid one
	if len(bookmarks) != 1 {
		t.Fatalf("expected 1 bookmark (skip missing href), got %d", len(bookmarks))
	}

	if bookmarks[0].Title != "Valid" {
		t.Errorf("expected 'Valid' bookmark, got %q", bookmarks[0].Title)
	}
}
