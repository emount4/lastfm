package openrouter

import (
	"encoding/json"
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
	}), "test-key")
	summary := vibe.Summary{Days: 1, Scrobbles: 3, TopTracks: []vibe.TrackCount{{Artist: "Рэйчи", Track: "Кукла", Count: 3}}}
	got, err := client.Describe(t.Context(), summary)
	if err != nil || got != "Сегодня ты ставишь Куклу на репит." {
		t.Fatalf("description = %q, error = %v", got, err)
	}
}
