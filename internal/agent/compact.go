package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/yogirk/cascade/internal/provider"
	"github.com/yogirk/cascade/pkg/types"
)

const compactionPrompt = `Summarize the conversation so far into a concise context summary. Preserve these details exactly:
- Table names, column names, and schema details mentioned
- SQL queries written or discussed (include the SQL text)
- Cost figures and budget information
- Decisions made and their rationale
- Any errors encountered and how they were resolved

Format as a structured summary the assistant can use to continue the conversation seamlessly. Do NOT include conversational filler. Be precise and factual.`

// CompactSession summarizes older messages to reduce context size.
// It keeps the most recent recentKeep messages intact and summarizes the rest.
// Returns the new message list and the summary text.
func CompactSession(ctx context.Context, prov provider.Provider, messages []types.Message, recentKeep int) ([]types.Message, string, error) {
	if len(messages) <= recentKeep+1 {
		// Nothing to compact (recentKeep + system prompt)
		return messages, "", nil
	}

	// Split: system prompt | older messages | recent messages
	var systemMsg *types.Message
	startIdx := 0
	if len(messages) > 0 && messages[0].Role == types.RoleSystem {
		systemMsg = &messages[0]
		startIdx = 1
	}

	cutoff := len(messages) - recentKeep
	if cutoff <= startIdx {
		return messages, "", nil
	}

	olderMessages := messages[startIdx:cutoff]
	recentMessages := messages[cutoff:]

	// Build compaction request
	compactMessages := make([]types.Message, 0, len(olderMessages)+2)
	if systemMsg != nil {
		compactMessages = append(compactMessages, *systemMsg)
	}
	compactMessages = append(compactMessages, olderMessages...)
	compactMessages = append(compactMessages, types.UserMessage(compactionPrompt))

	// Call LLM to generate summary (non-streaming, no tools)
	stream, err := prov.GenerateStream(ctx, compactMessages, nil)
	if err != nil {
		return messages, "", fmt.Errorf("compaction LLM call failed: %w", err)
	}

	// Drain tokens (we don't display them)
	go func() {
		for range stream.Tokens() {
		}
	}()

	response, err := stream.Result()
	if err != nil {
		return messages, "", fmt.Errorf("compaction LLM response failed: %w", err)
	}

	summary := strings.TrimSpace(response.Text)

	// Build new message list: summary as system message + recent messages
	newMessages := make([]types.Message, 0, len(recentMessages)+2)
	newMessages = append(newMessages, types.SystemMessage("## Conversation Summary (compacted)\n\n"+summary))
	newMessages = append(newMessages, recentMessages...)

	return newMessages, summary, nil
}

// ShouldCompact returns true if current context usage exceeds the threshold.
// lastPromptTokens is the most recent prompt token count from the LLM response.
// model is used to determine the context window size.
func ShouldCompact(lastPromptTokens int32, model string, thresholdPct float64) bool {
	ctxSize := contextWindowForModel(model)
	if ctxSize <= 0 {
		return false
	}
	usage := float64(lastPromptTokens) / float64(ctxSize) * 100
	return usage >= thresholdPct
}

// contextWindowForModel returns the context window size for known models.
func contextWindowForModel(model string) int32 {
	switch {
	case strings.Contains(model, "gemini-2.5"), strings.Contains(model, "gemini-2.0"):
		return 1_000_000
	case strings.Contains(model, "gemini-1.5-pro"):
		return 2_000_000
	case strings.Contains(model, "gemini-1.5-flash"):
		return 1_000_000
	default:
		return 1_000_000
	}
}
