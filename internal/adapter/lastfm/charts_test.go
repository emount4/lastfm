package lastfm

import (
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/emount4/lastfm/internal/domain"
)

func TestTopArtistsPage(t *testing.T) {
	const body = `{"topartists":{"artist":[{"name":"Artist","playcount":"42"}],"@attr":{"page":"2","totalPages":"3"}}}`
	client := NewClient(doFunc(func(req *http.Request) (*http.Response, error) {
		query := req.URL.Query()
		if query.Get("method") != "user.getTopArtists" || query.Get("user") != "listener" ||
			query.Get("period") != "overall" || query.Get("page") != "2" || query.Get("limit") != "500" {
			t.Fatalf("unexpected query: %v", query)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
	}), "test-key")
	entries, more, err := client.TopArtistsPage(t.Context(), "listener", 2)
	want := []domain.ArtistScrobbles{{Name: "Artist", Scrobbles: 42}}
	if err != nil || !more || !slices.Equal(entries, want) {
		t.Fatalf("entries = %+v, more = %t, error = %v", entries, more, err)
	}
}
