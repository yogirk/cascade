package app

import (
	"strings"

	"github.com/yogirk/cascade/internal/schema"
)

const baseSystemPrompt = `You are Cascade, an AI assistant for GCP data engineering. You help users diagnose pipeline failures, investigate costs, write queries, and manage their GCP data stack through conversational interaction.`

// BuildSystemPrompt creates the system prompt, optionally including BigQuery context.
func BuildSystemPrompt(bqComp *BigQueryComponents) string {
	if bqComp == nil || bqComp.Cache == nil || !bqComp.Cache.IsPopulated() {
		return baseSystemPrompt
	}

	summary, err := schema.BuildDatasetSummary(bqComp.Cache)
	if err != nil || summary == "" {
		return baseSystemPrompt
	}

	var sb strings.Builder
	sb.WriteString(baseSystemPrompt)
	sb.WriteString("\n\n## BigQuery Context\n\n")
	sb.WriteString(summary)
	sb.WriteString("\n\nWhen the user asks about data, tables, or queries, use the bigquery_schema tool to look up table details and the bigquery_query tool to execute SQL. Always estimate cost before executing queries.")

	return sb.String()
}
