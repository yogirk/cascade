package auth

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/impersonate"
)

var defaultScopes = []string{
	"https://www.googleapis.com/auth/cloud-platform",
}

// ResourceAuth holds the resolved GCP platform credentials.
// Platform tools (BigQuery, GCS, Logging, Composer) and Vertex AI consume this.
type ResourceAuth struct {
	// TokenSource provides GCP OAuth2 tokens. Nil if GCP auth failed.
	TokenSource oauth2.TokenSource

	// Project is the resolved GCP project ID.
	Project string

	// Location is the default GCP region (e.g. "us-central1").
	Location string

	// Available is true if GCP credentials were successfully resolved.
	Available bool

	// Warnings collected during resolution (non-fatal issues).
	Warnings []string
}

// ResolveResourceAuth builds GCP platform credentials from config.
// Never returns an error — failures produce a ResourceAuth with Available=false
// and descriptive Warnings, so platform tools degrade gracefully.
func ResolveResourceAuth(ctx context.Context, project, location, mode, impersonateSA, credentialsFile string) *ResourceAuth {
	r := &ResourceAuth{
		Location: location,
	}

	// Build token source based on auth mode
	var ts oauth2.TokenSource
	var err error

	switch mode {
	case "impersonation":
		if impersonateSA == "" {
			r.Warnings = append(r.Warnings, "gcp.auth.mode is 'impersonation' but no impersonate_service_account set; falling back to ADC")
			ts, err = adcTokenSource(ctx)
		} else {
			ts, err = impersonationTokenSource(ctx, impersonateSA)
			if err != nil {
				r.Warnings = append(r.Warnings, fmt.Sprintf("Impersonation failed: %v; falling back to ADC", err))
				ts, err = adcTokenSource(ctx)
			}
		}

	case "service_account_key":
		if credentialsFile == "" {
			r.Warnings = append(r.Warnings, "gcp.auth.mode is 'service_account_key' but no credentials_file set; falling back to ADC")
			ts, err = adcTokenSource(ctx)
		} else {
			ts, err = serviceAccountTokenSource(ctx, credentialsFile)
			if err != nil {
				r.Warnings = append(r.Warnings, fmt.Sprintf("Service account key failed: %v", err))
			}
		}

	default: // "adc" or empty
		ts, err = adcTokenSource(ctx)
	}

	if err != nil {
		r.Warnings = append(r.Warnings, fmt.Sprintf("GCP auth failed: %v", err))
		r.Warnings = append(r.Warnings, "BigQuery, GCS, Logging, and Composer tools will be unavailable")
		r.Warnings = append(r.Warnings, "Fix: run 'gcloud auth application-default login'")
		return r
	}

	r.TokenSource = ts
	r.Available = true

	// Resolve project: explicit config > ADC credentials > env var > gcloud CLI
	r.Project = project
	if r.Project == "" {
		r.Project = detectProject(ctx)
	}
	if r.Project == "" {
		r.Warnings = append(r.Warnings, "No GCP project detected; set [gcp] project or GOOGLE_CLOUD_PROJECT")
	}

	return r
}

func adcTokenSource(ctx context.Context) (oauth2.TokenSource, error) {
	creds, err := google.FindDefaultCredentials(ctx, defaultScopes...)
	if err != nil {
		return nil, err
	}
	return oauth2.ReuseTokenSource(nil, creds.TokenSource), nil
}

func impersonationTokenSource(ctx context.Context, targetSA string) (oauth2.TokenSource, error) {
	ts, err := impersonate.CredentialsTokenSource(ctx, impersonate.CredentialsConfig{
		TargetPrincipal: targetSA,
		Scopes:          defaultScopes,
	})
	if err != nil {
		return nil, fmt.Errorf("impersonation of %s failed (need roles/iam.serviceAccountTokenCreator): %w", targetSA, err)
	}
	return oauth2.ReuseTokenSource(nil, ts), nil
}

func serviceAccountTokenSource(ctx context.Context, credentialsFile string) (oauth2.TokenSource, error) {
	data, err := os.ReadFile(credentialsFile)
	if err != nil {
		return nil, fmt.Errorf("reading credentials file %s: %w", credentialsFile, err)
	}
	// Pin the expected credential type so a JSON file claiming to be e.g.
	// external_account or impersonated_service_account is rejected instead of
	// being loaded with its broader surface area. We only accept service account
	// keys on this path.
	creds, err := google.CredentialsFromJSONWithType(ctx, data, google.ServiceAccount, defaultScopes...)
	if err != nil {
		return nil, fmt.Errorf("parsing credentials file %s: %w", credentialsFile, err)
	}
	return oauth2.ReuseTokenSource(nil, creds.TokenSource), nil
}

// TokenSourceFromKeyFile returns an OAuth2 token source from a service account key file.
// Used for cross-project billing auth when the billing project requires different credentials.
func TokenSourceFromKeyFile(ctx context.Context, credentialsFile string) (oauth2.TokenSource, error) {
	return serviceAccountTokenSource(ctx, credentialsFile)
}

// detectProject resolves a GCP project ID from ADC, env vars, or gcloud CLI.
func detectProject(ctx context.Context) string {
	// Try ADC credentials
	if creds, err := google.FindDefaultCredentials(ctx, defaultScopes...); err == nil && creds.ProjectID != "" {
		return creds.ProjectID
	}
	// Try env vars
	if p := os.Getenv("GOOGLE_CLOUD_PROJECT"); p != "" {
		return p
	}
	if p := os.Getenv("GCLOUD_PROJECT"); p != "" {
		return p
	}
	// Try gcloud CLI
	out, err := exec.Command("gcloud", "config", "get-value", "project").Output()
	if err == nil {
		if p := strings.TrimSpace(string(out)); p != "" && p != "(unset)" {
			return p
		}
	}
	return ""
}
