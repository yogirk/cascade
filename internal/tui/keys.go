package tui

// KeyDef defines a keyboard shortcut with its key string and description.
// In Bubble Tea v2, key matching is done via KeyPressMsg.String() comparison.
type KeyDef struct {
	Keys []string // Key strings as returned by KeyPressMsg.String()
	Help string   // Description for help display
}

// Matches returns true if the given key string matches this binding.
func (k KeyDef) Matches(key string) bool {
	for _, s := range k.Keys {
		if s == key {
			return true
		}
	}
	return false
}

// KeyMap defines the keyboard shortcuts for the TUI.
type KeyMap struct {
	Submit      KeyDef // Enter: submit input
	Newline     KeyDef // Shift+Enter: add newline in input
	Cancel      KeyDef // Ctrl+C: cancel current operation / quit if idle
	Exit        KeyDef // Ctrl+D: exit application
	ClearScreen KeyDef // Ctrl+L: clear screen
	CycleMode   KeyDef // Shift+Tab: cycle permission mode
	Background  KeyDef // Ctrl+B: background (stub in Phase 1)
	Refresh     KeyDef // Ctrl+R: refresh cache (stub in Phase 1)
}

// DefaultKeyMap returns the default key bindings for Cascade.
// Key strings match Bubble Tea v2's KeyPressMsg.String() output.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Submit: KeyDef{
			Keys: []string{"enter"},
			Help: "submit",
		},
		Newline: KeyDef{
			Keys: []string{"shift+enter"},
			Help: "newline",
		},
		Cancel: KeyDef{
			Keys: []string{"ctrl+c"},
			Help: "cancel/quit",
		},
		Exit: KeyDef{
			Keys: []string{"ctrl+d"},
			Help: "exit",
		},
		ClearScreen: KeyDef{
			Keys: []string{"ctrl+l"},
			Help: "clear screen",
		},
		CycleMode: KeyDef{
			// Shift+Tab is represented as "shift+tab" in Bubble Tea v2
			Keys: []string{"shift+tab"},
			Help: "cycle permission mode",
		},
		Background: KeyDef{
			Keys: []string{"ctrl+b"},
			Help: "background (not implemented)",
		},
		Refresh: KeyDef{
			Keys: []string{"ctrl+r"},
			Help: "refresh cache (not implemented)",
		},
	}
}
