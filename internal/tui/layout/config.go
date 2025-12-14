package layout

// LayoutConfig holds all layout-related configuration values.
type LayoutConfig struct {
	Pane  PaneConfig
	Modal ModalConfig
	Input InputConfig
	Text  TextConfig
	Fuzzy FuzzyConfig
}

// PaneConfig holds pane dimension configuration.
type PaneConfig struct {
	// HeightReduction is subtracted from terminal height for pane content.
	// Accounts for: app padding (1) + breadcrumb (1) + pane borders (2) + help bar (3) = 7
	HeightReduction int

	// MinHeight is the minimum pane height.
	MinHeight int

	// ThreePaneWidthOffset is subtracted before dividing by 3.
	// Accounts for borders and spacing between 3 panes.
	ThreePaneWidthOffset int

	// FourPaneWidthOffset is subtracted before dividing by 4.
	// Accounts for borders and spacing between 4 panes.
	FourPaneWidthOffset int

	// MinThreePaneWidth is the minimum width for each pane in 3-pane layout.
	MinThreePaneWidth int

	// MinFourPaneWidth is the minimum width for each pane in 4-pane layout.
	MinFourPaneWidth int

	// ContentPadding is subtracted from pane width for item rendering.
	// Accounts for pane border/padding on each side.
	ContentPadding int

	// PinnedHeaderReduction accounts for header lines in pinned pane.
	PinnedHeaderReduction int
}

// ModalConfig holds modal dialog configuration.
type ModalConfig struct {
	// DefaultWidthPercent is the standard modal width as percentage of terminal width.
	DefaultWidthPercent int

	// LargeWidthPercent is used for modals needing more space (quick add confirm).
	LargeWidthPercent int

	// MinWidth is the minimum modal width in characters.
	MinWidth int

	// MaxWidth is the maximum modal width in characters.
	MaxWidth int

	// MoveMaxVisible: max items shown in move folder picker.
	MoveMaxVisible int

	// QuickAddFoldersVisible: max folders shown in quick add picker.
	QuickAddFoldersVisible int

	// HelpLeftColumnWidth: width for help overlay left column.
	HelpLeftColumnWidth int

	// HelpRightColumnWidth: width for help overlay right column.
	HelpRightColumnWidth int
}

// InputConfig holds text input configuration.
type InputConfig struct {
	// Character limits
	TitleCharLimit  int
	URLCharLimit    int
	TagsCharLimit   int
	SearchCharLimit int
	FilterCharLimit int

	// Display widths
	StandardWidth int // Used for title, URL, tags, search, move filter
	FilterWidth   int // Used for filter input (narrower)
	QuickAddWidth int // Used for quick add URL input (wider)
}

// TextConfig holds text truncation configuration.
type TextConfig struct {
	// Ellipsis is the string used to indicate truncation.
	Ellipsis string
}

// FuzzyConfig holds fuzzy finder layout configuration.
type FuzzyConfig struct {
	// ListWidthPercent: percentage of width for results list.
	ListWidthPercent int

	// PreviewWidthPercent: percentage of width for preview pane.
	PreviewWidthPercent int

	// HeaderReduction: lines for header, input, help, padding.
	HeaderReduction int
}

// DefaultConfig returns the default layout configuration.
func DefaultConfig() LayoutConfig {
	return LayoutConfig{
		Pane: PaneConfig{
			HeightReduction:       7, // app padding (1) + breadcrumb (1) + pane borders (2) + help bar (3)
			MinHeight:             5,
			ThreePaneWidthOffset:  8,
			FourPaneWidthOffset:   10,
			MinThreePaneWidth:     20,
			MinFourPaneWidth:      15,
			ContentPadding:        4,
			PinnedHeaderReduction: 4,
		},
		Modal: ModalConfig{
			DefaultWidthPercent:    40,
			LargeWidthPercent:      50,
			MinWidth:               50,
			MaxWidth:               80,
			MoveMaxVisible:         8,
			QuickAddFoldersVisible: 5,
			HelpLeftColumnWidth:    18,
			HelpRightColumnWidth:   20,
		},
		Input: InputConfig{
			TitleCharLimit:  100,
			URLCharLimit:    500,
			TagsCharLimit:   200,
			SearchCharLimit: 100,
			FilterCharLimit: 50,
			StandardWidth:   40,
			FilterWidth:     30,
			QuickAddWidth:   50,
		},
		Text: TextConfig{
			Ellipsis: "...",
		},
		Fuzzy: FuzzyConfig{
			ListWidthPercent:    40,
			PreviewWidthPercent: 55,
			HeaderReduction:     8,
		},
	}
}
