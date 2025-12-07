package tui_test

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nikbrunner/bm/internal/model"
	"github.com/nikbrunner/bm/internal/tui"
	"github.com/nikbrunner/bm/internal/tui/layout"
	"gotest.tools/v3/golden"
)

// pressKey simulates a key press and returns the updated app.
func pressKey(app tui.App, key rune) tui.App {
	updated, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{key}})
	return updated.(tui.App)
}

// testLayoutConfig returns a fixed layout config for consistent snapshot testing.
func testLayoutConfig() layout.LayoutConfig {
	cfg := layout.DefaultConfig()
	return cfg
}

// testStore creates a sample store for snapshot tests.
func testStore() *model.Store {
	f1ID := "folder-dev"
	f2ID := "folder-tools"
	return &model.Store{
		Folders: []model.Folder{
			{ID: f1ID, Name: "Development", ParentID: nil},
			{ID: f2ID, Name: "Tools", ParentID: nil},
			{ID: "folder-go", Name: "Go", ParentID: &f1ID},
		},
		Bookmarks: []model.Bookmark{
			{ID: "bm-1", Title: "GitHub", URL: "https://github.com", FolderID: nil},
			{ID: "bm-2", Title: "Go Docs", URL: "https://go.dev", FolderID: &f1ID, Tags: []string{"docs", "go"}},
		},
	}
}

// createTestApp creates a test app with fixed dimensions.
func createTestApp(width, height int) tui.App {
	store := testStore()
	cfg := testLayoutConfig()
	app := tui.NewApp(tui.AppParams{
		Store:        store,
		LayoutConfig: &cfg,
	})

	// Set fixed dimensions for consistent output
	app = app.WithDimensions(width, height)

	return app
}

func TestView_NormalMode_80x24(t *testing.T) {
	app := createTestApp(80, 24)
	output := layout.StripANSI(app.View())
	golden.Assert(t, output, "golden/normal_mode_80x24.golden")
}

func TestView_NormalMode_120x30(t *testing.T) {
	app := createTestApp(120, 30)
	output := layout.StripANSI(app.View())
	golden.Assert(t, output, "golden/normal_mode_120x30.golden")
}

func TestView_EmptyState(t *testing.T) {
	store := &model.Store{
		Folders:   []model.Folder{},
		Bookmarks: []model.Bookmark{},
	}
	cfg := testLayoutConfig()
	app := tui.NewApp(tui.AppParams{
		Store:        store,
		LayoutConfig: &cfg,
	})
	app = app.WithDimensions(80, 24)

	output := layout.StripANSI(app.View())
	golden.Assert(t, output, "golden/empty_state.golden")
}

// =============================================================================
// Interaction + Visual Tests
// =============================================================================

func TestView_Navigation_CursorDown(t *testing.T) {
	app := createTestApp(80, 24)

	// Press j to move cursor down
	app = pressKey(app, 'j')

	// Verify cursor moved
	if app.Cursor() != 1 {
		t.Errorf("expected cursor at 1, got %d", app.Cursor())
	}

	// Snapshot the result
	output := layout.StripANSI(app.View())
	golden.Assert(t, output, "golden/navigation_cursor_down.golden")
}

func TestView_Navigation_EnterFolder(t *testing.T) {
	app := createTestApp(80, 24)

	// Press l to enter first folder (Development)
	app = pressKey(app, 'l')

	// Verify we entered the folder
	if app.CurrentFolderID() == nil {
		t.Error("expected to be in a folder, but still at root")
	}

	// Snapshot the result
	output := layout.StripANSI(app.View())
	golden.Assert(t, output, "golden/navigation_enter_folder.golden")
}

func TestView_HelpOverlay(t *testing.T) {
	app := createTestApp(80, 24)

	// Press ? to open help
	app = pressKey(app, '?')

	// Verify help mode
	if app.Mode() != tui.ModeHelp {
		t.Errorf("expected ModeHelp, got %v", app.Mode())
	}

	// Snapshot the result
	output := layout.StripANSI(app.View())
	golden.Assert(t, output, "golden/help_overlay.golden")
}

func TestView_SearchMode(t *testing.T) {
	app := createTestApp(80, 24)

	// Press s to enter search mode
	app = pressKey(app, 's')

	// Verify search mode
	if app.Mode() != tui.ModeSearch {
		t.Errorf("expected ModeSearch, got %v", app.Mode())
	}

	// Snapshot the result
	output := layout.StripANSI(app.View())
	golden.Assert(t, output, "golden/search_mode.golden")
}

func TestView_FilterMode(t *testing.T) {
	app := createTestApp(80, 24)

	// Press / to enter filter mode
	app = pressKey(app, '/')

	// Verify filter mode
	if app.Mode() != tui.ModeFilter {
		t.Errorf("expected ModeFilter, got %v", app.Mode())
	}

	// Snapshot the result
	output := layout.StripANSI(app.View())
	golden.Assert(t, output, "golden/filter_mode.golden")
}

func TestView_AddBookmarkModal(t *testing.T) {
	app := createTestApp(80, 24)

	// Press a to open add bookmark modal
	app = pressKey(app, 'a')

	// Verify add bookmark mode
	if app.Mode() != tui.ModeAddBookmark {
		t.Errorf("expected ModeAddBookmark, got %v", app.Mode())
	}

	// Snapshot the result
	output := layout.StripANSI(app.View())
	golden.Assert(t, output, "golden/add_bookmark_modal.golden")
}

func TestView_AddFolderModal(t *testing.T) {
	app := createTestApp(80, 24)

	// Press A to open add folder modal
	app = pressKey(app, 'A')

	// Verify add folder mode
	if app.Mode() != tui.ModeAddFolder {
		t.Errorf("expected ModeAddFolder, got %v", app.Mode())
	}

	// Snapshot the result
	output := layout.StripANSI(app.View())
	golden.Assert(t, output, "golden/add_folder_modal.golden")
}

func TestView_ConfirmDelete(t *testing.T) {
	app := createTestApp(80, 24)

	// Press d to trigger delete (with confirmation enabled by default)
	app = pressKey(app, 'd')

	// Verify confirm delete mode
	if app.Mode() != tui.ModeConfirmDelete {
		t.Errorf("expected ModeConfirmDelete, got %v", app.Mode())
	}

	// Snapshot the result
	output := layout.StripANSI(app.View())
	golden.Assert(t, output, "golden/confirm_delete.golden")
}

func TestView_Flow_NavigateAndSearch(t *testing.T) {
	app := createTestApp(80, 24)

	// Navigate down twice
	app = pressKey(app, 'j')
	app = pressKey(app, 'j')

	// Enter search mode
	app = pressKey(app, 's')

	// Verify search mode
	if app.Mode() != tui.ModeSearch {
		t.Errorf("expected ModeSearch, got %v", app.Mode())
	}

	// Snapshot the result
	output := layout.StripANSI(app.View())
	golden.Assert(t, output, "golden/flow_navigate_and_search.golden")
}

func TestView_Flow_EnterFolderAndBack(t *testing.T) {
	app := createTestApp(80, 24)

	// Enter first folder
	app = pressKey(app, 'l')

	// Go back with h
	app = pressKey(app, 'h')

	// Verify back at root
	if app.CurrentFolderID() != nil {
		t.Error("expected to be at root after pressing h")
	}

	// Snapshot the result
	output := layout.StripANSI(app.View())
	golden.Assert(t, output, "golden/flow_enter_folder_and_back.golden")
}
