package vibe

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/emount4/lastfm/internal/domain"
)

type listeningFixture struct {
	t        *testing.T
	interval domain.ScrobbleRange
}

func (f listeningFixture) List(context.Context) ([]domain.Participant, error) {
	return []domain.Participant{
		{TelegramUserID: 1, LastFMUsername: "other"},
		{TelegramUserID: 2, LastFMUsername: "sender"},
	}, nil
}

func (f listeningFixture) ScrobblesPage(_ context.Context, username string, interval domain.ScrobbleRange, page int) ([]domain.Scrobble, bool, error) {
	if username != "sender" || interval != f.interval || page > 2 {
		f.t.Fatalf("history request = %q, %+v, page %d", username, interval, page)
	}
	if page == 1 {
		return []domain.Scrobble{{Artist: "Artist A", Track: "Song"}, {Artist: "Artist B", Track: "Song"}}, true, nil
	}
	return []domain.Scrobble{{Artist: "artist a", Track: "song"}}, false, nil
}

func (f listeningFixture) Describe(_ context.Context, summary Summary) (string, error) {
	want := []TrackCount{{Artist: "Artist A", Track: "Song", Count: 2}, {Artist: "Artist B", Track: "Song", Count: 1}}
	if summary.Days != 3 || summary.Scrobbles != 3 || !slices.Equal(summary.TopTracks, want) {
		f.t.Fatalf("unexpected summary: %+v", summary)
	}
	return "Трек на репите", nil
}

func TestDescribeUsesAuthorsCalendarHistory(t *testing.T) {
	moscow := time.FixedZone("MSK", 3*60*60)
	now := time.Date(2026, 11, 12, 1, 15, 0, 0, moscow)
	fixture := listeningFixture{t: t, interval: domain.ScrobbleRange{
		From: time.Date(2026, 11, 10, 0, 0, 0, 0, moscow).Unix(),
		To:   now.Unix(),
	}}
	service := NewService(fixture, fixture, fixture, func() time.Time { return now })
	if got, err := service.Describe(t.Context(), 2, 3); got != "Трек на репите" || err != nil {
		t.Fatalf("description = %q, error = %v", got, err)
	}
}
