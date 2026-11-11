package lastfm

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestArtistPlayCount(t *testing.T) {
	client := NewClient(doFunc(func(req *http.Request) (*http.Response, error) {
		query := req.URL.Query()
		if query.Get("method") != "artist.getInfo" || query.Get("username") != "listener" ||
			query.Get("artist") != "Florence + The Machine" || query.Get("autocorrect") != "1" {
			t.Fatalf("unexpected Last.fm query: %v", query)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(
				`{"artist":{"stats":{"playcount":"20000000","userplaycount":"42"}}}`,
			)),
		}, nil
	}), "test-key")

	count, err := client.ArtistPlayCount(t.Context(), "listener", "Florence + The Machine")
	if err != nil || count != 42 {
		t.Fatalf("count = %d, error = %v, want 42 and nil", count, err)
	}
}
