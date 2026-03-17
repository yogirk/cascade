package permission

// Mode represents the current permission enforcement mode.
type Mode int

const (
	ModeConfirm Mode = iota // Default: reads auto-approve, writes confirm
	ModePlan                // Read-only: everything non-read denied
	ModeBypass              // Auto-approve everything
)

func (m Mode) String() string {
	switch m {
	case ModeConfirm:
		return "CONFIRM"
	case ModePlan:
		return "PLAN"
	case ModeBypass:
		return "BYPASS"
	default:
		return "UNKNOWN"
	}
}

// CycleMode returns the next mode in the cycle: CONFIRM -> PLAN -> BYPASS -> CONFIRM.
func CycleMode(current Mode) Mode {
	switch current {
	case ModeConfirm:
		return ModePlan
	case ModePlan:
		return ModeBypass
	case ModeBypass:
		return ModeConfirm
	default:
		return ModeConfirm
	}
}
