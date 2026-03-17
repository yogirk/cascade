package auth

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

// mockRoundTripper implements http.RoundTripper for testing.
type mockRoundTripper struct {
	responses []*http.Response
	calls     int
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	idx := m.calls
	if idx >= len(m.responses) {
		idx = len(m.responses) - 1
	}
	m.calls++
	return m.responses[idx], nil
}

func newResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func TestRetryOn401_Retries(t *testing.T) {
	callCount := 0
	fn := func() (*http.Response, error) {
		callCount++
		if callCount == 1 {
			return newResponse(http.StatusUnauthorized, "unauthorized"), nil
		}
		return newResponse(http.StatusOK, "success"), nil
	}

	resp, err := RetryOn401(fn)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
	if callCount != 2 {
		t.Errorf("expected 2 calls, got %d", callCount)
	}
}

func TestRetryOn401_NoRetryOn200(t *testing.T) {
	callCount := 0
	fn := func() (*http.Response, error) {
		callCount++
		return newResponse(http.StatusOK, "ok"), nil
	}

	resp, err := RetryOn401(fn)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}
}

func TestRetryOn401_NoRetryOn403(t *testing.T) {
	callCount := 0
	fn := func() (*http.Response, error) {
		callCount++
		return newResponse(http.StatusForbidden, "forbidden"), nil
	}

	resp, err := RetryOn401(fn)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, resp.StatusCode)
	}
	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}
}

func TestRetryTransport_RetriesOn401(t *testing.T) {
	mock := &mockRoundTripper{
		responses: []*http.Response{
			newResponse(http.StatusUnauthorized, "unauthorized"),
			newResponse(http.StatusOK, "success"),
		},
	}
	transport := &RetryTransport{Base: mock}

	req, _ := http.NewRequest("GET", "https://example.com", nil)
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
	if mock.calls != 2 {
		t.Errorf("expected 2 calls, got %d", mock.calls)
	}
}

func TestRetryTransport_NoRetryOn200(t *testing.T) {
	mock := &mockRoundTripper{
		responses: []*http.Response{
			newResponse(http.StatusOK, "ok"),
		},
	}
	transport := &RetryTransport{Base: mock}

	req, _ := http.NewRequest("GET", "https://example.com", nil)
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
	if mock.calls != 1 {
		t.Errorf("expected 1 call, got %d", mock.calls)
	}
}

func TestRetryTransport_NoRetryOn403(t *testing.T) {
	mock := &mockRoundTripper{
		responses: []*http.Response{
			newResponse(http.StatusForbidden, "forbidden"),
		},
	}
	transport := &RetryTransport{Base: mock}

	req, _ := http.NewRequest("GET", "https://example.com", nil)
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, resp.StatusCode)
	}
	if mock.calls != 1 {
		t.Errorf("expected 1 call, got %d", mock.calls)
	}
}

func TestNewTokenSource_ADC(t *testing.T) {
	// We can't actually call GCP in unit tests, but we can verify
	// the function signature and error message pattern.
	// This test verifies that NewTokenSource exists and accepts the right args.
	// Actual ADC will fail since we're not in a GCP environment,
	// but the error should contain the helpful message.
	if testing.Short() {
		t.Skip("skipping ADC test in short mode")
	}

	// Verify the function exists and has the right signature by calling it.
	// It will fail because there are no default credentials in CI,
	// but the error should be descriptive.
	cfg := &AuthConfig{}
	_, err := NewTokenSource(t.Context(), cfg)
	if err == nil {
		// If it succeeds, we're in a GCP environment -- that's fine too.
		return
	}

	// Should contain helpful error message
	if !strings.Contains(err.Error(), "gcloud auth application-default login") {
		t.Errorf("error should contain fix instructions, got: %v", err)
	}
}

func TestNewTokenSource_Impersonation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping impersonation test in short mode")
	}

	cfg := &AuthConfig{
		ImpersonateServiceAccount: "test@project.iam.gserviceaccount.com",
	}
	_, err := NewTokenSource(t.Context(), cfg)
	if err == nil {
		return // Surprising but acceptable in a real GCP environment
	}

	// Error should mention the service account
	if !strings.Contains(err.Error(), "test@project.iam.gserviceaccount.com") {
		t.Errorf("error should mention target SA, got: %v", err)
	}
}
