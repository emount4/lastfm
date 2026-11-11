package lastfm

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

type doFunc func(*http.Request) (*http.Response, error)

func (do doFunc) Do(req *http.Request) (*http.Response, error) {
	return do(req)
}

func TestLookupProfile(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		profile string
		wantErr string
	}{
		{name: "canonical name", status: 200, body: `{"user":{"name":"Example_User"}}`, profile: "Example_User"},
		{name: "unknown user", status: 404, body: `{"error":6,"message":"User not found"}`},
		{name: "unknown user with OK status", status: 200, body: `{"error":6,"message":"User not found"}`},
		{name: "different error with same code", status: 400, body: `{"error":6,"message":"Invalid parameters"}`, wantErr: "API error 6"},
		{name: "invalid key", status: 200, body: `{"error":10,"message":"Invalid API key"}`, wantErr: "API error 10"},
		{name: "upstream failure", status: 503, body: "Service unavailable", wantErr: "HTTP status 503"},
		{name: "invalid JSON", status: 200, body: "{", wantErr: "invalid JSON response"},
		{name: "missing name", status: 200, body: `{"user":{}}`, wantErr: "response did not include a username"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(doFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet || req.URL.Scheme != "https" ||
					req.URL.Host != "ws.audioscrobbler.com" || req.URL.Path != "/2.0/" {
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL)
				}
				query := req.URL.Query()
				if query.Get("method") != "user.getInfo" || query.Get("user") != "example_user" ||
					query.Get("format") != "json" || query.Get("api_key") != "test-key" {
					t.Fatalf("unexpected query: %v", query)
				}
				return &http.Response{
					StatusCode: tt.status,
					Body:       io.NopCloser(strings.NewReader(tt.body)),
				}, nil
			}), "test-key")

			name, found, err := client.LookupProfile(t.Context(), " example_user ")
			if tt.wantErr == "" {
				if err != nil || name != tt.profile || found != (tt.profile != "") {
					t.Fatalf("got (%q, %t, %v), want (%q, %t, nil)", name, found, err, tt.profile, tt.profile != "")
				}
			} else if err == nil || !strings.Contains(err.Error(), tt.wantErr) || found || name != "" {
				t.Fatalf("got (%q, %t, %v), want error containing %q", name, found, err, tt.wantErr)
			}
		})
	}
}

func TestLookupProfileTransportErrorsHideAPIKey(t *testing.T) {
	const apiKey = "test-secret-key"
	for _, cause := range []error{errors.New("transport failed"), context.Canceled, context.DeadlineExceeded} {
		t.Run(cause.Error(), func(t *testing.T) {
			client := NewClient(doFunc(func(req *http.Request) (*http.Response, error) {
				return nil, &url.Error{Op: req.Method, URL: req.URL.String(), Err: cause}
			}), apiKey)

			_, _, err := client.LookupProfile(t.Context(), "example_user")
			if err == nil || strings.Contains(err.Error(), apiKey) {
				t.Fatalf("expected a sanitized transport error, got %v", err)
			}
			if (cause == context.Canceled || cause == context.DeadlineExceeded) && !errors.Is(err, cause) {
				t.Fatalf("expected %v to wrap %v", err, cause)
			}
		})
	}
}
