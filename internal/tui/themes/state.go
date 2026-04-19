package themes

import (
	"fmt"
	"image/color"
	"sync"
)

// Active palette state. Kept in this package (not in internal/tui) because
// non-tui packages like internal/app and internal/tools/* need to read the
// live palette too, and Go's layering prevents them from importing tui.
//
// Writers: internal/tui/styles.go calls SetActive from initPalette/SetTheme.
// Readers: any renderer can call ActivePalette() to build styles that respect
// the current theme.
var (
	stateMu        sync.RWMutex
	activeTheme    = Default()
	activeIsDark   = true // conservative default before HasDarkBackground runs
)

// SetActive records the currently active theme and lightness. Called by the
// TUI layer whenever the theme changes (startup, /theme, --theme flag).
func SetActive(t Theme, isDark bool) {
	stateMu.Lock()
	activeTheme = t
	activeIsDark = isDark
	stateMu.Unlock()
}

// ActivePalette returns the Palette of the current theme for the current
// lightness. Cheap — called by renderers on every render, so no allocations
// here beyond struct return.
func ActivePalette() Palette {
	stateMu.RLock()
	defer stateMu.RUnlock()
	return activeTheme.Pick(activeIsDark)
}

// ActiveTheme returns the currently active Theme (both variants). Exported
// as part of the theme inspection API — the deadcode analyzer flags this as
// unreachable because in-tree code only needs ActivePalette, but the
// symmetrical accessor is here for tests and future callers that want the
// theme metadata (Name, DisplayName, Description) not just the colors.
func ActiveTheme() Theme {
	stateMu.RLock()
	defer stateMu.RUnlock()
	return activeTheme
}

// ActiveIsDark reports whether the rendered variant is the Dark palette.
func ActiveIsDark() bool {
	stateMu.RLock()
	defer stateMu.RUnlock()
	return activeIsDark
}

// Hex converts any color.Color (including lipgloss.Color results) to a
// "#rrggbb" string suitable for callers that take hex strings — notably
// Glamour's ansi.StylePrimitive.Color, which accepts hex but not lipgloss
// color values directly.
func Hex(c color.Color) string {
	if c == nil {
		return ""
	}
	r, g, b, _ := c.RGBA()
	// RGBA returns 16-bit per channel; we want 8-bit.
	return fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8)
}
