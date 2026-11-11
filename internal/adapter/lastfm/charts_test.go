package lastfm

import (
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/emount4/lastfm/internal/domain"
)

func TestChartPage(t *testing.T) {
	for _, test := range []struct {
		kind   domain.MusicKind
		method string
		body   string
		want   domain.ChartEntry
	}{
		{domain.Artist, "user.getTopArtists", `{"topartists":{"artist":[{"name":"Artist","playcount":"42"}],"@attr":{"page":"2","totalPages":"3"}}}`,
			domain.ChartEntry{Artist: "Artist", Scrobbles: 42}},
		{domain.Track, "user.getTopTracks", `{"toptracks":{"track":[{"name":"Track","playcount":"42","artist":{"name":"Artist"}}],"@attr":{"page":"2","totalPages":"3"}}}`,
			domain.ChartEntry{Artist: "Artist", Title: "Track", Scrobbles: 42}},
		{domain.Album, "user.getTopAlbums", `{"topalbums":{"album":[{"name":"Album","playcount":"42","artist":{"name":"Artist"}}],"@attr":{"page":"2","totalPages":"3"}}}`,
			domain.ChartEntry{Artist: "Artist", Title: "Album", Scrobbles: 42}},
	} {
		t.Run(string(test.kind), func(t *testing.T) {
			client := NewClient(doFunc(func(req *http.Request) (*http.Response, error) {
				query := req.URL.Query()
				if query.Get("method") != test.method || query.Get("user") != "listener" ||
					query.Get("period") != "overall" || query.Get("page") != "2" || query.Get("limit") != "500" {
					t.Fatalf("unexpected query: %v", query)
				}
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(test.body))}, nil
			}), "test-key")
			entries, more, err := client.ChartPage(t.Context(), "listener", test.kind, 2)
			if err != nil || !more || !slices.Equal(entries, []domain.ChartEntry{test.want}) {
				t.Fatalf("entries = %+v, more = %t, error = %v", entries, more, err)
			}
		})
	}
}
