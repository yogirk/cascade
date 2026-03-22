package auth

import (
	"fmt"
	"os"

	"golang.org/x/oauth2"
	"google.golang.org/genai"
)

// ModelAuth holds the resolved LLM provider credentials.
type ModelAuth struct {
	// Provider is the resolved provider name.
	Provider string

	// GenAIConfig is set for vertex and gemini_api providers (GenAI SDK).
	GenAIConfig *genai.ClientConfig

	// APIKey is set for API-key providers (openai, anthropic).
	APIKey string

	// Warnings collected during resolution.
	Warnings []string
}

// ResolveModelAuth builds LLM provider credentials.
// For vertex, it reuses ResourceAuth credentials (the elegant inheritance).
func ResolveModelAuth(provider, model string, resource *ResourceAuth,
	vertexProject, vertexLocation string,
	geminiKeyEnv, openaiKeyEnv, anthropicKeyEnv string,
) (*ModelAuth, error) {

	m := &ModelAuth{}

	// Auto-detect provider if not specified
	if provider == "" {
		provider = autoDetectProvider(resource, geminiKeyEnv)
		m.Warnings = append(m.Warnings, fmt.Sprintf("Auto-detected provider: %s", provider))
	}
	m.Provider = provider

	switch provider {
	case "vertex":
		return resolveVertex(m, resource, vertexProject, vertexLocation)
	case "gemini_api", "gemini": // accept legacy "gemini" alias
		m.Provider = "gemini_api"
		return resolveAPIKeyGenAI(m, geminiKeyEnv, genai.BackendGeminiAPI)
	case "openai":
		return resolveAPIKey(m, openaiKeyEnv, "openai")
	case "anthropic":
		return resolveAPIKey(m, anthropicKeyEnv, "anthropic")
	default:
		return nil, fmt.Errorf("unknown provider %q: use vertex, gemini_api, openai, or anthropic", provider)
	}
}

func autoDetectProvider(resource *ResourceAuth, geminiKeyEnv string) string {
	// Prefer API key if available (cheaper, no Vertex AI enablement needed)
	if envVal := os.Getenv(geminiKeyEnv); envVal != "" {
		return "gemini_api"
	}
	// Fall back to vertex if GCP auth is available (covers Model Garden too)
	if resource != nil && resource.Available {
		return "vertex"
	}
	// Default to vertex (will fail with helpful error if no GCP auth)
	return "vertex"
}

func resolveVertex(m *ModelAuth, resource *ResourceAuth, vertexProject, vertexLocation string) (*ModelAuth, error) {
	if resource == nil || !resource.Available {
		return nil, fmt.Errorf("vertex provider requires GCP auth, but no credentials available.\n" +
			"Fix: run 'gcloud auth application-default login' or configure [gcp.auth]")
	}

	project := vertexProject
	if project == "" {
		project = resource.Project
	}
	if project == "" {
		return nil, fmt.Errorf("vertex provider requires a GCP project.\n" +
			"Set [gcp] project, [model.vertex] project, or GOOGLE_CLOUD_PROJECT")
	}

	location := vertexLocation
	if location == "" {
		location = resource.Location
	}
	if location == "" {
		location = "us-central1"
	}

	m.GenAIConfig = &genai.ClientConfig{
		Backend:    genai.BackendVertexAI,
		HTTPClient: oauth2.NewClient(nil, resource.TokenSource),
		Project:    project,
		Location:   location,
	}
	return m, nil
}

func resolveAPIKeyGenAI(m *ModelAuth, envName string, backend genai.Backend) (*ModelAuth, error) {
	key := os.Getenv(envName)
	if key == "" {
		return nil, fmt.Errorf("%s env var required for %s provider", envName, m.Provider)
	}
	m.APIKey = key
	m.GenAIConfig = &genai.ClientConfig{
		Backend: backend,
		APIKey:  key,
	}
	return m, nil
}

func resolveAPIKey(m *ModelAuth, envName string, providerName string) (*ModelAuth, error) {
	key := os.Getenv(envName)
	if key == "" {
		return nil, fmt.Errorf("%s env var required for %s provider", envName, providerName)
	}
	m.APIKey = key
	return m, nil
}
