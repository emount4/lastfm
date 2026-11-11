package lastfm

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestCheckScrobbleAccess(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr string
		apiCode int
	}{
		{name: "with tracks", body: `{"recenttracks":{"track":[{"name":"Example Track"}]}}`},
		{name: "empty history", body: `{"recenttracks":{"track":[]}}`},
		{name: "missing track list", body: `{"recenttracks":{}}`, wantErr: "response did not include a track list"},
		{name: "access denied", body: `{"error":17,"message":"Login required"}`, wantErr: "API error 17", apiCode: 17},
		{name: "temporary failure", body: `{"error":16,"message":"Temporary failure"}`, wantErr: "API error 16", apiCode: 16},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(doFunc(func(req *http.Request) (*http.Response, error) {
				query := req.URL.Query()
				if query.Get("method") != "user.getRecentTracks" || query.Get("limit") != "1" ||
					query.Get("user") != "Example_User" {
					t.Fatalf("unexpected request query: %v", query)
				}
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(tt.body))}, nil
			}), "test-key")

			err := client.CheckScrobbleAccess(t.Context(), " Example_User ")
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("expected available scrobbles, got %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
			if tt.apiCode != 0 {
				var failure *apiError
				if !errors.As(err, &failure) || failure.Code != tt.apiCode {
					t.Fatalf("expected wrapped API error %d, got %v", tt.apiCode, err)
				}
			}
		})
	}
}

func TestCheckScrobbleAccessWrapsDecodeError(t *testing.T) {
	client := NewClient(doFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("{"))}, nil
	}), "test-key")

	err := client.CheckScrobbleAccess(t.Context(), "Example_User")
	var syntaxErr *json.SyntaxError
	if !errors.As(err, &syntaxErr) {
		t.Fatalf("expected wrapped JSON syntax error, got %v", err)
	}
}
