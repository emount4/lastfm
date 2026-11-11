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

func TestFindTitleVariants(t *testing.T) {
	tests := []struct {
		name   string
		target domain.MusicTarget
		method string
		body   string
		titles []string
	}{
		{
			name:   "track with omitted exclamation mark",
			target: domain.MusicTarget{Kind: domain.Track, Artist: "kai angel", Title: "stfu"},
			method: "track.search",
			body: `{"results":{"trackmatches":{"track":[
				{"name":"STFU!","artist":"Kai Angel"},
				{"name":"STFU!","artist":"Another Artist"},
				{"name":"STFU Remix","artist":"Kai Angel"}
			]}}}`,
			titles: []string{"STFU!"},
		},
		{
			name:   "distinct album editions with trailing punctuation",
			target: domain.MusicTarget{Kind: domain.Album, Artist: "kai angel", Title: "shh.."},
			method: "artist.getTopAlbums",
			body: `{"topalbums":{"album":[
				{"name":"Shh..","artist":{"name":"Kai Angel"}},
				{"name":"Shh...","artist":{"name":"Kai Angel"}},
				{"name":"Shh!","artist":{"name":"Kai Angel"}}
			]}}`,
			titles: []string{"Shh...", "Shh!"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(doFunc(func(req *http.Request) (*http.Response, error) {
				query := req.URL.Query()
				if query.Get("method") != tt.method || query.Get("artist") != tt.target.Artist {
					t.Fatalf("unexpected request: %v", query)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(tt.body)),
				}, nil
			}), "test-key")

			got, err := client.FindTitleVariants(t.Context(), tt.target)
			if err != nil || len(got) != len(tt.titles) {
				t.Fatalf("variants = %+v, error = %v", got, err)
			}
			for i, title := range tt.titles {
				want := domain.MusicTarget{Kind: tt.target.Kind, Artist: "Kai Angel", Title: title}
				if got[i] != want {
					t.Fatalf("variant %d = %+v, want %+v", i, got[i], want)
				}
			}
		})
	}
}
