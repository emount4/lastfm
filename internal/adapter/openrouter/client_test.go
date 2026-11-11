package openrouter

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/emount4/lastfm/internal/application/vibe"
)

type doFunc func(*http.Request) (*http.Response, error)

func (f doFunc) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestDescribeReportsRateLimitDetails(t *testing.T) {
	client := NewClient(doFunc(func(*http.Request) (*http.Response, error) {
		const body = `{"error":{"message":"raw error detail","metadata":{"provider_name":"Google AI Studio","limit_source":"upstream_provider_shared_pool"}}}`
		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Header:     http.Header{"Retry-After": {"60"}},
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	}), "test-key", false)
	_, err := client.Describe(t.Context(), vibe.Summary{})
	if !errors.Is(err, vibe.ErrRateLimited) {
		t.Fatalf("error = %v, want rate limit", err)
	}
	for _, detail := range []string{"429", "Google AI Studio", "upstream_provider_shared_pool", "60"} {
		if !strings.Contains(err.Error(), detail) {
			t.Fatalf("missing %q in %v", detail, err)
		}
	}
	if strings.Contains(err.Error(), "raw error detail") {
		t.Fatalf("raw response was included: %v", err)
	}
}

func TestDescribeCallsFreeGemma(t *testing.T) {
	client := NewClient(doFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.String() != endpoint || req.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("unexpected OpenRouter request: %s %s", req.Method, req.URL)
		}
		var payload struct {
			Model    string `json:"model"`
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload.Model != "google/gemma-4-31b-it:free" || len(payload.Messages) != 1 ||
			!strings.Contains(payload.Messages[0].Content, `"Track":"Кукла","Count":3`) {
			t.Fatalf("unexpected OpenRouter payload: %+v", payload)
		}
		body := `{"choices":[{"message":{"content":" Сегодня ты ставишь Куклу на репит. "}}]}`
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
	}), "test-key", false)
	summary := vibe.Summary{Days: 1, Scrobbles: 3, TopTracks: []vibe.TrackCount{{Artist: "Рэйчи", Track: "Кукла", Count: 3}}}
	got, err := client.Describe(t.Context(), summary)
	if err != nil || got != "Сегодня ты ставишь Куклу на репит." {
		t.Fatalf("description = %q, error = %v", got, err)
	}
}

func TestDescribeTriesFreeFallbackOnlyForProviderLimit(t *testing.T) {
	for _, test := range []struct {
		source string
		retry  bool
	}{
		{source: "upstream_provider_shared_pool", retry: true},
		{source: "openrouter_free_daily_limit"},
	} {
		t.Run(test.source, func(t *testing.T) {
			var models []string
			client := NewClient(doFunc(func(req *http.Request) (*http.Response, error) {
				var payload struct {
					Model string `json:"model"`
				}
				if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
					t.Fatal(err)
				}
				models = append(models, payload.Model)
				if len(models) == 1 {
					body := `{"error":{"metadata":{"limit_source":"` + test.source + `"}}}`
					return &http.Response{StatusCode: http.StatusTooManyRequests, Body: io.NopCloser(strings.NewReader(body))}, nil
				}
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"Готово."}}]}`))}, nil
			}), "test-key", true)
			got, err := client.Describe(t.Context(), vibe.Summary{})
			if len(models) == 0 || models[0] != gemmaModel {
				t.Fatalf("requested models = %v", models)
			}
			if test.retry {
				if err != nil || got != "Готово." || len(models) != 2 || models[1] != freeFallbackModel {
					t.Fatalf("reply = %q, error = %v, models = %v", got, err, models)
				}
			} else if !errors.Is(err, vibe.ErrRateLimited) || len(models) != 1 {
				t.Fatalf("error = %v, models = %v", err, models)
			}
		})
	}
}
