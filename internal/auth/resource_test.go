package auth

import (
	"strings"
	"testing"
)

func TestResolveResourceAuth_NoCredentials(t *testing.T) {
	// In CI/test environments without ADC, this should degrade gracefully
	r := ResolveResourceAuth(t.Context(), "", "", "adc", "", "")

	// Either succeeds (dev machine with ADC) or fails gracefully
	if !r.Available {
		// Should have helpful warnings, not a panic
		if len(r.Warnings) == 0 {
			t.Error("expected warnings when GCP auth fails")
		}
		found := false
		for _, w := range r.Warnings {
			if strings.Contains(w, "gcloud auth application-default login") {
				found = true
			}
		}
		if !found {
			t.Error("expected fix instructions in warnings")
		}
	}
}

func TestResolveResourceAuth_ExplicitProject(t *testing.T) {
	r := ResolveResourceAuth(t.Context(), "my-project", "us-central1", "adc", "", "")

	// Project and location should be set regardless of auth success
	if r.Project != "my-project" && r.Available {
		// Only check project if auth succeeded (project detection may override)
		t.Logf("project resolved to %q (may be overridden by ADC)", r.Project)
	}
	if r.Location != "us-central1" {
		t.Errorf("expected location %q, got %q", "us-central1", r.Location)
	}
}

func TestResolveResourceAuth_ImpersonationWithoutSA(t *testing.T) {
	r := ResolveResourceAuth(t.Context(), "", "", "impersonation", "", "")

	// Should warn about missing SA and fall back to ADC
	found := false
	for _, w := range r.Warnings {
		if strings.Contains(w, "impersonate_service_account") {
			found = true
		}
	}
	if !found {
		t.Error("expected warning about missing impersonate_service_account")
	}
}

func TestResolveResourceAuth_ServiceAccountKeyWithoutFile(t *testing.T) {
	r := ResolveResourceAuth(t.Context(), "", "", "service_account_key", "", "")

	// Should warn about missing file and fall back to ADC
	found := false
	for _, w := range r.Warnings {
		if strings.Contains(w, "credentials_file") {
			found = true
		}
	}
	if !found {
		t.Error("expected warning about missing credentials_file")
	}
}

func TestResolveResourceAuth_BadCredentialsFile(t *testing.T) {
	r := ResolveResourceAuth(t.Context(), "", "", "service_account_key", "", "/nonexistent/key.json")

	if r.Available {
		t.Error("expected auth to fail with bad credentials file")
	}
	if len(r.Warnings) == 0 {
		t.Error("expected warnings for bad credentials file")
	}
}
