package importer

import (
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/nikbrunner/bm/internal/model"
	"golang.org/x/net/html"
)

// ParseHTMLBookmarks parses Netscape bookmark HTML and returns folders + bookmarks.
func ParseHTMLBookmarks(r io.Reader) ([]model.Folder, []model.Bookmark, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, nil, err
	}

	var folders []model.Folder
	var bookmarks []model.Bookmark

	// Track current folder stack for hierarchy
	var folderStack []*string // stack of folder IDs, nil = root
	var pendingFolder *model.Folder // folder waiting to be pushed on next DL

	var parse func(*html.Node)
	parse = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch strings.ToLower(n.Data) {
			case "h3":
				// Folder definition - get name from text content
				name := getTextContent(n)
				if name != "" {
					// Get parent folder ID (current top of stack)
					var parentID *string
					if len(folderStack) > 0 {
						parentID = folderStack[len(folderStack)-1]
					}

					folder := model.NewFolder(model.NewFolderParams{
						Name:     name,
						ParentID: parentID,
					})
					folders = append(folders, folder)

					// Mark this folder as pending - will be pushed when we see the next DL
					pendingFolder = &folders[len(folders)-1]
				}
				return // Don't recurse into H3

			case "a":
				// Bookmark definition
				href := getAttr(n, "href")
				if href == "" {
					// Skip bookmarks without URL
					return
				}

				title := getTextContent(n)
				if title == "" {
					title = href // fallback to URL as title
				}

				// Get parent folder ID (current top of stack)
				var folderID *string
				if len(folderStack) > 0 {
					folderID = folderStack[len(folderStack)-1]
				}

				// Parse ADD_DATE timestamp
				createdAt := time.Now()
				if addDate := getAttr(n, "add_date"); addDate != "" {
					if ts, err := strconv.ParseInt(addDate, 10, 64); err == nil {
						createdAt = time.Unix(ts, 0)
					}
				}

				bookmark := model.Bookmark{
					ID:        model.GenerateUUID(),
					Title:     title,
					URL:       href,
					FolderID:  folderID,
					Tags:      []string{},
					CreatedAt: createdAt,
					VisitedAt: nil,
				}
				bookmarks = append(bookmarks, bookmark)
				return // Don't recurse into A

			case "dl":
				// Definition list - marks folder contents
				// If we have a pending folder, push it now
				pushedFolder := false
				if pendingFolder != nil {
					id := pendingFolder.ID
					folderStack = append(folderStack, &id)
					pendingFolder = nil
					pushedFolder = true
				}

				// Process children
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					parse(c)
				}

				// Pop if we pushed
				if pushedFolder && len(folderStack) > 0 {
					folderStack = folderStack[:len(folderStack)-1]
				}
				return // Don't recurse further, we handled children
			}
		}

		// Recurse into children
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			parse(c)
		}
	}

	parse(doc)
	return folders, bookmarks, nil
}

// getTextContent returns the text content of a node.
func getTextContent(n *html.Node) string {
	var text strings.Builder
	var extract func(*html.Node)
	extract = func(n *html.Node) {
		if n.Type == html.TextNode {
			text.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			extract(c)
		}
	}
	extract(n)
	return strings.TrimSpace(text.String())
}

// getAttr returns the value of an attribute, case-insensitive.
func getAttr(n *html.Node, key string) string {
	key = strings.ToLower(key)
	for _, attr := range n.Attr {
		if strings.ToLower(attr.Key) == key {
			return attr.Val
		}
	}
	return ""
}
