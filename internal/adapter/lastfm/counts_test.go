package lastfm

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/emount4/lastfm/internal/domain"
)

func TestPlayCount(t *testing.T) {
	tests := []struct {
		kind  domain.MusicKind
		title string
		body  string
	}{
		{kind: domain.Artist, body: `{"artist":{"stats":{"playcount":"20000000","userplaycount":"42"}}}`},
		{kind: domain.Track, title: "Dog Days Are Over", body: `{"track":{"playcount":"20000000","userplaycount":"42"}}`},
		{kind: domain.Album, title: "Lungs", body: `{"album":{"playcount":"20000000","userplaycount":42}}`},
	}
	for _, tt := range tests {
		t.Run(string(tt.kind), func(t *testing.T) {
			client := NewClient(doFunc(func(req *http.Request) (*http.Response, error) {
				query := req.URL.Query()
				if query.Get("method") != string(tt.kind)+".getInfo" || query.Get("username") != "listener" ||
					query.Get("artist") != "Florence + The Machine" || query.Get("autocorrect") != "1" ||
					tt.kind != domain.Artist && query.Get(string(tt.kind)) != tt.title {
					t.Fatalf("unexpected Last.fm query: %v", query)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(tt.body)),
				}, nil
			}), "test-key")
			target := domain.MusicTarget{Kind: tt.kind, Artist: "Florence + The Machine", Title: tt.title}
			count, err := client.PlayCount(t.Context(), "listener", target)
			if err != nil || count != 42 {
				t.Fatalf("count = %d, error = %v, want 42 and nil", count, err)
			}
		})
	}
}

func TestPlayCountRespectsExplicitYo(t *testing.T) {
	for _, test := range []struct {
		kind domain.MusicKind
		body string
	}{
		{domain.Artist, `{"artist":{"name":"Елка","stats":{"userplaycount":"42"}}}`},
		{domain.Track, `{"track":{"name":"Все","artist":{"name":"Ёлка"},"userplaycount":"42"}}`},
		{domain.Album, `{"album":{"name":"Всё","artist":"Елка","userplaycount":"42"}}`},
	} {
		client := NewClient(doFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(test.body))}, nil
		}), "test-key")
		target := domain.MusicTarget{Kind: test.kind, Artist: "Ёлка", Title: "Всё"}
		if count, err := client.PlayCount(t.Context(), "listener", target); err != nil || count != 0 {
			t.Fatalf("%s: count = %d, error = %v", test.kind, count, err)
		}
		target.Artist, target.Title = "Елка", "Все"
		if count, err := client.PlayCount(t.Context(), "listener", target); err != nil || count != 42 {
			t.Fatalf("%s: count = %d, error = %v", test.kind, count, err)
		}
	}
}
