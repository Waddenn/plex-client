package shared

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// ClampMin returns min if value is lower, otherwise value.
func ClampMin(value, min int) int {
	if value < min {
		return min
	}
	return value
}

const (
	SplitThreshold = 80
	SplitLeftRatio = 0.50
	SplitMinLeft   = 30
	SplitMinRight  = 24
)

// SplitWidths returns left/right widths for a two-panel layout.
func SplitWidths(total int, leftRatio float64, minLeft, minRight int) (int, int) {
	left := int(float64(total) * leftRatio)
	if left < minLeft {
		left = minLeft
	}

	right := total - left
	if right < minRight {
		right = minRight
		left = total - right
		if left < minLeft {
			left = minLeft
		}
	}

	return left, right
}

// SplitWithSidebar returns left/right widths for a two-panel content area when a fixed sidebar is present.
// The separator is positioned at leftRatio of the total width when possible.
func SplitWithSidebar(total, sidebarWidth int, leftRatio float64, minLeft, minRight int) (int, int) {
	contentWidth := total - sidebarWidth
	if contentWidth <= 0 {
		return 0, 0
	}

	desiredSplit := int(float64(total) * leftRatio)
	left := desiredSplit - sidebarWidth
	right := contentWidth - left

	if left < minLeft || right < minRight {
		return SplitWidths(contentWidth, leftRatio, minLeft, minRight)
	}

	return left, right
}

// RenderHeader renders a standard header using the shared header style.
func RenderHeader(content string, width int) string {
	return StyleHeader.Copy().Width(width).Render(content)
}

// RenderHeaderLegacySafe renders a header using the legacy browser style and a fixed height.
// Returns the rendered header and the fixed height used for layout.
func RenderHeaderLegacySafe(content string, width int) (string, int) {
	safeWidth := ClampMin(width, 20)
	header := StyleHeader.Copy().Width(safeWidth).Render(content)
	header = lipgloss.NewStyle().
		Width(safeWidth).
		Height(3).
		MaxHeight(3).
		Render(header)
	return header, 3
}

// RenderFooter renders a footer with optional left and right content.
// When right is provided, it is right-aligned within the available width.
func RenderFooter(left, right string, width int) string {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)

	content := left
	if right != "" {
		if left == "" {
			space := width - lipgloss.Width(right) - 2
			if space > 0 {
				content = strings.Repeat(" ", space) + right
			} else {
				content = right
			}
		} else {
			space := width - lipgloss.Width(left) - lipgloss.Width(right) - 2
			if space > 0 {
				content = left + strings.Repeat(" ", space) + right
			} else {
				content = left + "  " + right
			}
		}
	}

	return StyleFooter.Copy().Width(width).Render(content)
}

// RenderFooterLegacySafe renders a single-line footer with a safe width.
func RenderFooterLegacySafe(left, right string, width int) (string, int) {
	safeWidth := ClampMin(width, 20)
	// Ensure single-line by truncating left/right to fit.
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left != "" && right != "" {
		availableLeft := safeWidth - lipgloss.Width(right) - 2
		if availableLeft < 1 {
			// Not enough room for left; prioritize right.
			left = ""
		} else if lipgloss.Width(left) > availableLeft {
			left = Truncate(left, availableLeft)
		}
	} else if left == "" && right != "" {
		if lipgloss.Width(right) > safeWidth-2 {
			right = Truncate(right, safeWidth-2)
		}
	} else if right == "" && left != "" {
		if lipgloss.Width(left) > safeWidth-2 {
			left = Truncate(left, safeWidth-2)
		}
	}

	content := RenderFooter(left, right, safeWidth)
	footer := lipgloss.NewStyle().Width(safeWidth).Height(1).Render(content)
	return footer, 1
}

// IsBlankVisible returns true if s is empty or only whitespace after stripping ANSI codes.
func IsBlankVisible(s string) bool {
	return strings.TrimSpace(ansi.Strip(s)) == ""
}
