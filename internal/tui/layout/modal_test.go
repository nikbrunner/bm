package layout

import "testing"

func TestCalculateModalWidth(t *testing.T) {
	cfg := DefaultConfig().Modal

	tests := []struct {
		name          string
		terminalWidth int
		widthPercent  int
		want          int
	}{
		// 200*40/100=80, at MaxWidth
		{"wide terminal at max", 200, 40, 80},
		// 150*50/100=75, within bounds
		{"large terminal large percent", 150, 50, 75},
		// 120*40/100=48, but MinWidth=50
		{"medium terminal clamps to min", 120, 40, 50},
		// 100*50/100=50, at MinWidth
		{"at min width", 100, 50, 50},
		// 50*40/100=20, MinWidth=50, but terminal-4=46, so 46
		{"small terminal clamps to terminal", 50, 40, 46},
		// 20*40/100=8, MinWidth=50, but terminal-4=16, so 16
		{"very small terminal", 20, 40, 16},
		// 5*40/100=2, MinWidth=50, but terminal-4=1, so 1
		{"tiny terminal clamps to 1", 5, 40, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateModalWidth(tt.terminalWidth, tt.widthPercent, cfg)
			if got != tt.want {
				t.Errorf("CalculateModalWidth(%d, %d%%) = %d, want %d",
					tt.terminalWidth, tt.widthPercent, got, tt.want)
			}
		})
	}
}

func TestCalculateFuzzyLayout(t *testing.T) {
	cfg := DefaultConfig().Fuzzy

	tests := []struct {
		name           string
		width, height  int
		wantList       int
		wantPreview    int
		wantListHeight int
	}{
		{"standard terminal", 80, 24, 32, 44, 16},          // 80*40/100=32, 80*55/100=44, 24-8=16
		{"wide terminal", 120, 30, 48, 66, 22},             // 120*40/100=48, 120*55/100=66, 30-8=22
		{"narrow terminal", 60, 20, 24, 33, 12},            // 60*40/100=24, 60*55/100=33, 20-8=12
		{"short terminal clamps height", 80, 5, 32, 44, 1}, // height clamps to 1
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateFuzzyLayout(tt.width, tt.height, cfg)
			if got.ListWidth != tt.wantList || got.PreviewWidth != tt.wantPreview || got.ListHeight != tt.wantListHeight {
				t.Errorf("CalculateFuzzyLayout(%d, %d) = {%d, %d, %d}, want {%d, %d, %d}",
					tt.width, tt.height,
					got.ListWidth, got.PreviewWidth, got.ListHeight,
					tt.wantList, tt.wantPreview, tt.wantListHeight)
			}
		})
	}
}

func TestCalculateVisibleListItems(t *testing.T) {
	tests := []struct {
		name        string
		maxVisible  int
		selectedIdx int
		totalItems  int
		wantStart   int
		wantEnd     int
	}{
		{"at start", 5, 0, 10, 0, 5},
		{"near start", 5, 2, 10, 0, 5},
		{"in middle", 5, 7, 10, 3, 8},
		{"at end", 5, 9, 10, 5, 10},
		{"fewer than max", 5, 2, 3, 0, 3},
		{"exact max items", 5, 2, 5, 0, 5},
		{"selected beyond max", 8, 10, 15, 3, 11},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end := CalculateVisibleListItems(tt.maxVisible, tt.selectedIdx, tt.totalItems)
			if start != tt.wantStart || end != tt.wantEnd {
				t.Errorf("CalculateVisibleListItems(%d, %d, %d) = (%d, %d), want (%d, %d)",
					tt.maxVisible, tt.selectedIdx, tt.totalItems,
					start, end, tt.wantStart, tt.wantEnd)
			}
		})
	}
}
