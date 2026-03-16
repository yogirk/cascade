package permission

// RiskLevel classifies the risk of a tool operation.
type RiskLevel int

const (
	RiskReadOnly    RiskLevel = iota // No side effects
	RiskDML                          // Creates/modifies data
	RiskDDL                          // Schema changes
	RiskDestructive                  // Deletes data or resources
	RiskAdmin                        // Administrative operations
)

func (r RiskLevel) String() string {
	switch r {
	case RiskReadOnly:
		return "READ_ONLY"
	case RiskDML:
		return "DML"
	case RiskDDL:
		return "DDL"
	case RiskDestructive:
		return "DESTRUCTIVE"
	case RiskAdmin:
		return "ADMIN"
	default:
		return "UNKNOWN"
	}
}

// Badge returns a styled badge string for display in the TUI.
func (r RiskLevel) Badge() string {
	switch r {
	case RiskReadOnly:
		return "[READ]"
	case RiskDML:
		return "[DML]"
	case RiskDDL:
		return "[DDL]"
	case RiskDestructive:
		return "[DESTRUCTIVE]"
	case RiskAdmin:
		return "[ADMIN]"
	default:
		return "[UNKNOWN]"
	}
}

// RequiresConfirmation returns true if the risk level requires user confirmation
// (DML and above).
func RequiresConfirmation(level RiskLevel) bool {
	return level >= RiskDML
}
