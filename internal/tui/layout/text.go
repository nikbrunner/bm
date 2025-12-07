package layout

import (
	"regexp"
	"unicode/utf8"
)

// ansiRegex matches ANSI escape sequences.
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// StripANSI removes ANSI escape codes from a string.
func StripANSI(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

// VisibleLength returns the visible length of a string (excluding ANSI codes).
func VisibleLength(s string) int {
	return utf8.RuneCountInString(StripANSI(s))
}

// TruncateText truncates text to maxWidth with ellipsis.
// Handles edge cases where text is shorter than maxWidth or maxWidth is very small.
// Returns the truncated text and whether truncation occurred.
func TruncateText(text string, maxWidth int, cfg TextConfig) (string, bool) {
	if maxWidth <= 0 {
		return "", true
	}

	ellipsisLen := utf8.RuneCountInString(cfg.Ellipsis)
	textLen := utf8.RuneCountInString(text)

	if textLen <= maxWidth {
		return text, false
	}

	// Need space for ellipsis
	if maxWidth <= ellipsisLen {
		// Not enough room for any text + ellipsis, just return truncated ellipsis
		runes := []rune(cfg.Ellipsis)
		return string(runes[:maxWidth]), true
	}

	runes := []rune(text)
	truncLen := maxWidth - ellipsisLen
	return string(runes[:truncLen]) + cfg.Ellipsis, true
}

// TruncateWithPrefixSuffix truncates text while preserving prefix and suffix.
// Example: TruncateWithPrefixSuffix("Development", 12, "* ", "/", cfg) -> "* Devel.../"
// Returns the truncated text and whether truncation occurred.
func TruncateWithPrefixSuffix(text string, maxWidth int, prefix, suffix string, cfg TextConfig) (string, bool) {
	if maxWidth <= 0 {
		return "", true
	}

	combined := prefix + text + suffix
	combinedLen := utf8.RuneCountInString(combined)

	if combinedLen <= maxWidth {
		return combined, false
	}

	// Calculate available space for text
	prefixLen := utf8.RuneCountInString(prefix)
	suffixLen := utf8.RuneCountInString(suffix)
	ellipsisLen := utf8.RuneCountInString(cfg.Ellipsis)
	overhead := prefixLen + suffixLen + ellipsisLen

	if overhead >= maxWidth {
		// Not enough room even for prefix + ellipsis + suffix
		// Fall back to simple truncation
		return TruncateText(combined, maxWidth, cfg)
	}

	availableForText := maxWidth - overhead
	runes := []rune(text)

	if availableForText <= 0 {
		return prefix + cfg.Ellipsis + suffix, true
	}

	return prefix + string(runes[:availableForText]) + cfg.Ellipsis + suffix, true
}

// TruncateANSIAware truncates styled text, preserving ANSI codes.
// This is critical for fuzzy finder item rendering where matches are highlighted.
// The result will have a reset code appended to prevent style bleed.
func TruncateANSIAware(styledText string, maxWidth int, cfg TextConfig) string {
	if maxWidth <= 0 {
		return ""
	}

	// If the visible content is already short enough, return as-is
	visLen := VisibleLength(styledText)
	if visLen <= maxWidth {
		return styledText
	}

	ellipsisLen := utf8.RuneCountInString(cfg.Ellipsis)
	targetVisibleLen := maxWidth - ellipsisLen
	if targetVisibleLen < 0 {
		targetVisibleLen = 0
	}

	// Walk through preserving ANSI codes
	var result []byte
	var visibleCount int
	input := []byte(styledText)
	resetCode := []byte("\x1b[0m")

	i := 0
	for i < len(input) && visibleCount < targetVisibleLen {
		// Check for ANSI escape sequence
		if input[i] == '\x1b' && i+1 < len(input) && input[i+1] == '[' {
			// Find end of ANSI sequence
			j := i + 2
			for j < len(input) && input[j] != 'm' {
				j++
			}
			if j < len(input) {
				// Include the 'm'
				result = append(result, input[i:j+1]...)
				i = j + 1
				continue
			}
		}

		// Regular character - decode UTF-8
		r, size := utf8.DecodeRune(input[i:])
		if r != utf8.RuneError {
			result = append(result, input[i:i+size]...)
			visibleCount++
		}
		i += size
	}

	// Add ellipsis and reset code to clear any active styling
	result = append(result, []byte(cfg.Ellipsis)...)
	result = append(result, resetCode...)

	return string(result)
}
