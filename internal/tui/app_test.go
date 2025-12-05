package tui_test

import (
	"testing"
	"time"

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

func TestApp_Cut_DD_Bookmark(t *testing.T) {
	// Test cut with bookmarks (which delete immediately without confirmation)
	store := &model.Store{
		Folders: []model.Folder{},
		Bookmarks: []model.Bookmark{
			{ID: "b1", Title: "Bookmark 1", URL: "https://1.com", FolderID: nil},
			{ID: "b2", Title: "Bookmark 2", URL: "https://2.com", FolderID: nil},
		},
	}

	app := tui.NewApp(tui.AppParams{Store: store})

	// Press d twice for dd (cut)
	updated, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	app = updated.(tui.App)
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	app = updated.(tui.App)

	// Should have yanked the first bookmark
	yanked := app.YankedItem()
	if yanked == nil {
		t.Fatal("expected item to be yanked after cut")
	}
	if yanked.IsFolder() || yanked.Bookmark.ID != "b1" {
		t.Error("expected bookmark b1 to be in yank buffer")
	}

	// Item should be deleted
	if len(app.Items()) != 1 {
		t.Errorf("cut should delete item, expected 1 item, got %d", len(app.Items()))
	}

	// Remaining item should be b2
	if app.Items()[0].Bookmark.ID != "b2" {
		t.Error("expected b2 to remain after cutting b1")
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

// === CRUD Tests: Edit ===

func TestApp_EditFolder_OpenModal(t *testing.T) {
	store := &model.Store{
		Folders: []model.Folder{
			{ID: "f1", Name: "Original Name", ParentID: nil},
		},
		Bookmarks: []model.Bookmark{},
	}

	app := tui.NewApp(tui.AppParams{Store: store})

	// Press 'e' to open edit modal
	updated, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	app = updated.(tui.App)

	// Should be in edit folder mode
	if app.Mode() != tui.ModeEditFolder {
		t.Errorf("expected ModeEditFolder, got %d", app.Mode())
	}
}

func TestApp_EditFolder_Cancel(t *testing.T) {
	store := &model.Store{
		Folders: []model.Folder{
			{ID: "f1", Name: "Original Name", ParentID: nil},
		},
		Bookmarks: []model.Bookmark{},
	}

	app := tui.NewApp(tui.AppParams{Store: store})

	// Open edit modal
	updated, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	app = updated.(tui.App)

	// Press Esc to cancel
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	app = updated.(tui.App)

	// Should be back in normal mode
	if app.Mode() != tui.ModeNormal {
		t.Error("expected normal mode after Esc")
	}

	// Folder name should be unchanged
	if app.Items()[0].Folder.Name != "Original Name" {
		t.Error("folder name should not change after cancel")
	}
}

func TestApp_EditFolder_Submit(t *testing.T) {
	store := &model.Store{
		Folders: []model.Folder{
			{ID: "f1", Name: "Original", ParentID: nil},
		},
		Bookmarks: []model.Bookmark{},
	}

	app := tui.NewApp(tui.AppParams{Store: store})

	// Open edit modal
	updated, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	app = updated.(tui.App)

	// Clear existing text and type new name
	// Select all and delete (Ctrl+U clears line in textinput)
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	app = updated.(tui.App)

	// Type new name
	for _, r := range "New Name" {
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

	// Folder should have new name
	if app.Items()[0].Folder.Name != "New Name" {
		t.Errorf("expected folder name 'New Name', got %q", app.Items()[0].Folder.Name)
	}

	// Should still be the same folder (same ID)
	if app.Items()[0].Folder.ID != "f1" {
		t.Error("folder ID should not change")
	}
}

func TestApp_EditBookmark_OpenModal(t *testing.T) {
	store := &model.Store{
		Folders: []model.Folder{},
		Bookmarks: []model.Bookmark{
			{ID: "b1", Title: "Original Title", URL: "https://original.com", FolderID: nil},
		},
	}

	app := tui.NewApp(tui.AppParams{Store: store})

	// Press 'e' to open edit modal
	updated, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	app = updated.(tui.App)

	// Should be in edit bookmark mode
	if app.Mode() != tui.ModeEditBookmark {
		t.Errorf("expected ModeEditBookmark, got %d", app.Mode())
	}
}

func TestApp_EditBookmark_Submit(t *testing.T) {
	store := &model.Store{
		Folders: []model.Folder{},
		Bookmarks: []model.Bookmark{
			{ID: "b1", Title: "Old Title", URL: "https://old.com", FolderID: nil},
		},
	}

	app := tui.NewApp(tui.AppParams{Store: store})

	// Open edit modal
	updated, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	app = updated.(tui.App)

	// Clear and type new title
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	app = updated.(tui.App)
	for _, r := range "New Title" {
		updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		app = updated.(tui.App)
	}

	// Tab to URL field
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyTab})
	app = updated.(tui.App)

	// Clear and type new URL
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	app = updated.(tui.App)
	for _, r := range "https://new.com" {
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

	// Bookmark should have new values
	if app.Items()[0].Bookmark.Title != "New Title" {
		t.Errorf("expected title 'New Title', got %q", app.Items()[0].Bookmark.Title)
	}
	if app.Items()[0].Bookmark.URL != "https://new.com" {
		t.Errorf("expected URL 'https://new.com', got %q", app.Items()[0].Bookmark.URL)
	}

	// Should still be the same bookmark (same ID)
	if app.Items()[0].Bookmark.ID != "b1" {
		t.Error("bookmark ID should not change")
	}
}

func TestApp_Edit_EmptyList(t *testing.T) {
	store := &model.Store{
		Folders:   []model.Folder{},
		Bookmarks: []model.Bookmark{},
	}

	app := tui.NewApp(tui.AppParams{Store: store})

	// Press 'e' on empty list
	updated, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	app = updated.(tui.App)

	// Should stay in normal mode (nothing to edit)
	if app.Mode() != tui.ModeNormal {
		t.Error("pressing 'e' on empty list should do nothing")
	}
}

// === CRUD Tests: Edit Tags ===

func TestApp_EditTags_OpenModal(t *testing.T) {
	store := &model.Store{
		Folders: []model.Folder{},
		Bookmarks: []model.Bookmark{
			{ID: "b1", Title: "Test", URL: "https://test.com", FolderID: nil, Tags: []string{"old", "tags"}},
		},
	}

	app := tui.NewApp(tui.AppParams{Store: store})

	// Press 't' to open edit tags modal
	updated, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	app = updated.(tui.App)

	// Should be in edit tags mode
	if app.Mode() != tui.ModeEditTags {
		t.Errorf("expected ModeEditTags, got %d", app.Mode())
	}
}

func TestApp_EditTags_OnFolder(t *testing.T) {
	store := &model.Store{
		Folders: []model.Folder{
			{ID: "f1", Name: "Folder", ParentID: nil},
		},
		Bookmarks: []model.Bookmark{},
	}

	app := tui.NewApp(tui.AppParams{Store: store})

	// Press 't' on a folder
	updated, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	app = updated.(tui.App)

	// Should stay in normal mode (folders don't have tags)
	if app.Mode() != tui.ModeNormal {
		t.Error("pressing 't' on folder should do nothing")
	}
}

func TestApp_EditTags_Cancel(t *testing.T) {
	store := &model.Store{
		Folders: []model.Folder{},
		Bookmarks: []model.Bookmark{
			{ID: "b1", Title: "Test", URL: "https://test.com", FolderID: nil, Tags: []string{"original"}},
		},
	}

	app := tui.NewApp(tui.AppParams{Store: store})

	// Open tags modal
	updated, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	app = updated.(tui.App)

	// Press Esc to cancel
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	app = updated.(tui.App)

	// Should be back in normal mode
	if app.Mode() != tui.ModeNormal {
		t.Error("expected normal mode after Esc")
	}

	// Tags should be unchanged
	if len(app.Items()[0].Bookmark.Tags) != 1 || app.Items()[0].Bookmark.Tags[0] != "original" {
		t.Error("tags should not change after cancel")
	}
}

func TestApp_EditTags_Submit(t *testing.T) {
	store := &model.Store{
		Folders: []model.Folder{},
		Bookmarks: []model.Bookmark{
			{ID: "b1", Title: "Test", URL: "https://test.com", FolderID: nil, Tags: []string{"old"}},
		},
	}

	app := tui.NewApp(tui.AppParams{Store: store})

	// Open tags modal
	updated, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	app = updated.(tui.App)

	// Clear and type new tags (comma-separated)
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	app = updated.(tui.App)
	for _, r := range "new, tags, here" {
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

	// Tags should be updated
	tags := app.Items()[0].Bookmark.Tags
	if len(tags) != 3 {
		t.Fatalf("expected 3 tags, got %d: %v", len(tags), tags)
	}
	if tags[0] != "new" || tags[1] != "tags" || tags[2] != "here" {
		t.Errorf("expected tags [new, tags, here], got %v", tags)
	}
}

func TestApp_EditTags_EmptyList(t *testing.T) {
	store := &model.Store{
		Folders:   []model.Folder{},
		Bookmarks: []model.Bookmark{},
	}

	app := tui.NewApp(tui.AppParams{Store: store})

	// Press 't' on empty list
	updated, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	app = updated.(tui.App)

	// Should stay in normal mode
	if app.Mode() != tui.ModeNormal {
		t.Error("pressing 't' on empty list should do nothing")
	}
}

// === CRUD Tests: Delete Folder Confirmation ===

func TestApp_DeleteFolder_ShowsConfirmation(t *testing.T) {
	store := &model.Store{
		Folders: []model.Folder{
			{ID: "f1", Name: "My Folder", ParentID: nil},
		},
		Bookmarks: []model.Bookmark{},
	}

	app := tui.NewApp(tui.AppParams{Store: store})

	// Press dd on a folder
	updated, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	app = updated.(tui.App)
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	app = updated.(tui.App)

	// Should be in confirm delete mode
	if app.Mode() != tui.ModeConfirmDelete {
		t.Errorf("expected ModeConfirmDelete, got %d", app.Mode())
	}

	// Folder should still exist (not deleted yet)
	if len(app.Items()) != 1 {
		t.Error("folder should not be deleted until confirmed")
	}
}

func TestApp_DeleteFolder_ConfirmWithEnter(t *testing.T) {
	store := &model.Store{
		Folders: []model.Folder{
			{ID: "f1", Name: "My Folder", ParentID: nil},
		},
		Bookmarks: []model.Bookmark{},
	}

	app := tui.NewApp(tui.AppParams{Store: store})

	// Press dd to open confirmation
	updated, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	app = updated.(tui.App)
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	app = updated.(tui.App)

	// Press Enter to confirm
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	app = updated.(tui.App)

	// Should be back in normal mode
	if app.Mode() != tui.ModeNormal {
		t.Errorf("expected normal mode after confirm, got %d", app.Mode())
	}

	// Folder should be deleted
	if len(app.Items()) != 0 {
		t.Error("folder should be deleted after confirmation")
	}

	// Should have yanked the item
	if app.YankedItem() == nil {
		t.Error("deleted item should be in yank buffer")
	}
}

func TestApp_DeleteFolder_CancelWithEsc(t *testing.T) {
	store := &model.Store{
		Folders: []model.Folder{
			{ID: "f1", Name: "My Folder", ParentID: nil},
		},
		Bookmarks: []model.Bookmark{},
	}

	app := tui.NewApp(tui.AppParams{Store: store})

	// Press dd to open confirmation
	updated, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	app = updated.(tui.App)
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	app = updated.(tui.App)

	// Press Esc to cancel
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	app = updated.(tui.App)

	// Should be back in normal mode
	if app.Mode() != tui.ModeNormal {
		t.Errorf("expected normal mode after cancel, got %d", app.Mode())
	}

	// Folder should NOT be deleted
	if len(app.Items()) != 1 {
		t.Error("folder should not be deleted after cancel")
	}
}

func TestApp_DeleteBookmark_NoConfirmation(t *testing.T) {
	store := &model.Store{
		Folders: []model.Folder{},
		Bookmarks: []model.Bookmark{
			{ID: "b1", Title: "My Bookmark", URL: "https://test.com", FolderID: nil},
		},
	}

	app := tui.NewApp(tui.AppParams{Store: store})

	// Press dd on a bookmark
	updated, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	app = updated.(tui.App)
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	app = updated.(tui.App)

	// Should delete immediately (no confirmation for bookmarks)
	if app.Mode() != tui.ModeNormal {
		t.Error("bookmarks should delete immediately without confirmation")
	}

	// Bookmark should be deleted
	if len(app.Items()) != 0 {
		t.Error("bookmark should be deleted immediately")
	}
}

// === Phase 4 Tests: Sort Mode ===

func TestApp_SortMode_DefaultIsManual(t *testing.T) {
	store := &model.Store{
		Folders:   []model.Folder{},
		Bookmarks: []model.Bookmark{},
	}

	app := tui.NewApp(tui.AppParams{Store: store})

	// Default sort mode should be manual (preserves insertion order)
	if app.SortMode() != tui.SortManual {
		t.Errorf("expected default sort mode to be SortManual, got %d", app.SortMode())
	}
}

func TestApp_SortMode_Cycle(t *testing.T) {
	store := &model.Store{
		Folders:   []model.Folder{},
		Bookmarks: []model.Bookmark{},
	}

	app := tui.NewApp(tui.AppParams{Store: store})

	// Start at manual
	if app.SortMode() != tui.SortManual {
		t.Fatal("expected to start at SortManual")
	}

	// Press 's' to cycle to alpha
	updated, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	app = updated.(tui.App)
	if app.SortMode() != tui.SortAlpha {
		t.Errorf("expected SortAlpha after first 's', got %d", app.SortMode())
	}

	// Press 's' to cycle to date created
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	app = updated.(tui.App)
	if app.SortMode() != tui.SortCreated {
		t.Errorf("expected SortCreated after second 's', got %d", app.SortMode())
	}

	// Press 's' to cycle to date visited
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	app = updated.(tui.App)
	if app.SortMode() != tui.SortVisited {
		t.Errorf("expected SortVisited after third 's', got %d", app.SortMode())
	}

	// Press 's' to cycle back to manual
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	app = updated.(tui.App)
	if app.SortMode() != tui.SortManual {
		t.Errorf("expected SortManual after fourth 's', got %d", app.SortMode())
	}
}

func TestApp_SortMode_Alpha_SortsFoldersAndBookmarks(t *testing.T) {
	store := &model.Store{
		Folders: []model.Folder{
			{ID: "f1", Name: "Zebra", ParentID: nil},
			{ID: "f2", Name: "Alpha", ParentID: nil},
			{ID: "f3", Name: "Middle", ParentID: nil},
		},
		Bookmarks: []model.Bookmark{
			{ID: "b1", Title: "Zoo", URL: "https://zoo.com", FolderID: nil},
			{ID: "b2", Title: "Ant", URL: "https://ant.com", FolderID: nil},
		},
	}

	app := tui.NewApp(tui.AppParams{Store: store})

	// Cycle to alpha sort
	updated, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	app = updated.(tui.App)

	items := app.Items()

	// Folders should be sorted alphabetically (still before bookmarks)
	if items[0].Folder.Name != "Alpha" {
		t.Errorf("expected first folder to be 'Alpha', got %q", items[0].Folder.Name)
	}
	if items[1].Folder.Name != "Middle" {
		t.Errorf("expected second folder to be 'Middle', got %q", items[1].Folder.Name)
	}
	if items[2].Folder.Name != "Zebra" {
		t.Errorf("expected third folder to be 'Zebra', got %q", items[2].Folder.Name)
	}

	// Bookmarks should be sorted alphabetically
	if items[3].Bookmark.Title != "Ant" {
		t.Errorf("expected first bookmark to be 'Ant', got %q", items[3].Bookmark.Title)
	}
	if items[4].Bookmark.Title != "Zoo" {
		t.Errorf("expected second bookmark to be 'Zoo', got %q", items[4].Bookmark.Title)
	}
}

func TestApp_SortMode_DateCreated(t *testing.T) {
	now := time.Now()
	store := &model.Store{
		Folders: []model.Folder{
			{ID: "f1", Name: "Folder 1", ParentID: nil},
		},
		Bookmarks: []model.Bookmark{
			{ID: "b1", Title: "Old", URL: "https://old.com", FolderID: nil, CreatedAt: now.Add(-48 * time.Hour)},
			{ID: "b2", Title: "New", URL: "https://new.com", FolderID: nil, CreatedAt: now},
			{ID: "b3", Title: "Middle", URL: "https://mid.com", FolderID: nil, CreatedAt: now.Add(-24 * time.Hour)},
		},
	}

	app := tui.NewApp(tui.AppParams{Store: store})

	// Cycle to date created sort (manual -> alpha -> created)
	updated, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	app = updated.(tui.App)
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	app = updated.(tui.App)

	items := app.Items()

	// First item is folder (folders first)
	if !items[0].IsFolder() {
		t.Error("expected folder first")
	}

	// Bookmarks should be sorted by created date (newest first)
	if items[1].Bookmark.Title != "New" {
		t.Errorf("expected newest bookmark first, got %q", items[1].Bookmark.Title)
	}
	if items[2].Bookmark.Title != "Middle" {
		t.Errorf("expected middle bookmark second, got %q", items[2].Bookmark.Title)
	}
	if items[3].Bookmark.Title != "Old" {
		t.Errorf("expected oldest bookmark last, got %q", items[3].Bookmark.Title)
	}
}

func TestApp_SortMode_DateVisited(t *testing.T) {
	now := time.Now()
	oldTime := now.Add(-48 * time.Hour)
	newTime := now

	store := &model.Store{
		Folders: []model.Folder{},
		Bookmarks: []model.Bookmark{
			{ID: "b1", Title: "Never Visited", URL: "https://never.com", FolderID: nil, VisitedAt: nil},
			{ID: "b2", Title: "Old Visit", URL: "https://old.com", FolderID: nil, VisitedAt: &oldTime},
			{ID: "b3", Title: "Recent Visit", URL: "https://new.com", FolderID: nil, VisitedAt: &newTime},
		},
	}

	app := tui.NewApp(tui.AppParams{Store: store})

	// Cycle to date visited sort (manual -> alpha -> created -> visited)
	for i := 0; i < 3; i++ {
		updated, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
		app = updated.(tui.App)
	}

	items := app.Items()

	// Bookmarks with visits should be sorted by visit date (most recent first)
	// Bookmarks without visits should be at the end
	if items[0].Bookmark.Title != "Recent Visit" {
		t.Errorf("expected most recently visited first, got %q", items[0].Bookmark.Title)
	}
	if items[1].Bookmark.Title != "Old Visit" {
		t.Errorf("expected old visit second, got %q", items[1].Bookmark.Title)
	}
	if items[2].Bookmark.Title != "Never Visited" {
		t.Errorf("expected never visited last, got %q", items[2].Bookmark.Title)
	}
}

// === Phase 4 Tests: Fuzzy Finder ===

func TestApp_FuzzyFinder_Open(t *testing.T) {
	store := &model.Store{
		Folders: []model.Folder{
			{ID: "f1", Name: "Dev", ParentID: nil},
			{ID: "f2", Name: "Design", ParentID: nil},
		},
		Bookmarks: []model.Bookmark{},
	}

	app := tui.NewApp(tui.AppParams{Store: store})

	// Press '/' to open fuzzy finder
	updated, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	app = updated.(tui.App)

	// Should be in search mode
	if app.Mode() != tui.ModeSearch {
		t.Errorf("expected ModeSearch after '/', got %d", app.Mode())
	}

	// Should have all items in fuzzy matches (no query = show all)
	if len(app.FuzzyMatches()) != 2 {
		t.Errorf("expected 2 fuzzy matches, got %d", len(app.FuzzyMatches()))
	}

	// Cursor should start at 0
	if app.FuzzyCursor() != 0 {
		t.Errorf("expected fuzzyCursor 0, got %d", app.FuzzyCursor())
	}
}

func TestApp_FuzzyFinder_Cancel(t *testing.T) {
	store := &model.Store{
		Folders: []model.Folder{
			{ID: "f1", Name: "Dev", ParentID: nil},
			{ID: "f2", Name: "Design", ParentID: nil},
		},
		Bookmarks: []model.Bookmark{},
	}

	app := tui.NewApp(tui.AppParams{Store: store})
	originalCursor := app.Cursor()

	// Open fuzzy finder
	updated, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	app = updated.(tui.App)

	// Press Esc to cancel
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	app = updated.(tui.App)

	// Should be back in normal mode
	if app.Mode() != tui.ModeNormal {
		t.Error("expected normal mode after Esc")
	}

	// Cursor should be unchanged
	if app.Cursor() != originalCursor {
		t.Errorf("expected cursor %d, got %d", originalCursor, app.Cursor())
	}

	// Fuzzy matches should be cleared
	if len(app.FuzzyMatches()) != 0 {
		t.Errorf("expected 0 fuzzy matches after cancel, got %d", len(app.FuzzyMatches()))
	}
}

func TestApp_FuzzyFinder_Filters(t *testing.T) {
	store := &model.Store{
		Folders: []model.Folder{
			{ID: "f1", Name: "Development", ParentID: nil},
			{ID: "f2", Name: "Design", ParentID: nil},
			{ID: "f3", Name: "Reading", ParentID: nil},
		},
		Bookmarks: []model.Bookmark{
			{ID: "b1", Title: "DevOps Guide", URL: "https://devops.com", FolderID: nil},
			{ID: "b2", Title: "Reading List", URL: "https://reading.com", FolderID: nil},
		},
	}

	app := tui.NewApp(tui.AppParams{Store: store})

	// Open fuzzy finder and type "Dev"
	updated, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	app = updated.(tui.App)

	for _, r := range "Dev" {
		updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		app = updated.(tui.App)
	}

	// Should filter fuzzy matches to items containing "Dev"
	matches := app.FuzzyMatches()
	if len(matches) != 2 {
		t.Fatalf("expected 2 fuzzy matches for 'Dev', got %d", len(matches))
	}

	// Check that we got the right items
	found := map[string]bool{"Development": false, "DevOps Guide": false}
	for _, match := range matches {
		title := match.Item.Title()
		if _, ok := found[title]; ok {
			found[title] = true
		}
	}
	for name, wasFound := range found {
		if !wasFound {
			t.Errorf("expected to find %q in fuzzy matches", name)
		}
	}
}

func TestApp_FuzzyFinder_Navigate(t *testing.T) {
	store := &model.Store{
		Folders: []model.Folder{
			{ID: "f1", Name: "Alpha", ParentID: nil},
			{ID: "f2", Name: "Beta", ParentID: nil},
			{ID: "f3", Name: "Gamma", ParentID: nil},
		},
		Bookmarks: []model.Bookmark{},
	}

	app := tui.NewApp(tui.AppParams{Store: store})

	// Open fuzzy finder
	updated, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	app = updated.(tui.App)

	// Should start at 0
	if app.FuzzyCursor() != 0 {
		t.Errorf("expected fuzzyCursor 0, got %d", app.FuzzyCursor())
	}

	// Press down arrow to move down
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyDown})
	app = updated.(tui.App)
	if app.FuzzyCursor() != 1 {
		t.Errorf("expected fuzzyCursor 1 after down, got %d", app.FuzzyCursor())
	}

	// Press up arrow to move up
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyUp})
	app = updated.(tui.App)
	if app.FuzzyCursor() != 0 {
		t.Errorf("expected fuzzyCursor 0 after up, got %d", app.FuzzyCursor())
	}
}

func TestApp_FuzzyFinder_SelectFolder(t *testing.T) {
	store := &model.Store{
		Folders: []model.Folder{
			{ID: "f1", Name: "Alpha", ParentID: nil},
			{ID: "f2", Name: "Beta", ParentID: nil},
			{ID: "f3", Name: "Gamma", ParentID: nil},
		},
		Bookmarks: []model.Bookmark{},
	}

	app := tui.NewApp(tui.AppParams{Store: store})

	// Open fuzzy finder
	updated, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	app = updated.(tui.App)

	// Navigate to second item (Beta)
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyDown})
	app = updated.(tui.App)

	// Press Enter to select - should navigate INTO Beta folder
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	app = updated.(tui.App)

	// Should be back in normal mode
	if app.Mode() != tui.ModeNormal {
		t.Error("expected normal mode after Enter")
	}

	// Should now be inside Beta folder
	if app.CurrentFolderID() == nil || *app.CurrentFolderID() != "f2" {
		t.Error("expected to be inside Beta folder (f2)")
	}

	// Fuzzy matches should be cleared
	if len(app.FuzzyMatches()) != 0 {
		t.Errorf("expected 0 fuzzy matches after select, got %d", len(app.FuzzyMatches()))
	}
}

func TestApp_FuzzyFinder_SelectBookmark(t *testing.T) {
	f1 := "f1"
	store := &model.Store{
		Folders: []model.Folder{
			{ID: "f1", Name: "Development", ParentID: nil},
		},
		Bookmarks: []model.Bookmark{
			{ID: "b1", Title: "TanStack Router", URL: "https://tanstack.com", FolderID: &f1},
			{ID: "b2", Title: "React Docs", URL: "https://react.dev", FolderID: &f1},
		},
	}

	app := tui.NewApp(tui.AppParams{Store: store})

	// Start at root (folder f1 is visible)
	if app.CurrentFolderID() != nil {
		t.Fatal("should start at root")
	}

	// Open fuzzy finder and search for "TanStack"
	updated, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	app = updated.(tui.App)

	for _, r := range "TanStack" {
		updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		app = updated.(tui.App)
	}

	// Should have 1 match
	if len(app.FuzzyMatches()) != 1 {
		t.Fatalf("expected 1 fuzzy match, got %d", len(app.FuzzyMatches()))
	}

	// Press Enter to select
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	app = updated.(tui.App)

	// Should navigate to folder f1 where the bookmark lives
	if app.CurrentFolderID() == nil || *app.CurrentFolderID() != "f1" {
		t.Error("expected to be inside Development folder (f1)")
	}

	// Cursor should be on TanStack Router bookmark
	if app.Cursor() != 0 {
		t.Errorf("expected cursor 0 (TanStack Router), got %d", app.Cursor())
	}

	// Verify it's the right item
	items := app.Items()
	if len(items) < 1 || items[app.Cursor()].Bookmark.Title != "TanStack Router" {
		t.Error("expected cursor to be on TanStack Router bookmark")
	}
}
