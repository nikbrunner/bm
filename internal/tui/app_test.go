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

// === CRUD Tests: Yank/Cut/Paste ===

func TestApp_Yank_YY(t *testing.T) {
	store := &model.Store{
		Folders: []model.Folder{
			{ID: "f1", Name: "Folder 1", ParentID: nil},
			{ID: "f2", Name: "Folder 2", ParentID: nil},
		},
		Bookmarks: []model.Bookmark{},
	}

	app := tui.NewApp(tui.AppParams{Store: store})

	// Initially no yanked item
	if app.YankedItem() != nil {
		t.Error("expected no yanked item initially")
	}

	// Press y twice for yy (yank)
	updated, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	app = updated.(tui.App)
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	app = updated.(tui.App)

	// Should have yanked the first folder
	yanked := app.YankedItem()
	if yanked == nil {
		t.Fatal("expected item to be yanked")
	}
	if !yanked.IsFolder() || yanked.Folder.ID != "f1" {
		t.Error("expected folder f1 to be yanked")
	}

	// Original item should still exist (yank doesn't delete)
	if len(app.Items()) != 2 {
		t.Errorf("yank should not delete, expected 2 items, got %d", len(app.Items()))
	}
}

func TestApp_Yank_SingleY_Cancels(t *testing.T) {
	store := &model.Store{
		Folders: []model.Folder{
			{ID: "f1", Name: "Folder 1", ParentID: nil},
		},
		Bookmarks: []model.Bookmark{},
	}

	app := tui.NewApp(tui.AppParams{Store: store})

	// Press y once, then j (should cancel yy sequence)
	updated, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	app = updated.(tui.App)
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	app = updated.(tui.App)

	// Should not have yanked
	if app.YankedItem() != nil {
		t.Error("single y followed by other key should not yank")
	}
}

func TestApp_Cut_DD(t *testing.T) {
	store := &model.Store{
		Folders: []model.Folder{
			{ID: "f1", Name: "Folder 1", ParentID: nil},
			{ID: "f2", Name: "Folder 2", ParentID: nil},
		},
		Bookmarks: []model.Bookmark{},
	}

	app := tui.NewApp(tui.AppParams{Store: store})

	// Press d twice for dd (cut)
	updated, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	app = updated.(tui.App)
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	app = updated.(tui.App)

	// Should have yanked the first folder
	yanked := app.YankedItem()
	if yanked == nil {
		t.Fatal("expected item to be yanked after cut")
	}
	if !yanked.IsFolder() || yanked.Folder.ID != "f1" {
		t.Error("expected folder f1 to be in yank buffer")
	}

	// Item should be deleted
	if len(app.Items()) != 1 {
		t.Errorf("cut should delete item, expected 1 item, got %d", len(app.Items()))
	}

	// Remaining item should be f2
	if app.Items()[0].Folder.ID != "f2" {
		t.Error("expected f2 to remain after cutting f1")
	}
}

func TestApp_Paste_P_After(t *testing.T) {
	store := &model.Store{
		Folders: []model.Folder{
			{ID: "f1", Name: "Folder 1", ParentID: nil},
			{ID: "f2", Name: "Folder 2", ParentID: nil},
		},
		Bookmarks: []model.Bookmark{},
	}

	app := tui.NewApp(tui.AppParams{Store: store})

	// Yank first item (f1)
	updated, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	app = updated.(tui.App)
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	app = updated.(tui.App)

	// Move to second item
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	app = updated.(tui.App)

	// Paste after (p) - should insert copy of f1 after f2
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	app = updated.(tui.App)

	// Should now have 3 items
	if len(app.Items()) != 3 {
		t.Errorf("expected 3 items after paste, got %d", len(app.Items()))
	}

	// Order should be: f1, f2, (copy of f1)
	items := app.Items()
	if items[0].Folder.ID != "f1" {
		t.Error("first item should still be f1")
	}
	if items[1].Folder.ID != "f2" {
		t.Error("second item should still be f2")
	}
	// Third item should be a new folder with same name but different ID
	if items[2].Folder.Name != "Folder 1" {
		t.Error("pasted item should have same name as yanked")
	}
	if items[2].Folder.ID == "f1" {
		t.Error("pasted item should have new ID")
	}
}

func TestApp_Paste_ShiftP_Before(t *testing.T) {
	store := &model.Store{
		Folders: []model.Folder{
			{ID: "f1", Name: "Folder 1", ParentID: nil},
			{ID: "f2", Name: "Folder 2", ParentID: nil},
		},
		Bookmarks: []model.Bookmark{},
	}

	app := tui.NewApp(tui.AppParams{Store: store})

	// Move to second item and yank it
	updated, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	app = updated.(tui.App)
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	app = updated.(tui.App)
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	app = updated.(tui.App)

	// Move back to first item
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	app = updated.(tui.App)

	// Paste before (P) - should insert copy of f2 before f1
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'P'}})
	app = updated.(tui.App)

	// Should now have 3 items
	if len(app.Items()) != 3 {
		t.Errorf("expected 3 items after paste, got %d", len(app.Items()))
	}

	// Order should be: (copy of f2), f1, f2
	items := app.Items()
	if items[0].Folder.Name != "Folder 2" {
		t.Errorf("first item should be pasted Folder 2, got %s", items[0].Folder.Name)
	}
	if items[1].Folder.ID != "f1" {
		t.Error("second item should be f1")
	}
	if items[2].Folder.ID != "f2" {
		t.Error("third item should be f2")
	}
}

func TestApp_Paste_NoYankedItem(t *testing.T) {
	store := &model.Store{
		Folders: []model.Folder{
			{ID: "f1", Name: "Folder 1", ParentID: nil},
		},
		Bookmarks: []model.Bookmark{},
	}

	app := tui.NewApp(tui.AppParams{Store: store})

	// Try to paste without yanking first
	updated, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	app = updated.(tui.App)

	// Should still have 1 item (no change)
	if len(app.Items()) != 1 {
		t.Errorf("paste without yank should do nothing, expected 1 item, got %d", len(app.Items()))
	}
}

// === CRUD Tests: Modals ===

func TestApp_AddBookmark_OpenModal(t *testing.T) {
	store := &model.Store{
		Folders:   []model.Folder{},
		Bookmarks: []model.Bookmark{},
	}

	app := tui.NewApp(tui.AppParams{Store: store})

	// Initially not in modal mode
	if app.Mode() != tui.ModeNormal {
		t.Error("expected to start in normal mode")
	}

	// Press 'a' to open add bookmark modal
	updated, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	app = updated.(tui.App)

	if app.Mode() != tui.ModeAddBookmark {
		t.Error("expected to be in add bookmark mode after pressing 'a'")
	}
}

func TestApp_AddBookmark_Cancel(t *testing.T) {
	store := &model.Store{
		Folders:   []model.Folder{},
		Bookmarks: []model.Bookmark{},
	}

	app := tui.NewApp(tui.AppParams{Store: store})

	// Open modal
	updated, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	app = updated.(tui.App)

	// Press Esc to cancel
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	app = updated.(tui.App)

	// Should be back in normal mode
	if app.Mode() != tui.ModeNormal {
		t.Error("expected to be in normal mode after Esc")
	}

	// No bookmark should be added
	if len(app.Items()) != 0 {
		t.Error("no bookmark should be added after cancel")
	}
}

func TestApp_AddFolder_OpenModal(t *testing.T) {
	store := &model.Store{
		Folders:   []model.Folder{},
		Bookmarks: []model.Bookmark{},
	}

	app := tui.NewApp(tui.AppParams{Store: store})

	// Press 'A' (shift+a) to open add folder modal
	updated, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})
	app = updated.(tui.App)

	if app.Mode() != tui.ModeAddFolder {
		t.Error("expected to be in add folder mode after pressing 'A'")
	}
}

func TestApp_AddFolder_Cancel(t *testing.T) {
	store := &model.Store{
		Folders:   []model.Folder{},
		Bookmarks: []model.Bookmark{},
	}

	app := tui.NewApp(tui.AppParams{Store: store})

	// Open modal
	updated, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})
	app = updated.(tui.App)

	// Press Esc to cancel
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	app = updated.(tui.App)

	// Should be back in normal mode
	if app.Mode() != tui.ModeNormal {
		t.Error("expected to be in normal mode after Esc")
	}

	// No folder should be added
	if len(app.Items()) != 0 {
		t.Error("no folder should be added after cancel")
	}
}

func TestApp_AddFolder_Submit(t *testing.T) {
	store := &model.Store{
		Folders:   []model.Folder{},
		Bookmarks: []model.Bookmark{},
	}

	app := tui.NewApp(tui.AppParams{Store: store})

	// Open add folder modal
	updated, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})
	app = updated.(tui.App)

	// Type folder name
	for _, r := range "My Folder" {
		updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		app = updated.(tui.App)
	}

	// Press Enter to submit
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	app = updated.(tui.App)

	// Should be back in normal mode
	if app.Mode() != tui.ModeNormal {
		t.Errorf("expected normal mode after submit, got %d", app.Mode())
	}

	// Should have 1 folder
	if len(app.Items()) != 1 {
		t.Fatalf("expected 1 item after adding folder, got %d", len(app.Items()))
	}

	// Folder should have correct name
	if !app.Items()[0].IsFolder() {
		t.Error("expected item to be a folder")
	}
	if app.Items()[0].Folder.Name != "My Folder" {
		t.Errorf("expected folder name 'My Folder', got %q", app.Items()[0].Folder.Name)
	}
}

func TestApp_AddFolder_EmptyName(t *testing.T) {
	store := &model.Store{
		Folders:   []model.Folder{},
		Bookmarks: []model.Bookmark{},
	}

	app := tui.NewApp(tui.AppParams{Store: store})

	// Open add folder modal
	updated, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})
	app = updated.(tui.App)

	// Press Enter without typing anything
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	app = updated.(tui.App)

	// Should still be in modal mode (no submission without name)
	if app.Mode() != tui.ModeAddFolder {
		t.Error("expected to stay in modal mode when submitting empty name")
	}

	// No folder should be added
	if len(app.Items()) != 0 {
		t.Error("no folder should be added with empty name")
	}
}
