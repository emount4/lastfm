package lastfm

import (
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/emount4/lastfm/internal/domain"
)

func TestSearchPrefersDistinctExactMatches(t *testing.T) {
	const response = `{"results":{"albummatches":{"album":[
		{"name":"Example live","artist":"A"},
		{"name":"Example","artist":"A"},
		{"name":"EXAMPLE","artist":"a"},
		{"name":"Example","artist":"B"}
	]}}}`
	client := NewClient(doFunc(func(req *http.Request) (*http.Response, error) {
		query := req.URL.Query()
		if query.Get("method") != "album.search" || query.Get("album") != "Example" {
			t.Fatalf("unexpected search query: %v", query)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(response))}, nil
	}), "test-key")

	got, err := client.Search(t.Context(), domain.Album, "Example")
	want := []domain.MusicTarget{
		{Kind: domain.Album, Artist: "A", Title: "Example"},
		{Kind: domain.Album, Artist: "B", Title: "Example"},
	}
	if err != nil || !slices.Equal(got, want) {
		t.Fatalf("search = %+v, error = %v, want %+v", got, err, want)
	}
}
