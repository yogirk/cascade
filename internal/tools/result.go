package tools

// Result holds the output of a tool execution.
type Result struct {
	Content string // Text result for LLM context
	Display string // Formatted output for TUI (may include ANSI)
	IsError bool   // Whether this is an error result
}
