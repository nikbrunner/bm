package ai

import (
	"fmt"
	"strings"

	"github.com/nikbrunner/bm/internal/model"
)

const maxSampleTitles = 3

// BuildContext generates a compressed representation of the bookmark store
// suitable for AI context. Includes folder paths with sample bookmark titles
// and a list of existing tags.
func BuildContext(store *model.Store) string {
	var sb strings.Builder

	sb.WriteString("Available folders (with sample bookmarks):\n")

	// Build folder tree starting from root
	buildFolderTree(&sb, store, nil, "")

	// Add existing tags
	tags := GetAllUniqueTags(store)
	if len(tags) > 0 {
		sb.WriteString("\nExisting tags: ")
		sb.WriteString(strings.Join(tags, ", "))
	}

	return sb.String()
}

// buildFolderTree recursively builds the folder tree representation.
func buildFolderTree(sb *strings.Builder, store *model.Store, parentID *string, path string) {
	folders := store.GetFoldersInFolder(parentID)

	for _, folder := range folders {
		currentPath := path + "/" + folder.Name

		sb.WriteString(currentPath)
		sb.WriteString("\n")

		// Add sample bookmark titles
		bookmarks := store.GetBookmarksInFolder(&folder.ID)
		sampleCount := min(len(bookmarks), maxSampleTitles)
		if sampleCount > 0 {
			titles := make([]string, sampleCount)
			for i := 0; i < sampleCount; i++ {
				titles[i] = fmt.Sprintf("\"%s\"", bookmarks[i].Title)
			}
			sb.WriteString("  - ")
			sb.WriteString(strings.Join(titles, ", "))
			sb.WriteString("\n")
		}

		// Recurse into subfolders
		buildFolderTree(sb, store, &folder.ID, currentPath)
	}
}

// GetAllUniqueTags returns all unique tags from the bookmark store.
func GetAllUniqueTags(store *model.Store) []string {
	tagSet := make(map[string]bool)
	for _, b := range store.Bookmarks {
		for _, tag := range b.Tags {
			tagSet[tag] = true
		}
	}

	tags := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		tags = append(tags, tag)
	}

	return tags
}
