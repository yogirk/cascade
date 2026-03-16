package auth

import "net/http"

// RetryOn401 calls fn and retries once if the response status is 401.
// This handles the common case where a cached GCP token has expired
// mid-session. The caller should ensure that fn obtains a fresh token
// on the retry (e.g., via oauth2.ReuseTokenSource which auto-refreshes).
func RetryOn401(fn func() (*http.Response, error)) (*http.Response, error) {
	resp, err := fn()
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusUnauthorized {
		// Close the failed response body before retrying
		resp.Body.Close()
		// Retry once — the token source should provide a fresh token
		return fn()
	}

	return resp, nil
}

// RetryTransport wraps an http.RoundTripper and retries on 401 responses.
// This is useful as middleware for HTTP clients making GCP API calls.
type RetryTransport struct {
	Base http.RoundTripper
}

// RoundTrip implements http.RoundTripper. On 401, it closes the response
// body and retries the request once.
func (t *RetryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.Base.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		return t.Base.RoundTrip(req)
	}

	return resp, nil
}
