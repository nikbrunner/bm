package model

import "time"

// Bookmark represents a saved URL with metadata.
type Bookmark struct {
	ID        string     `json:"id"`
	Title     string     `json:"title"`
	URL       string     `json:"url"`
	FolderID  *string    `json:"folderId"` // nil = root level
	Tags      []string   `json:"tags"`
	CreatedAt time.Time  `json:"createdAt"`
	VisitedAt *time.Time `json:"visitedAt"` // nil = never visited
}

// NewBookmarkParams holds parameters for creating a new Bookmark.
type NewBookmarkParams struct {
	Title    string
	URL      string
	FolderID *string
	Tags     []string
}

// NewBookmark creates a Bookmark with generated UUID and timestamps.
func NewBookmark(params NewBookmarkParams) Bookmark {
	tags := params.Tags
	if tags == nil {
		tags = []string{}
	}

	return Bookmark{
		ID:        generateUUID(),
		Title:     params.Title,
		URL:       params.URL,
		FolderID:  params.FolderID,
		Tags:      tags,
		CreatedAt: time.Now(),
		VisitedAt: nil,
	}
}
