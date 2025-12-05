package search

import (
	"testing"
	"time"

	"github.com/nikbrunner/bm/internal/model"
)

func TestFuzzySearchBookmarks_EmptyQuery(t *testing.T) {
	store := model.NewStore()
	store.AddBookmark(model.Bookmark{
		ID:        "b1",
		Title:     "GitHub",
		URL:       "https://github.com",
		CreatedAt: time.Now(),
	})

	results := FuzzySearchBookmarks(store, "")

	if len(results) != 0 {
		t.Errorf("expected 0 results for empty query, got %d", len(results))
	}
}

func TestFuzzySearchBookmarks_ExactMatch(t *testing.T) {
	store := model.NewStore()
	store.AddBookmark(model.Bookmark{
		ID:        "b1",
		Title:     "GitHub",
		URL:       "https://github.com",
		CreatedAt: time.Now(),
	})
	store.AddBookmark(model.Bookmark{
		ID:        "b2",
		Title:     "GitLab",
		URL:       "https://gitlab.com",
		CreatedAt: time.Now(),
	})

	results := FuzzySearchBookmarks(store, "GitHub")

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Bookmark.Title != "GitHub" {
		t.Errorf("expected GitHub, got %s", results[0].Bookmark.Title)
	}
}

func TestFuzzySearchBookmarks_FuzzyMatch(t *testing.T) {
	store := model.NewStore()
	store.AddBookmark(model.Bookmark{
		ID:        "b1",
		Title:     "TanStack Router",
		URL:       "https://tanstack.com/router",
		CreatedAt: time.Now(),
	})
	store.AddBookmark(model.Bookmark{
		ID:        "b2",
		Title:     "React Router",
		URL:       "https://reactrouter.com",
		CreatedAt: time.Now(),
	})

	// "tanrou" should fuzzy match "TanStack Router"
	results := FuzzySearchBookmarks(store, "tanrou")

	if len(results) < 1 {
		t.Fatalf("expected at least 1 result for 'tanrou', got %d", len(results))
	}
	// TanStack Router should be first (better match)
	if results[0].Bookmark.Title != "TanStack Router" {
		t.Errorf("expected TanStack Router as first result, got %s", results[0].Bookmark.Title)
	}
}

func TestFuzzySearchBookmarks_MultipleMatches(t *testing.T) {
	store := model.NewStore()
	store.AddBookmark(model.Bookmark{
		ID:        "b1",
		Title:     "GitHub",
		URL:       "https://github.com",
		CreatedAt: time.Now(),
	})
	store.AddBookmark(model.Bookmark{
		ID:        "b2",
		Title:     "GitLab",
		URL:       "https://gitlab.com",
		CreatedAt: time.Now(),
	})
	store.AddBookmark(model.Bookmark{
		ID:        "b3",
		Title:     "Gitea",
		URL:       "https://gitea.io",
		CreatedAt: time.Now(),
	})

	results := FuzzySearchBookmarks(store, "git")

	if len(results) != 3 {
		t.Errorf("expected 3 results for 'git', got %d", len(results))
	}
}

func TestFuzzySearchBookmarks_NoMatch(t *testing.T) {
	store := model.NewStore()
	store.AddBookmark(model.Bookmark{
		ID:        "b1",
		Title:     "GitHub",
		URL:       "https://github.com",
		CreatedAt: time.Now(),
	})

	results := FuzzySearchBookmarks(store, "xyz123")

	if len(results) != 0 {
		t.Errorf("expected 0 results for 'xyz123', got %d", len(results))
	}
}

func TestFuzzySearchBookmarks_CaseInsensitive(t *testing.T) {
	store := model.NewStore()
	store.AddBookmark(model.Bookmark{
		ID:        "b1",
		Title:     "GitHub",
		URL:       "https://github.com",
		CreatedAt: time.Now(),
	})

	results := FuzzySearchBookmarks(store, "github")

	if len(results) != 1 {
		t.Fatalf("expected 1 result for case-insensitive match, got %d", len(results))
	}
}

func TestFuzzySearchBookmarks_SortedByScore(t *testing.T) {
	store := model.NewStore()
	store.AddBookmark(model.Bookmark{
		ID:        "b1",
		Title:     "React Router Documentation",
		URL:       "https://reactrouter.com",
		CreatedAt: time.Now(),
	})
	store.AddBookmark(model.Bookmark{
		ID:        "b2",
		Title:     "Router",
		URL:       "https://router.example.com",
		CreatedAt: time.Now(),
	})

	results := FuzzySearchBookmarks(store, "router")

	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(results))
	}
	// "Router" should rank higher (exact match) than "React Router Documentation"
	if results[0].Bookmark.Title != "Router" {
		t.Errorf("expected 'Router' as first result (exact match), got %s", results[0].Bookmark.Title)
	}
}
