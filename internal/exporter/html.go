package exporter

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nikbrunner/bm/internal/model"
)

// DefaultExportPath returns the default export file path.
// Format: ~/Downloads/bookmarks-export-YYYY-MM-DD.html
func DefaultExportPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	filename := fmt.Sprintf("bookmarks-export-%s.html", time.Now().Format("2006-01-02"))
	return filepath.Join(home, "Downloads", filename), nil
}

// ExportHTML exports the store to Netscape bookmark HTML format.
func ExportHTML(store *model.Store) string {
	var b strings.Builder

	// Header
	b.WriteString("<!DOCTYPE NETSCAPE-Bookmark-file-1>\n")
	b.WriteString("<META HTTP-EQUIV=\"Content-Type\" CONTENT=\"text/html; charset=UTF-8\">\n")
	b.WriteString("<TITLE>Bookmarks</TITLE>\n")
	b.WriteString("<H1>Bookmarks</H1>\n")
	b.WriteString("<DL><p>\n")

	// Write root level items
	writeItems(&b, store, nil, 1)

	// Footer
	b.WriteString("</DL><p>\n")

	return b.String()
}

// writeItems recursively writes folders and bookmarks for a given parent.
func writeItems(b *strings.Builder, store *model.Store, parentID *string, indent int) {
	prefix := strings.Repeat("    ", indent)

	// Get folders at this level
	folders := store.GetFoldersInFolder(parentID)
	for _, folder := range folders {
		// Write folder header
		fmt.Fprintf(b, "%s<DT><H3>%s</H3>\n", prefix, html.EscapeString(folder.Name))
		fmt.Fprintf(b, "%s<DL><p>\n", prefix)

		// Recurse into folder
		folderID := folder.ID
		writeItems(b, store, &folderID, indent+1)

		// Close folder
		fmt.Fprintf(b, "%s</DL><p>\n", prefix)
	}

	// Get bookmarks at this level
	bookmarks := store.GetBookmarksInFolder(parentID)
	for _, bookmark := range bookmarks {
		timestamp := bookmark.CreatedAt.Unix()
		fmt.Fprintf(b,
			"%s<DT><A HREF=\"%s\" ADD_DATE=\"%d\">%s</A>\n",
			prefix,
			html.EscapeString(bookmark.URL),
			timestamp,
			html.EscapeString(bookmark.Title),
		)
	}
}
