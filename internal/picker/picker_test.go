package picker

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nikbrunner/bm/internal/model"
	"github.com/nikbrunner/bm/internal/search"
)

func TestPicker_InitialState(t *testing.T) {
	results := []search.SearchResult{
		{Bookmark: &model.Bookmark{ID: "b1", Title: "GitHub", URL: "https://github.com"}},
		{Bookmark: &model.Bookmark{ID: "b2", Title: "GitLab", URL: "https://gitlab.com"}},
	}

	p := New(results, "git")

	if p.cursor != 0 {
		t.Errorf("expected cursor at 0, got %d", p.cursor)
	}
	if len(p.results) != 2 {
		t.Errorf("expected 2 results, got %d", len(p.results))
	}
}

func TestPicker_NavigateDown(t *testing.T) {
	results := []search.SearchResult{
		{Bookmark: &model.Bookmark{ID: "b1", Title: "GitHub", URL: "https://github.com"}},
		{Bookmark: &model.Bookmark{ID: "b2", Title: "GitLab", URL: "https://gitlab.com"}},
	}

	p := New(results, "git")
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}

	newModel, _ := p.Update(msg)
	p = newModel.(Picker)

	if p.cursor != 1 {
		t.Errorf("expected cursor at 1, got %d", p.cursor)
	}
}

func TestPicker_NavigateUp(t *testing.T) {
	results := []search.SearchResult{
		{Bookmark: &model.Bookmark{ID: "b1", Title: "GitHub", URL: "https://github.com"}},
		{Bookmark: &model.Bookmark{ID: "b2", Title: "GitLab", URL: "https://gitlab.com"}},
	}

	p := New(results, "git")
	// Move down first
	p.cursor = 1

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}
	newModel, _ := p.Update(msg)
	p = newModel.(Picker)

	if p.cursor != 0 {
		t.Errorf("expected cursor at 0, got %d", p.cursor)
	}
}

func TestPicker_BoundsCheck(t *testing.T) {
	results := []search.SearchResult{
		{Bookmark: &model.Bookmark{ID: "b1", Title: "GitHub", URL: "https://github.com"}},
	}

	p := New(results, "git")

	// Try to go up from 0 (should stay at 0)
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}
	newModel, _ := p.Update(msg)
	p = newModel.(Picker)

	if p.cursor != 0 {
		t.Errorf("expected cursor at 0, got %d", p.cursor)
	}

	// Try to go down from last (should stay at last)
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	newModel, _ = p.Update(msg)
	p = newModel.(Picker)

	if p.cursor != 0 {
		t.Errorf("expected cursor at 0 (only 1 item), got %d", p.cursor)
	}
}

func TestPicker_SelectItem(t *testing.T) {
	results := []search.SearchResult{
		{Bookmark: &model.Bookmark{ID: "b1", Title: "GitHub", URL: "https://github.com", CreatedAt: time.Now()}},
		{Bookmark: &model.Bookmark{ID: "b2", Title: "GitLab", URL: "https://gitlab.com", CreatedAt: time.Now()}},
	}

	p := New(results, "git")
	p.cursor = 1 // Select GitLab

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	newModel, cmd := p.Update(msg)
	p = newModel.(Picker)

	if !p.selected {
		t.Error("expected selected to be true after Enter")
	}

	// Should return quit command
	if cmd == nil {
		t.Error("expected quit command after selection")
	}
}

func TestPicker_Cancel(t *testing.T) {
	results := []search.SearchResult{
		{Bookmark: &model.Bookmark{ID: "b1", Title: "GitHub", URL: "https://github.com"}},
	}

	p := New(results, "git")

	msg := tea.KeyMsg{Type: tea.KeyEsc}
	newModel, cmd := p.Update(msg)
	p = newModel.(Picker)

	if !p.cancelled {
		t.Error("expected cancelled to be true after Esc")
	}
	if cmd == nil {
		t.Error("expected quit command after cancel")
	}
}

func TestPicker_SelectedBookmark(t *testing.T) {
	bm := &model.Bookmark{ID: "b1", Title: "GitHub", URL: "https://github.com", CreatedAt: time.Now()}
	results := []search.SearchResult{
		{Bookmark: bm},
	}

	p := New(results, "git")
	p.selected = true

	got := p.SelectedBookmark()
	if got != bm {
		t.Errorf("expected selected bookmark to be returned")
	}
}

func TestPicker_SelectedBookmark_Cancelled(t *testing.T) {
	results := []search.SearchResult{
		{Bookmark: &model.Bookmark{ID: "b1", Title: "GitHub", URL: "https://github.com"}},
	}

	p := New(results, "git")
	p.cancelled = true

	got := p.SelectedBookmark()
	if got != nil {
		t.Error("expected nil when cancelled")
	}
}

func TestPicker_ArrowKeys(t *testing.T) {
	results := []search.SearchResult{
		{Bookmark: &model.Bookmark{ID: "b1", Title: "GitHub", URL: "https://github.com"}},
		{Bookmark: &model.Bookmark{ID: "b2", Title: "GitLab", URL: "https://gitlab.com"}},
	}

	p := New(results, "git")

	// Test down arrow
	msg := tea.KeyMsg{Type: tea.KeyDown}
	newModel, _ := p.Update(msg)
	p = newModel.(Picker)
	if p.cursor != 1 {
		t.Errorf("expected cursor at 1 after down arrow, got %d", p.cursor)
	}

	// Test up arrow
	msg = tea.KeyMsg{Type: tea.KeyUp}
	newModel, _ = p.Update(msg)
	p = newModel.(Picker)
	if p.cursor != 0 {
		t.Errorf("expected cursor at 0 after up arrow, got %d", p.cursor)
	}
}
