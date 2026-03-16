package auth

import (
	"strings"
	"testing"

	"google.golang.org/genai"
)

func TestResolveModelAuth_GeminiAPI(t *testing.T) {
	t.Setenv("GOOGLE_API_KEY", "test-key-123")

	m, err := ResolveModelAuth("gemini_api", "gemini-2.5-pro", nil,
		"", "", "GOOGLE_API_KEY", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if m.Provider != "gemini_api" {
		t.Errorf("expected provider %q, got %q", "gemini_api", m.Provider)
	}
	if m.GenAIConfig == nil {
		t.Fatal("expected GenAIConfig to be set")
	}
	if m.GenAIConfig.Backend != genai.BackendGeminiAPI {
		t.Errorf("expected backend GeminiAPI, got %v", m.GenAIConfig.Backend)
	}
	if m.GenAIConfig.APIKey != "test-key-123" {
		t.Errorf("expected API key from env, got %q", m.GenAIConfig.APIKey)
	}
}

func TestResolveModelAuth_GeminiLegacyAlias(t *testing.T) {
	t.Setenv("GOOGLE_API_KEY", "test-key")

	m, err := ResolveModelAuth("gemini", "gemini-2.5-pro", nil,
		"", "", "GOOGLE_API_KEY", "", "")
	if err != nil {
		t.Fatal(err)
	}
	// Legacy "gemini" should resolve to "gemini_api"
	if m.Provider != "gemini_api" {
		t.Errorf("expected provider %q, got %q", "gemini_api", m.Provider)
	}
}

func TestResolveModelAuth_GeminiMissingKey(t *testing.T) {
	t.Setenv("GOOGLE_API_KEY", "")

	_, err := ResolveModelAuth("gemini_api", "gemini-2.5-pro", nil,
		"", "", "GOOGLE_API_KEY", "", "")
	if err == nil {
		t.Fatal("expected error when API key is missing")
	}
	if !strings.Contains(err.Error(), "GOOGLE_API_KEY") {
		t.Errorf("error should mention env var, got: %v", err)
	}
}

func TestResolveModelAuth_VertexWithResource(t *testing.T) {
	resource := &ResourceAuth{
		Available:   true,
		Project:     "my-gcp-project",
		Location:    "us-central1",
		TokenSource: nil, // TokenSource would be real in production
	}

	m, err := ResolveModelAuth("vertex", "gemini-2.5-pro", resource,
		"", "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if m.Provider != "vertex" {
		t.Errorf("expected provider %q, got %q", "vertex", m.Provider)
	}
	if m.GenAIConfig == nil {
		t.Fatal("expected GenAIConfig")
	}
	if m.GenAIConfig.Backend != genai.BackendVertexAI {
		t.Errorf("expected Vertex backend")
	}
	if m.GenAIConfig.Project != "my-gcp-project" {
		t.Errorf("expected project inherited from resource, got %q", m.GenAIConfig.Project)
	}
	if m.GenAIConfig.Location != "us-central1" {
		t.Errorf("expected location inherited from resource, got %q", m.GenAIConfig.Location)
	}
}

func TestResolveModelAuth_VertexOverridesResource(t *testing.T) {
	resource := &ResourceAuth{
		Available: true,
		Project:   "resource-project",
		Location:  "us-central1",
	}

	m, err := ResolveModelAuth("vertex", "gemini-2.5-pro", resource,
		"vertex-project", "europe-west4", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if m.GenAIConfig.Project != "vertex-project" {
		t.Errorf("expected vertex project override, got %q", m.GenAIConfig.Project)
	}
	if m.GenAIConfig.Location != "europe-west4" {
		t.Errorf("expected vertex location override, got %q", m.GenAIConfig.Location)
	}
}

func TestResolveModelAuth_VertexWithoutResource(t *testing.T) {
	_, err := ResolveModelAuth("vertex", "gemini-2.5-pro", nil,
		"", "", "", "", "")
	if err == nil {
		t.Fatal("expected error when vertex has no GCP auth")
	}
	if !strings.Contains(err.Error(), "GCP auth") {
		t.Errorf("error should mention GCP auth, got: %v", err)
	}
}

func TestResolveModelAuth_AutoDetectVertexNoAPIKey(t *testing.T) {
	t.Setenv("GOOGLE_API_KEY", "")

	resource := &ResourceAuth{
		Available: true,
		Project:   "my-project",
		Location:  "us-central1",
	}

	m, err := ResolveModelAuth("", "gemini-2.5-pro", resource,
		"", "", "GOOGLE_API_KEY", "", "")
	if err != nil {
		t.Fatal(err)
	}
	// No API key, but GCP auth available → vertex
	if m.Provider != "vertex" {
		t.Errorf("expected auto-detected vertex, got %q", m.Provider)
	}
}

func TestResolveModelAuth_AutoDetectPrefersAPIKey(t *testing.T) {
	t.Setenv("GOOGLE_API_KEY", "test-key")

	resource := &ResourceAuth{
		Available: true,
		Project:   "my-project",
		Location:  "us-central1",
	}

	m, err := ResolveModelAuth("", "gemini-2.5-pro", resource,
		"", "", "GOOGLE_API_KEY", "", "")
	if err != nil {
		t.Fatal(err)
	}
	// API key exists → prefer gemini_api even with GCP auth available
	if m.Provider != "gemini_api" {
		t.Errorf("expected auto-detected gemini_api, got %q", m.Provider)
	}
}

func TestResolveModelAuth_AutoDetectGeminiAPI(t *testing.T) {
	t.Setenv("GOOGLE_API_KEY", "test-key")

	// No GCP auth, but API key exists → gemini_api
	m, err := ResolveModelAuth("", "gemini-2.5-pro", nil,
		"", "", "GOOGLE_API_KEY", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if m.Provider != "gemini_api" {
		t.Errorf("expected auto-detected gemini_api, got %q", m.Provider)
	}
}

func TestResolveModelAuth_OpenAI(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-test-123")

	m, err := ResolveModelAuth("openai", "gpt-4", nil,
		"", "", "", "OPENAI_API_KEY", "")
	if err != nil {
		t.Fatal(err)
	}
	if m.Provider != "openai" {
		t.Errorf("expected provider %q, got %q", "openai", m.Provider)
	}
	if m.APIKey != "sk-test-123" {
		t.Errorf("expected API key from env")
	}
	if m.GenAIConfig != nil {
		t.Error("openai should not set GenAIConfig")
	}
}

func TestResolveModelAuth_Anthropic(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-test")

	m, err := ResolveModelAuth("anthropic", "claude-sonnet-4-20250514", nil,
		"", "", "", "", "ANTHROPIC_API_KEY")
	if err != nil {
		t.Fatal(err)
	}
	if m.Provider != "anthropic" {
		t.Errorf("expected provider %q, got %q", "anthropic", m.Provider)
	}
	if m.APIKey != "sk-ant-test" {
		t.Errorf("expected API key from env")
	}
}

func TestResolveModelAuth_UnknownProvider(t *testing.T) {
	_, err := ResolveModelAuth("cohere", "command-r", nil,
		"", "", "", "", "")
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}
