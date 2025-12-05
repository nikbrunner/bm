package tui_test

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nikbrunner/bm/internal/model"
	"github.com/nikbrunner/bm/internal/tui"
)

func stringPtr(s string) *string { return &s }

func TestApp_Navigation_JK(t *testing.T) {
	store := &model.Store{
		Folders: []model.Folder{
			{ID: "f1", Name: "Folder 1", ParentID: nil},
			{ID: "f2", Name: "Folder 2", ParentID: nil},
			{ID: "f3", Name: "Folder 3", ParentID: nil},
		},
		Bookmarks: []model.Bookmark{},
	}

	app := tui.NewApp(tui.AppParams{Store: store})

	// Initial cursor should be 0
	if app.Cursor() != 0 {
		t.Errorf("expected initial cursor 0, got %d", app.Cursor())
	}

	// Press j to move down
	updated, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	app = updated.(tui.App)

	if app.Cursor() != 1 {
		t.Errorf("after j, expected cursor 1, got %d", app.Cursor())
	}

	// Press k to move up
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	app = updated.(tui.App)

	if app.Cursor() != 0 {
		t.Errorf("after k, expected cursor 0, got %d", app.Cursor())
	}

	// Press k at top should stay at 0 (no wrap)
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	app = updated.(tui.App)

	if app.Cursor() != 0 {
		t.Errorf("k at top should stay at 0, got %d", app.Cursor())
	}
}

func TestApp_Navigation_JK_AtBounds(t *testing.T) {
	store := &model.Store{
		Folders: []model.Folder{
			{ID: "f1", Name: "Folder 1", ParentID: nil},
			{ID: "f2", Name: "Folder 2", ParentID: nil},
		},
		Bookmarks: []model.Bookmark{},
	}

	app := tui.NewApp(tui.AppParams{Store: store})

	// Move to bottom
	updated, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	app = updated.(tui.App)

	if app.Cursor() != 1 {
		t.Errorf("expected cursor 1, got %d", app.Cursor())
	}

	// Press j at bottom should stay at bottom
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	app = updated.(tui.App)

	if app.Cursor() != 1 {
		t.Errorf("j at bottom should stay at 1, got %d", app.Cursor())
	}
}

func TestApp_Navigation_HL(t *testing.T) {
	f1ID := "f1"
	store := &model.Store{
		Folders: []model.Folder{
			{ID: "f1", Name: "Development", ParentID: nil},
			{ID: "f2", Name: "React", ParentID: &f1ID},
		},
		Bookmarks: []model.Bookmark{},
	}

	app := tui.NewApp(tui.AppParams{Store: store})

	// Should start at root
	if app.CurrentFolderID() != nil {
		t.Error("expected to start at root (nil)")
	}

	// Press l to enter the first folder
	updated, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	app = updated.(tui.App)

	if app.CurrentFolderID() == nil || *app.CurrentFolderID() != "f1" {
		t.Error("expected to enter folder f1")
	}

	// Cursor should reset to 0
	if app.Cursor() != 0 {
		t.Errorf("cursor should reset to 0 when entering folder, got %d", app.Cursor())
	}

	// Press h to go back
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	app = updated.(tui.App)

	if app.CurrentFolderID() != nil {
		t.Error("expected to be back at root")
	}
}

func TestApp_Navigation_HL_NestedFolders(t *testing.T) {
	f1ID := "f1"
	f2ID := "f2"
	store := &model.Store{
		Folders: []model.Folder{
			{ID: "f1", Name: "Development", ParentID: nil},
			{ID: "f2", Name: "React", ParentID: &f1ID},
			{ID: "f3", Name: "Hooks", ParentID: &f2ID},
		},
		Bookmarks: []model.Bookmark{},
	}

	app := tui.NewApp(tui.AppParams{Store: store})

	// Enter f1
	updated, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	app = updated.(tui.App)

	// Enter f2
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	app = updated.(tui.App)

	if app.CurrentFolderID() == nil || *app.CurrentFolderID() != "f2" {
		t.Errorf("expected to be in f2, got %v", app.CurrentFolderID())
	}

	// Go back to f1
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	app = updated.(tui.App)

	if app.CurrentFolderID() == nil || *app.CurrentFolderID() != "f1" {
		t.Errorf("expected to be in f1, got %v", app.CurrentFolderID())
	}

	// Go back to root
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	app = updated.(tui.App)

	if app.CurrentFolderID() != nil {
		t.Error("expected to be at root")
	}
}

func TestApp_Navigation_L_OnBookmark(t *testing.T) {
	store := &model.Store{
		Folders: []model.Folder{},
		Bookmarks: []model.Bookmark{
			{ID: "b1", Title: "Test", URL: "https://example.com", FolderID: nil},
		},
	}

	app := tui.NewApp(tui.AppParams{Store: store})

	// Press l on a bookmark should do nothing
	updated, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	app = updated.(tui.App)

	// Should still be at root
	if app.CurrentFolderID() != nil {
		t.Error("pressing l on bookmark should not navigate")
	}
}

func TestApp_Navigation_H_AtRoot(t *testing.T) {
	store := &model.Store{
		Folders: []model.Folder{
			{ID: "f1", Name: "Test", ParentID: nil},
		},
		Bookmarks: []model.Bookmark{},
	}

	app := tui.NewApp(tui.AppParams{Store: store})

	// Press h at root should do nothing
	updated, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	app = updated.(tui.App)

	if app.CurrentFolderID() != nil {
		t.Error("pressing h at root should stay at root")
	}
}

func TestApp_Navigation_GG_G(t *testing.T) {
	store := &model.Store{
		Folders: []model.Folder{
			{ID: "f1", Name: "A", ParentID: nil},
			{ID: "f2", Name: "B", ParentID: nil},
			{ID: "f3", Name: "C", ParentID: nil},
			{ID: "f4", Name: "D", ParentID: nil},
			{ID: "f5", Name: "E", ParentID: nil},
		},
		Bookmarks: []model.Bookmark{},
	}

	app := tui.NewApp(tui.AppParams{Store: store})

	// Press G to go to bottom
	updated, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	app = updated.(tui.App)

	if app.Cursor() != 4 {
		t.Errorf("G should go to last item (4), got %d", app.Cursor())
	}

	// Press g twice for gg
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	app = updated.(tui.App)
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	app = updated.(tui.App)

	if app.Cursor() != 0 {
		t.Errorf("gg should go to first item (0), got %d", app.Cursor())
	}
}

func TestApp_Navigation_G_SingleG(t *testing.T) {
	store := &model.Store{
		Folders: []model.Folder{
			{ID: "f1", Name: "A", ParentID: nil},
			{ID: "f2", Name: "B", ParentID: nil},
		},
		Bookmarks: []model.Bookmark{},
	}

	app := tui.NewApp(tui.AppParams{Store: store})

	// Move to position 1
	updated, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	app = updated.(tui.App)

	// Single g followed by different key should not jump to top
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	app = updated.(tui.App)
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	app = updated.(tui.App)

	// Cursor should still be at 1 (or wherever j took it)
	// The g was "cancelled" by j
	if app.Cursor() != 1 {
		t.Errorf("single g followed by j should cancel gg, cursor at %d", app.Cursor())
	}
}

func TestApp_EmptyStore(t *testing.T) {
	store := &model.Store{
		Folders:   []model.Folder{},
		Bookmarks: []model.Bookmark{},
	}

	app := tui.NewApp(tui.AppParams{Store: store})

	// Should handle empty list gracefully
	if app.Cursor() != 0 {
		t.Errorf("cursor should be 0 for empty store, got %d", app.Cursor())
	}

	// Navigation should not crash
	updated, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	app = updated.(tui.App)

	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	app = updated.(tui.App)

	// Should still be at 0
	if app.Cursor() != 0 {
		t.Errorf("cursor should stay at 0 for empty store, got %d", app.Cursor())
	}
}

func TestApp_ItemsOrder_FoldersFirst(t *testing.T) {
	store := &model.Store{
		Folders: []model.Folder{
			{ID: "f1", Name: "Zebra Folder", ParentID: nil},
		},
		Bookmarks: []model.Bookmark{
			{ID: "b1", Title: "Alpha Bookmark", URL: "https://alpha.com", FolderID: nil},
		},
	}

	app := tui.NewApp(tui.AppParams{Store: store})

	items := app.Items()

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	// First item should be the folder
	if !items[0].IsFolder() {
		t.Error("first item should be a folder")
	}

	// Second item should be the bookmark
	if items[1].IsFolder() {
		t.Error("second item should be a bookmark")
	}
}
