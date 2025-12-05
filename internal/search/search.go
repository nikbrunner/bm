package search

import (
	"github.com/nikbrunner/bm/internal/model"
	"github.com/sahilm/fuzzy"
)

// SearchResult represents a fuzzy search match.
type SearchResult struct {
	Bookmark       *model.Bookmark
	MatchedIndexes []int
	Score          int
}

// bookmarkTitles implements fuzzy.Source for bookmark slice.
type bookmarkTitles []*model.Bookmark

func (bt bookmarkTitles) String(i int) string {
	return bt[i].Title
}

func (bt bookmarkTitles) Len() int {
	return len(bt)
}

// FuzzySearchBookmarks searches all bookmarks by title using fuzzy matching.
// Returns results sorted by match score (best first).
func FuzzySearchBookmarks(store *model.Store, query string) []SearchResult {
	if query == "" {
		return nil
	}

	// Build slice of bookmark pointers
	bookmarks := make(bookmarkTitles, len(store.Bookmarks))
	for i := range store.Bookmarks {
		bookmarks[i] = &store.Bookmarks[i]
	}

	// Run fuzzy matching
	matches := fuzzy.FindFrom(query, bookmarks)

	// Convert to SearchResult
	results := make([]SearchResult, len(matches))
	for i, m := range matches {
		results[i] = SearchResult{
			Bookmark:       bookmarks[m.Index],
			MatchedIndexes: m.MatchedIndexes,
			Score:          m.Score,
		}
	}

	return results
}
