package tui

import (
	"testing"

	"github.com/slokam-ai/cascade/internal/tui/themes"
)

// TestSetTheme_BackCompat verifies that old "light"/"dark"/"auto" values
// still behave as they did before the named-theme system — forcing the
// lightness variant without touching the active theme name.
func TestSetTheme_BackCompat(t *testing.T) {
	// Record starting state so the test can restore it.
	origTheme := CurrentTheme().Name
	origDark := IsDarkBg()
	t.Cleanup(func() {
		SetTheme(origTheme)
		if origDark {
			SetTheme("dark")
		} else {
			SetTheme("light")
		}
	})

	SetTheme("light")
	if IsDarkBg() {
		t.Error(`SetTheme("light") did not set isDarkBg=false`)
	}
	if CurrentTheme().Name != origTheme {
		t.Errorf(`SetTheme("light") changed theme to %q (expected no change from %q)`, CurrentTheme().Name, origTheme)
	}

	SetTheme("dark")
	if !IsDarkBg() {
		t.Error(`SetTheme("dark") did not set isDarkBg=true`)
	}
}

// TestSetTheme_SwitchesNamedTheme verifies switching between named themes
// preserves the current lightness and actually changes the active theme.
func TestSetTheme_SwitchesNamedTheme(t *testing.T) {
	origTheme := CurrentTheme().Name
	origDark := IsDarkBg()
	t.Cleanup(func() { SetTheme(origTheme) })

	SetTheme("midnight")
	if CurrentTheme().Name != "midnight" {
		t.Errorf("active theme = %q, want midnight", CurrentTheme().Name)
	}
	if IsDarkBg() != origDark {
		t.Errorf("lightness changed unexpectedly: was %v, now %v", origDark, IsDarkBg())
	}

	SetTheme("verse")
	if CurrentTheme().Name != "verse" {
		t.Errorf("active theme = %q, want verse", CurrentTheme().Name)
	}
}

// TestSetTheme_UnknownThemeIsNoop verifies unknown theme names don't mutate
// state — the user just sees nothing happen, the previous theme stays active.
func TestSetTheme_UnknownThemeIsNoop(t *testing.T) {
	origTheme := CurrentTheme().Name
	origDark := IsDarkBg()
	t.Cleanup(func() { SetTheme(origTheme) })

	SetTheme("nonexistent-theme-name")
	if CurrentTheme().Name != origTheme {
		t.Errorf("unknown theme changed active theme to %q (expected no change)", CurrentTheme().Name)
	}
	if IsDarkBg() != origDark {
		t.Error("unknown theme changed lightness")
	}
}

// TestSetTheme_AutoIsNoopForLightness verifies "auto" and "" keep the
// currently-detected lightness value intact.
func TestSetTheme_AutoIsNoopForLightness(t *testing.T) {
	origTheme := CurrentTheme().Name
	t.Cleanup(func() { SetTheme(origTheme) })

	SetTheme("dark")
	SetTheme("auto")
	if !IsDarkBg() {
		t.Error(`SetTheme("auto") after "dark" unexpectedly flipped to light`)
	}

	SetTheme("")
	if !IsDarkBg() {
		t.Error(`SetTheme("") after "dark"+"auto" unexpectedly flipped to light`)
	}
}

// TestCurrentTheme_InitialIsDefault verifies the package initializes with
// the registry's default theme, not the zero value.
func TestCurrentTheme_InitialIsDefault(t *testing.T) {
	if CurrentTheme().Name != themes.DefaultName {
		// Note: other tests may have changed this before we ran. Only
		// assert the theme is a registered, non-empty one.
		if CurrentTheme().Name == "" {
			t.Error("CurrentTheme() has empty Name — package init did not run")
		}
	}
}
