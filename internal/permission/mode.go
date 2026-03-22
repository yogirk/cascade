package permission

import "strings"

// Mode represents the current permission policy profile.
type Mode int

const (
	ModeAsk Mode = iota // Default: reads auto-approve, writes ask
	ModeReadOnly        // Deny non-read operations
	ModeFullAccess      // Allow everything
)

// Legacy aliases preserved for compatibility with older code and planning docs.
const (
	ModeConfirm = ModeAsk
	ModePlan    = ModeReadOnly
	ModeBypass  = ModeFullAccess
)

func (m Mode) String() string {
	switch m {
	case ModeAsk:
		return "ASK"
	case ModeReadOnly:
		return "READ ONLY"
	case ModeFullAccess:
		return "FULL ACCESS"
	default:
		return "ASK"
	}
}

// CycleMode returns the next mode in the cycle:
// ASK -> READ ONLY -> FULL ACCESS -> ASK.
func CycleMode(current Mode) Mode {
	switch current {
	case ModeAsk:
		return ModeReadOnly
	case ModeReadOnly:
		return ModeFullAccess
	case ModeFullAccess:
		return ModeAsk
	default:
		return ModeAsk
	}
}

// ParseMode accepts both the new policy-first names and the legacy aliases.
func ParseMode(raw string) Mode {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "", "ask", "confirm":
		return ModeAsk
	case "read-only", "readonly", "plan":
		return ModeReadOnly
	case "full-access", "full", "bypass":
		return ModeFullAccess
	default:
		return ModeAsk
	}
}
