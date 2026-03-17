// Package auth provides GCP authentication utilities for Cascade.
// It uses Application Default Credentials (ADC) with optional
// service account impersonation.
package auth

import (
	"context"
	"fmt"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/impersonate"
)

var defaultScopes = []string{
	"https://www.googleapis.com/auth/cloud-platform",
}

// AuthConfig mirrors config.AuthConfig to avoid circular imports.
// The caller converts from config.AuthConfig to this type.
type AuthConfig struct {
	ImpersonateServiceAccount string
}

// NewTokenSource creates an oauth2.TokenSource for GCP API calls.
// When ImpersonateServiceAccount is empty, it uses ADC (Application Default
// Credentials). When set, it creates an impersonated token source.
// The result is wrapped in oauth2.ReuseTokenSource for automatic caching.
func NewTokenSource(ctx context.Context, cfg *AuthConfig) (oauth2.TokenSource, error) {
	var ts oauth2.TokenSource

	if cfg.ImpersonateServiceAccount != "" {
		// Service account impersonation
		impTS, err := impersonate.CredentialsTokenSource(ctx,
			impersonate.CredentialsConfig{
				TargetPrincipal: cfg.ImpersonateServiceAccount,
				Scopes:          defaultScopes,
			})
		if err != nil {
			return nil, fmt.Errorf("failed to create impersonated credentials: %w\n"+
				"Ensure you have roles/iam.serviceAccountTokenCreator on %s",
				err, cfg.ImpersonateServiceAccount)
		}
		ts = impTS
	} else {
		// Standard ADC
		creds, err := google.FindDefaultCredentials(ctx, defaultScopes...)
		if err != nil {
			return nil, fmt.Errorf("GCP auth failed. Run: gcloud auth application-default login\n%w", err)
		}
		ts = creds.TokenSource
	}

	return oauth2.ReuseTokenSource(nil, ts), nil
}
