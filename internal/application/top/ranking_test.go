package top

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/emount4/lastfm/internal/domain"
)

type chartFixture struct {
	participants []domain.Participant
}

func (f chartFixture) List(context.Context) ([]domain.Participant, error) {
	return f.participants, nil
}

func (f chartFixture) ChartPage(_ context.Context, username string, _ domain.MusicKind, page int) ([]domain.ChartEntry, bool, error) {
	if username == "partial" {
		if page == 1 {
			return []domain.ChartEntry{{Artist: "Partial only", Scrobbles: 9999}}, true, nil
		}
		return nil, false, errors.New("Last.fm unavailable")
	}
	if page == 1 {
		entries := make([]domain.ChartEntry, 26)
		for i := range entries {
			entries[i] = domain.ChartEntry{Artist: fmt.Sprintf("%s-%02d", username, i), Scrobbles: 80}
		}
		return entries, true, nil
	}
	if username == "first" {
		return []domain.ChartEntry{{Artist: "Shared artist", Scrobbles: 60}}, false, nil
	}
	return []domain.ChartEntry{{Artist: "SHARED ARTIST", Scrobbles: 61}}, false, nil
}

func TestArtistsRanksAcrossAllCompleteCharts(t *testing.T) {
	fixture := chartFixture{participants: []domain.Participant{
		{LastFMUsername: "first"},
		{LastFMUsername: "second"},
		{LastFMUsername: "partial"},
	}}
	got, err := NewService(fixture, fixture, nil, nil, nil).Rank(t.Context(), domain.Artist, domain.Period{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Entries) != 25 || got.ProfilesIncluded != 2 || got.ProfilesTotal != 3 {
		t.Fatalf("unexpected ranking size or coverage: %+v", got)
	}
	if want := (domain.ChartEntry{Artist: "Shared artist", Scrobbles: 121}); got.Entries[0] != want {
		t.Fatalf("first artist = %+v, want %+v", got.Entries[0], want)
	}
	if slices.ContainsFunc(got.Entries, func(entry domain.ChartEntry) bool {
		return entry.Artist == "Partial only"
	}) {
		t.Fatal("incomplete chart affected the group ranking")
	}
}

type scrobbleReaderFunc func(context.Context, string, domain.ScrobbleRange, int) ([]domain.Scrobble, bool, error)

func (f scrobbleReaderFunc) ScrobblesPage(ctx context.Context, username string, interval domain.ScrobbleRange, page int) ([]domain.Scrobble, bool, error) {
	return f(ctx, username, interval, page)
}

func TestArtistsPeriodUsesOneWindowForAllProfiles(t *testing.T) {
	participants := chartFixture{participants: []domain.Participant{{LastFMUsername: "first"}, {LastFMUsername: "second"}}}
	now := time.Date(2026, time.November, 11, 14, 0, 0, 0, time.FixedZone("MSK", 3*60*60))
	period, err := domain.ParsePeriod("2W")
	if err != nil {
		t.Fatal(err)
	}
	clockCalls := 0
	wantInterval := domain.ScrobbleRange{From: now.Add(-14 * 24 * time.Hour).Unix(), To: now.Unix()}
	reader := scrobbleReaderFunc(func(_ context.Context, username string, interval domain.ScrobbleRange, page int) ([]domain.Scrobble, bool, error) {
		if interval != wantInterval {
			t.Errorf("interval = %+v, want %+v", interval, wantInterval)
		}
		if page == 1 {
			return []domain.Scrobble{{Artist: "Shared Artist"}}, username == "first", nil
		}
		return []domain.Scrobble{{Artist: "shared artist"}, {Artist: "Other Artist"}}, false, nil
	})

	service := NewService(participants, nil, reader, nil, func() time.Time {
		clockCalls++
		return now
	})
	got, err := service.Rank(t.Context(), domain.Artist, period)
	want := []domain.ChartEntry{{Artist: "Shared Artist", Scrobbles: 3}, {Artist: "Other Artist", Scrobbles: 1}}
	if err != nil || !slices.Equal(got.Entries, want) || clockCalls != 1 || got.ProfilesIncluded != 2 {
		t.Fatalf("ranking = %+v, error = %v, clock calls = %d", got, err, clockCalls)
	}
}

func TestRankTracksAndAlbumsByArtistAndTitle(t *testing.T) {
	fixture := chartFixture{participants: []domain.Participant{{LastFMUsername: "first"}, {LastFMUsername: "second"}}}
	period, err := domain.ParsePeriod("1w")
	if err != nil {
		t.Fatal(err)
	}
	reader := scrobbleReaderFunc(func(_ context.Context, username string, _ domain.ScrobbleRange, _ int) ([]domain.Scrobble, bool, error) {
		if username == "second" {
			return []domain.Scrobble{
				{Artist: "artist a", Track: "same", Album: "release"},
				{Artist: "Artist A", Track: "Another", Album: "Release"},
			}, false, nil
		}
		return []domain.Scrobble{
			{Artist: "Artist A", Track: "Same", Album: "Release"},
			{Artist: "Artist B", Track: "Same", Album: "Release"},
			{Artist: "Artist A", Track: "No album"},
		}, false, nil
	})
	service := NewService(fixture, nil, reader, nil, time.Now)
	for _, test := range []struct {
		kind domain.MusicKind
		want []domain.ChartEntry
	}{
		{domain.Track, []domain.ChartEntry{
			{Artist: "Artist A", Title: "Same", Scrobbles: 2},
			{Artist: "Artist A", Title: "Another", Scrobbles: 1},
			{Artist: "Artist A", Title: "No album", Scrobbles: 1},
			{Artist: "Artist B", Title: "Same", Scrobbles: 1},
		}},
		{domain.Album, []domain.ChartEntry{
			{Artist: "Artist A", Title: "Release", Scrobbles: 3},
			{Artist: "Artist B", Title: "Release", Scrobbles: 1},
		}},
	} {
		t.Run(string(test.kind), func(t *testing.T) {
			got, err := service.Rank(t.Context(), test.kind, period)
			if err != nil || !slices.Equal(got.Entries, test.want) {
				t.Fatalf("ranking = %+v, error = %v", got, err)
			}
		})
	}
}
