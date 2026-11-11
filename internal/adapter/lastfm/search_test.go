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

func TestSearchMatchesEAndYoInOneDirection(t *testing.T) {
	const body = `{"results":{"trackmatches":{"track":[{"name":"Ёж","artist":"A"},{"name":"Еж","artist":"B"}]}}}`
	client := NewClient(doFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
	}), "test-key")
	for query, count := range map[string]int{"еж": 2, "ёЖ": 1} {
		matches, err := client.Search(t.Context(), domain.Track, query)
		if err != nil || len(matches) != count || matches[0].Title != "Ёж" {
			t.Fatalf("%q: matches = %+v, error = %v", query, matches, err)
		}
	}
}

func TestFindSpellingVariants(t *testing.T) {
	client := NewClient(doFunc(func(req *http.Request) (*http.Response, error) {
		query := req.URL.Query()
		var body string
		switch query.Get("method") {
		case "artist.search":
			body = `{"results":{"artistmatches":{"artist":[{"name":"Ёлка"},{"name":"Елка"},{"name":"Ёлка live"}]}}}`
		case "artist.getTopAlbums":
			body = `{"error":6,"message":"Artist not found"}`
		case "album.search":
			body = `{"results":{"albummatches":{"album":[{"name":"Всё","artist":"Ёлка"},{"name":"Все","artist":"Другой артист"}]}}}`
		default:
			t.Fatalf("unexpected query: %v", query)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
	}), "test-key")
	for _, test := range []struct {
		query domain.MusicTarget
		want  domain.MusicTarget
	}{
		{domain.MusicTarget{Kind: domain.Artist, Artist: "Елка"}, domain.MusicTarget{Kind: domain.Artist, Artist: "Ёлка"}},
		{domain.MusicTarget{Kind: domain.Album, Artist: "Елка", Title: "Все"}, domain.MusicTarget{Kind: domain.Album, Artist: "Ёлка", Title: "Всё"}},
	} {
		got, err := client.FindVariants(t.Context(), test.query)
		if err != nil || !slices.Equal(got, []domain.MusicTarget{test.want}) {
			t.Fatalf("query = %+v, variants = %+v, error = %v", test.query, got, err)
		}
	}
}

func TestFindVariants(t *testing.T) {
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

			got, err := client.FindVariants(t.Context(), tt.target)
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
