package layout

import "testing"

func TestCalculatePaneHeight(t *testing.T) {
	cfg := DefaultConfig().Pane

	tests := []struct {
		name           string
		terminalHeight int
		want           int
	}{
		{"normal terminal", 24, 17},               // 24 - 7 = 17
		{"large terminal", 50, 43},                // 50 - 7 = 43
		{"small terminal enforces min", 8, 5},     // 8 - 7 = 1, min is 5
		{"exactly at reduction", 7, 5},            // 7 - 7 = 0, min is 5
		{"terminal smaller than reduction", 4, 5}, // negative clamps to min
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculatePaneHeight(tt.terminalHeight, cfg)
			if got != tt.want {
				t.Errorf("CalculatePaneHeight(%d) = %d, want %d",
					tt.terminalHeight, got, tt.want)
			}
		})
	}
}

func TestCalculatePaneWidth_ThreePanes(t *testing.T) {
	cfg := DefaultConfig().Pane

	tests := []struct {
		name          string
		terminalWidth int
		hasPinned     bool
		atRoot        bool
		wantWidth     int
		wantCount     int
	}{
		{"normal width no pinned", 80, false, true, 24, 3},   // (80-8)/3 = 24
		{"with pinned at root", 80, true, true, 24, 3},       // still 3-pane
		{"small width enforces min", 40, false, true, 20, 3}, // (40-8)/3 = 10, min 20
		{"no pinned not at root", 80, false, false, 24, 3},   // still 3-pane
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculatePaneWidth(tt.terminalWidth, tt.hasPinned, tt.atRoot, cfg)
			if got.Width != tt.wantWidth || got.Count != tt.wantCount {
				t.Errorf("CalculatePaneWidth(%d, %v, %v) = {%d, %d}, want {%d, %d}",
					tt.terminalWidth, tt.hasPinned, tt.atRoot,
					got.Width, got.Count, tt.wantWidth, tt.wantCount)
			}
		})
	}
}

func TestCalculatePaneWidth_FourPanes(t *testing.T) {
	cfg := DefaultConfig().Pane

	tests := []struct {
		name          string
		terminalWidth int
		wantWidth     int
	}{
		{"normal width", 100, 22},      // (100-10)/4 = 22
		{"small enforces min", 50, 15}, // (50-10)/4 = 10, min 15
		{"wide terminal", 160, 37},     // (160-10)/4 = 37
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 4-pane: pinned items present, NOT at root
			got := CalculatePaneWidth(tt.terminalWidth, true, false, cfg)
			if got.Width != tt.wantWidth || got.Count != 4 {
				t.Errorf("CalculatePaneWidth(%d, true, false) = {%d, %d}, want {%d, 4}",
					tt.terminalWidth, got.Width, got.Count, tt.wantWidth)
			}
		})
	}
}

func TestCalculateItemWidth(t *testing.T) {
	cfg := DefaultConfig().Pane

	tests := []struct {
		name      string
		paneWidth int
		want      int
	}{
		{"normal pane", 24, 20}, // 24 - 4 = 20
		{"wide pane", 40, 36},   // 40 - 4 = 36
		{"narrow pane", 15, 11}, // 15 - 4 = 11
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateItemWidth(tt.paneWidth, cfg)
			if got != tt.want {
				t.Errorf("CalculateItemWidth(%d) = %d, want %d",
					tt.paneWidth, got, tt.want)
			}
		})
	}
}

func TestCalculateVisibleHeight(t *testing.T) {
	tests := []struct {
		name        string
		paneHeight  int
		headerLines int
		want        int
	}{
		{"normal with header", 18, 4, 14},
		{"no header", 18, 0, 18},
		{"header equals height", 10, 10, 1}, // clamps to 1
		{"header exceeds height", 5, 10, 1}, // clamps to 1
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateVisibleHeight(tt.paneHeight, tt.headerLines)
			if got != tt.want {
				t.Errorf("CalculateVisibleHeight(%d, %d) = %d, want %d",
					tt.paneHeight, tt.headerLines, got, tt.want)
			}
		})
	}
}

func TestCalculateViewportOffset(t *testing.T) {
	tests := []struct {
		name           string
		selected       int
		total          int
		viewportHeight int
		want           int
	}{
		{"no scroll needed", 2, 5, 10, 0},
		{"selection near start", 1, 20, 10, 0},
		{"selection in middle", 10, 20, 10, 5}, // 10 - 10/2 = 5
		{"selection near end", 18, 20, 10, 10}, // max offset = 20-10 = 10
		{"selection at end", 19, 20, 10, 10},
		{"all items visible", 5, 8, 10, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateViewportOffset(tt.selected, tt.total, tt.viewportHeight)
			if got != tt.want {
				t.Errorf("CalculateViewportOffset(%d, %d, %d) = %d, want %d",
					tt.selected, tt.total, tt.viewportHeight, got, tt.want)
			}
		})
	}
}
