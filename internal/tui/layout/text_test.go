package layout

import "testing"

func TestStripANSI(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no ANSI", "hello", "hello"},
		{"bold", "\x1b[1mhello\x1b[0m", "hello"},
		{"color", "\x1b[31mred\x1b[0m", "red"},
		{"mixed", "normal \x1b[1;4mbold underline\x1b[0m normal", "normal bold underline normal"},
		{"empty", "", ""},
		{"only ANSI", "\x1b[1m\x1b[0m", ""},
		{"multiple codes", "\x1b[1m\x1b[31mred bold\x1b[0m", "red bold"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripANSI(tt.input)
			if got != tt.want {
				t.Errorf("StripANSI(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestVisibleLength(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"plain text", "hello", 5},
		{"with ANSI bold", "\x1b[1mhello\x1b[0m", 5},
		{"unicode", "こんにちは", 5},
		{"mixed ANSI and unicode", "\x1b[1mこんにちは\x1b[0m", 5},
		{"empty", "", 0},
		{"only ANSI", "\x1b[1m\x1b[0m", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := VisibleLength(tt.input)
			if got != tt.want {
				t.Errorf("VisibleLength(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestTruncateText(t *testing.T) {
	cfg := DefaultConfig().Text

	tests := []struct {
		name      string
		text      string
		maxWidth  int
		want      string
		truncated bool
	}{
		{"no truncation needed", "hello", 10, "hello", false},
		{"exact length", "hello", 5, "hello", false},
		{"needs truncation", "hello world", 8, "hello...", true},
		{"very short max", "hello", 3, "...", true},
		{"max is 2", "hello", 2, "..", true},
		{"max is 1", "hello", 1, ".", true},
		{"max is 0", "hello", 0, "", true},
		{"empty string", "", 10, "", false},
		{"unicode text", "こんにちは", 4, "こ...", true},
		{"unicode no truncation", "こんにちは", 10, "こんにちは", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, truncated := TruncateText(tt.text, tt.maxWidth, cfg)
			if got != tt.want || truncated != tt.truncated {
				t.Errorf("TruncateText(%q, %d) = (%q, %v), want (%q, %v)",
					tt.text, tt.maxWidth, got, truncated, tt.want, tt.truncated)
			}
		})
	}
}

func TestTruncateWithPrefixSuffix(t *testing.T) {
	cfg := DefaultConfig().Text

	tests := []struct {
		name      string
		text      string
		maxWidth  int
		prefix    string
		suffix    string
		want      string
		truncated bool
	}{
		{"no truncation", "Dev", 10, "* ", "/", "* Dev/", false},
		{"with truncation", "Development", 12, "* ", "/", "* Develo.../", true}, // 2+6+3+1=12
		{"just fits", "Dev", 6, "* ", "/", "* Dev/", false},
		{"no prefix/suffix", "Development", 8, "", "", "Devel...", true},
		{"pinned folder fits", "MyFolder", 12, "* ", "/", "* MyFolder/", false},
		{"pinned folder truncates", "VeryLongFolderName", 12, "* ", "/", "* VeryLo.../", true}, // 2+6+3+1=12
		{"empty text", "", 10, "* ", "/", "* /", false},
		{"only overhead fits", "x", 5, "* ", "/", "* x/", false},
		{"text exactly fits max", "x", 4, "* ", "/", "* x/", false},   // 2+1+1=4, fits exactly
		{"needs truncation tight", "abc", 4, "* ", "/", "*...", true}, // falls back to simple truncation
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, truncated := TruncateWithPrefixSuffix(tt.text, tt.maxWidth, tt.prefix, tt.suffix, cfg)
			if got != tt.want || truncated != tt.truncated {
				t.Errorf("TruncateWithPrefixSuffix(%q, %d, %q, %q) = (%q, %v), want (%q, %v)",
					tt.text, tt.maxWidth, tt.prefix, tt.suffix, got, truncated, tt.want, tt.truncated)
			}
		})
	}
}

func TestTruncateANSIAware(t *testing.T) {
	cfg := DefaultConfig().Text

	tests := []struct {
		name         string
		input        string
		maxWidth     int
		wantVisLen   int  // expected visible length
		wantHasReset bool // should end with reset code
	}{
		{"no truncation plain", "hello", 10, 5, false},
		{"no truncation styled", "\x1b[1mhello\x1b[0m", 10, 5, false},
		{"truncation plain", "hello world", 8, 8, true},
		{"truncation styled", "\x1b[1mhello world\x1b[0m", 8, 8, true},
		{"partial style", "he\x1b[1mllo wor\x1b[0mld", 8, 8, true},
		{"short but fits ellipsis", "hello", 5, 5, true}, // 2 chars + "..." = 5
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TruncateANSIAware(tt.input, tt.maxWidth, cfg)
			gotVisLen := VisibleLength(got)

			if gotVisLen > tt.wantVisLen {
				t.Errorf("TruncateANSIAware visible length = %d, want <= %d (got: %q)",
					gotVisLen, tt.wantVisLen, got)
			}

			// Check for reset code if truncation occurred
			needsTrunc := VisibleLength(tt.input) > tt.maxWidth
			if needsTrunc {
				hasReset := len(got) >= 4 && got[len(got)-4:] == "\x1b[0m"
				if !hasReset {
					t.Errorf("TruncateANSIAware should end with reset code when truncating, got: %q", got)
				}
			}
		})
	}
}

func TestTruncateANSIAware_EdgeCases(t *testing.T) {
	cfg := DefaultConfig().Text

	tests := []struct {
		name     string
		input    string
		maxWidth int
	}{
		{"zero width", "hello", 0},
		{"negative width", "hello", -1},
		{"empty input", "", 10},
		{"only ANSI codes", "\x1b[1m\x1b[0m", 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			got := TruncateANSIAware(tt.input, tt.maxWidth, cfg)
			_ = got // Just verify no panic
		})
	}
}
