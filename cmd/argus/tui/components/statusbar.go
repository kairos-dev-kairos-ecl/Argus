// Package components provides reusable TUI building blocks for the Argus TUI.
package components

import (
	"strings"

	"github.com/argusxdr/argus/cmd/argus/tui/theme"
	"github.com/charmbracelet/lipgloss"
)

// StatusBar renders a one-line status bar with left, center, and right sections.
// The output is always exactly `width` visible runes wide.
type StatusBar struct {
	width int
}

// NewStatusBar creates a StatusBar that renders at the given width.
func NewStatusBar(width int) StatusBar {
	return StatusBar{width: width}
}

// Render produces a string that is exactly sb.width visible characters wide.
// It places left-aligned text on the left, centered text in the middle, and
// right-aligned text on the right. All three sections are padded/truncated so
// the total visible width equals sb.width.
func (sb StatusBar) Render(left, center, right string) string {
	if sb.width <= 0 {
		return ""
	}

	// Measure each section's visual width (no ANSI codes yet).
	lw := lipgloss.Width(left)
	cw := lipgloss.Width(center)
	rw := lipgloss.Width(right)

	// Available space to distribute.
	available := sb.width

	// If the three sections together are wider than available, truncate equally.
	// Simple strategy: allocate thirds, then adjust.
	third := available / 3

	// Clamp each section to its allocation.
	left = truncateToWidth(left, third)
	right = truncateToWidth(right, third)
	center = truncateToWidth(center, third)

	// Recalculate after potential truncation.
	lw = lipgloss.Width(left)
	rw = lipgloss.Width(right)
	cw = lipgloss.Width(center)

	// Build the bar with padding.
	// Layout: [left][gap1][center][gap2][right]
	// gap1 + gap2 = available - lw - cw - rw
	totalContent := lw + cw + rw
	gaps := available - totalContent
	if gaps < 0 {
		gaps = 0
	}

	// Distribute: center should actually be centered in the total width.
	// leftPad + lw + gap1 + cw + gap2 + rw = width
	// We want center to be at position (width-cw)/2.
	centerStart := (available - cw) / 2
	gap1 := centerStart - lw
	if gap1 < 0 {
		gap1 = 0
	}
	gap2 := available - lw - gap1 - cw - rw
	if gap2 < 0 {
		gap2 = 0
	}

	// Recalculate total and add any remainder to gap2.
	total := lw + gap1 + cw + gap2 + rw
	if total < available {
		gap2 += available - total
	}

	line := left + strings.Repeat(" ", gap1) + center + strings.Repeat(" ", gap2) + right

	// Final width check — if we're still off due to rounding, pad or trim.
	lineW := lipgloss.Width(line)
	if lineW < available {
		line += strings.Repeat(" ", available-lineW)
	} else if lineW > available {
		line = truncateToWidth(line, available)
	}

	return theme.StatusBar.Width(sb.width).Render(line)
}

// truncateToWidth truncates s to at most maxWidth visible runes.
func truncateToWidth(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	w := lipgloss.Width(s)
	if w <= maxWidth {
		return s
	}
	// Truncate rune by rune.
	runes := []rune(s)
	result := []rune{}
	current := 0
	for _, r := range runes {
		rw := lipgloss.Width(string(r))
		if current+rw > maxWidth {
			break
		}
		result = append(result, r)
		current += rw
	}
	return string(result)
}
