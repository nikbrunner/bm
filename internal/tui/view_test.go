package tui_test

import (
	"testing"

	"github.com/nikbrunner/bm/internal/model"
	"github.com/nikbrunner/bm/internal/tui"
	"github.com/nikbrunner/bm/internal/tui/layout"
	"gotest.tools/v3/golden"
)

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
