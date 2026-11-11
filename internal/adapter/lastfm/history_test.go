package lastfm

import (
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/emount4/lastfm/internal/domain"
)

func TestScrobblesPageUsesHalfOpenRange(t *testing.T) {
	const body = `{"recenttracks":{"track":[
		{"artist":{"#text":"Artist"},"name":"Now","date":{"uts":"150"},"@attr":{"nowplaying":"true"}},
		{"artist":{"#text":"Artist"},"name":"No date"},
		{"artist":{"#text":"Artist"},"name":"Too late","date":{"uts":"200"}},
		{"artist":{"#text":" Artist "},"name":" Track ","album":{"#text":" Album "},"date":{"uts":"100"}},
		{"artist":{"#text":"Artist"},"name":"Too early","date":{"uts":"99"}}
	],"@attr":{"page":"2","totalPages":"3"}}}`
	client := NewClient(doFunc(func(req *http.Request) (*http.Response, error) {
		query := req.URL.Query()
		if query.Get("method") != "user.getRecentTracks" || query.Get("user") != "listener" ||
			query.Get("page") != "2" || query.Get("limit") != "200" ||
			query.Get("from") != "99" || query.Get("to") != "200" {
			t.Fatalf("unexpected query: %v", query)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
	}), "test-key")
	entries, more, err := client.ScrobblesPage(t.Context(), "listener", domain.ScrobbleRange{From: 100, To: 200}, 2)
	want := []domain.Scrobble{{Artist: "Artist", Track: "Track", Album: "Album"}}
	if err != nil || !more || !slices.Equal(entries, want) {
		t.Fatalf("entries = %+v, more = %t, error = %v", entries, more, err)
	}
}

func TestScrobbleCounts(t *testing.T) {
	client := NewClient(doFunc(func(req *http.Request) (*http.Response, error) {
		query := req.URL.Query()
		if query.Get("user") != "listener" {
			t.Fatalf("unexpected query: %v", query)
		}
		var body string
		switch query.Get("method") {
		case "user.getInfo":
			body = `{"user":{"name":"listener","playcount":"40078"}}`
		case "user.getRecentTracks":
			if query.Get("limit") != "1" || query.Get("from") != "100" || query.Get("to") != "200" {
				t.Fatalf("unexpected history query: %v", query)
			}
			body = `{"recenttracks":{"track":[{"@attr":{"nowplaying":"true"}}],"@attr":{"total":"123"}}}`
		default:
			t.Fatalf("unexpected method: %v", query)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
	}), "test-key")
	if count, err := client.TotalScrobbles(t.Context(), "listener"); err != nil || count != 40078 {
		t.Fatalf("total count = %d, error = %v", count, err)
	}
	if count, err := client.ScrobblesInRange(t.Context(), "listener", domain.ScrobbleRange{From: 100, To: 200}); err != nil || count != 123 {
		t.Fatalf("period count = %d, error = %v", count, err)
	}
}
