package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// modelEntry represents an available model for a provider.
type modelEntry struct {
	ID       string // e.g., "gemini-3-flash-preview"
	Friendly string // e.g., "Gemini 3 (Flash)"
	Note     string // e.g., "fastest, cheapest"
}

// availableModels maps provider names to their curated model lists.
var availableModels = map[string][]modelEntry{
	"gemini_api": {
		{ID: "gemini-3.1-pro-preview", Friendly: "Gemini 3.1 (Pro)", Note: "latest, reasoning"},
		{ID: "gemini-3-flash-preview", Friendly: "Gemini 3 (Flash)", Note: "fast, agentic"},
		{ID: "gemini-3.1-flash-lite-preview", Friendly: "Gemini 3.1 (Flash Lite)", Note: "cheapest"},
		{ID: "gemini-2.5-pro", Friendly: "Gemini 2.5 (Pro)", Note: "stable, high quality"},
		{ID: "gemini-2.5-flash", Friendly: "Gemini 2.5 (Flash)", Note: "stable, fast"},
		{ID: "gemini-2.5-flash-lite", Friendly: "Gemini 2.5 (Flash Lite)", Note: "stable, budget"},
	},
	"vertex": {
		{ID: "gemini-3.1-pro-preview", Friendly: "Gemini 3.1 (Pro)", Note: "latest, reasoning — global region"},
		{ID: "gemini-3-flash", Friendly: "Gemini 3 (Flash)", Note: "fast, agentic — global region"},
		{ID: "gemini-3.1-flash-lite", Friendly: "Gemini 3.1 (Flash Lite)", Note: "cheapest — global region"},
		{ID: "gemini-2.5-pro", Friendly: "Gemini 2.5 (Pro)", Note: "stable, high quality"},
		{ID: "gemini-2.5-flash", Friendly: "Gemini 2.5 (Flash)", Note: "stable, fast"},
		{ID: "gemini-2.5-flash-lite", Friendly: "Gemini 2.5 (Flash Lite)", Note: "stable, budget"},
	},
	"openai": {
		{ID: "gpt-4o", Friendly: "GPT-4o", Note: "balanced"},
		{ID: "gpt-4o-mini", Friendly: "GPT-4o Mini", Note: "fast, cheap"},
		{ID: "gpt-4-turbo", Friendly: "GPT-4 Turbo", Note: "high quality"},
		{ID: "o3-mini", Friendly: "o3-mini", Note: "reasoning"},
	},
	"anthropic": {
		{ID: "claude-sonnet-4-5-20250514", Friendly: "Sonnet 4.5", Note: "balanced"},
		{ID: "claude-opus-4-20250514", Friendly: "Opus 4", Note: "highest quality"},
		{ID: "claude-haiku-3-5-20241022", Friendly: "Haiku 3.5", Note: "fast, cheap"},
	},
}

// Styles for model picker rendering.
var (
	mpAccentStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#38BDF8")).Bold(true)
	mpNameStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#F3F4F6"))
	mpIDStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	mpNoteStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#4B5563"))
	mpDimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#4B5563"))
	mpCurStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#34D399"))
	mpProvStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B9FFF")).Bold(true)
)

// ModelPickerModel is an interactive model selector.
type ModelPickerModel struct {
	models       []modelEntry
	providerName string
	cursor       int
	currentModel string
	active       bool
	chosen       string // set when user confirms
}

// NewModelPicker creates a model picker for the given provider.
func NewModelPicker() ModelPickerModel {
	return ModelPickerModel{}
}

// Show activates the picker for the given provider and current model.
func (p *ModelPickerModel) Show(providerName, currentModel string) {
	models, ok := availableModels[providerName]
	if !ok {
		return
	}
	p.models = models
	p.providerName = providerName
	p.currentModel = currentModel
	p.active = true
	p.chosen = ""

	// Set cursor to current model
	p.cursor = 0
	for i, m := range models {
		if m.ID == currentModel {
			p.cursor = i
			break
		}
	}
}

// Active returns whether the picker is visible.
func (p *ModelPickerModel) Active() bool {
	return p.active
}

// Chosen returns the selected model ID after confirmation, or empty string.
func (p *ModelPickerModel) Chosen() string {
	return p.chosen
}

// Dismiss hides the picker without selecting.
func (p *ModelPickerModel) Dismiss() {
	p.active = false
	p.chosen = ""
}

// MoveUp moves the cursor up.
func (p *ModelPickerModel) MoveUp() {
	if p.cursor > 0 {
		p.cursor--
	}
}

// MoveDown moves the cursor down.
func (p *ModelPickerModel) MoveDown() {
	if p.cursor < len(p.models)-1 {
		p.cursor++
	}
}

// Confirm selects the current cursor position.
func (p *ModelPickerModel) Confirm() {
	if p.cursor >= 0 && p.cursor < len(p.models) {
		p.chosen = p.models[p.cursor].ID
	}
	p.active = false
}

// Height returns the rendered height of the picker.
func (p *ModelPickerModel) Height() int {
	if !p.active {
		return 0
	}
	return len(p.models) + 5 // header + models + footer + padding
}

// View renders the interactive model picker.
func (p ModelPickerModel) View() string {
	if !p.active || len(p.models) == 0 {
		return ""
	}

	var sb strings.Builder

	providerFriendly := p.providerName
	switch p.providerName {
	case "gemini_api":
		providerFriendly = "Gemini API"
	case "vertex":
		providerFriendly = "Vertex AI"
	case "openai":
		providerFriendly = "OpenAI"
	case "anthropic":
		providerFriendly = "Anthropic"
	}

	sb.WriteString("  " + mpProvStyle.Render("Select Model") + "  " +
		mpIDStyle.Render(providerFriendly) + "\n\n")

	for i, m := range p.models {
		isCurrent := m.ID == p.currentModel
		isSelected := i == p.cursor

		var prefix string
		if isSelected {
			prefix = mpAccentStyle.Render("  ►")
		} else {
			prefix = "   "
		}

		name := mpNameStyle.Render(m.Friendly)
		id := mpIDStyle.Render(m.ID)
		note := mpNoteStyle.Render(m.Note)
		if isSelected {
			name = mpAccentStyle.Render(m.Friendly)
		}

		line := fmt.Sprintf("%s  %s  %s  %s", prefix, name, id, note)

		if isCurrent {
			line += "  " + mpCurStyle.Render("current")
		}

		sb.WriteString(line + "\n")
	}

	sb.WriteString("\n  " + mpDimStyle.Render("↑↓ select   Enter confirm   Esc cancel"))

	return sb.String()
}

// resolveModelByNumber checks if input is a number and returns the model ID.
func resolveModelByNumber(providerName, input string) string {
	models, ok := availableModels[providerName]
	if !ok {
		return ""
	}

	var num int
	if _, err := fmt.Sscanf(input, "%d", &num); err != nil {
		return ""
	}

	if num < 1 || num > len(models) {
		return ""
	}

	return models[num-1].ID
}
