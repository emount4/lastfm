package top

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/emount4/lastfm/internal/domain"
)

type countFixture struct {
	t        *testing.T
	interval domain.ScrobbleRange
}

func (f countFixture) TotalScrobbles(_ context.Context, username string) (int64, error) {
	if f.interval != (domain.ScrobbleRange{}) {
		f.t.Error("requested all-time count for a specific period")
	}
	return f.count(username)
}

func (f countFixture) ScrobblesInRange(_ context.Context, username string, interval domain.ScrobbleRange) (int64, error) {
	if interval != f.interval || interval == (domain.ScrobbleRange{}) {
		f.t.Errorf("interval = %+v, want %+v", interval, f.interval)
	}
	return f.count(username)
}

func (f countFixture) count(username string) (int64, error) {
	switch username {
	case "first":
		return 20, nil
	case "second":
		return 80, nil
	case "quiet":
		return 0, nil
	default:
		return 0, errors.New("profile unavailable")
	}
}

func TestUsersRanksAvailableProfiles(t *testing.T) {
	first := domain.Participant{TelegramUserID: 1, LastFMUsername: "first"}
	second := domain.Participant{TelegramUserID: 2, LastFMUsername: "second"}
	participants := chartFixture{participants: []domain.Participant{
		first, second, {LastFMUsername: "quiet"}, {LastFMUsername: "unavailable"},
	}}
	now := time.Date(2026, time.November, 11, 14, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		arg      string
		interval domain.ScrobbleRange
	}{
		{arg: ""},
		{arg: "7d", interval: domain.ScrobbleRange{From: now.AddDate(0, 0, -7).Unix(), To: now.Unix()}},
	} {
		t.Run(test.arg, func(t *testing.T) {
			period, err := domain.ParsePeriod(test.arg)
			if err != nil {
				t.Fatal(err)
			}
			service := NewService(participants, nil, nil, countFixture{t: t, interval: test.interval}, func() time.Time { return now })
			ranking, err := service.Users(t.Context(), period)
			want := []UserEntry{{Participant: second, Scrobbles: 80}, {Participant: first, Scrobbles: 20}}
			if err != nil || !slices.Equal(ranking.Entries, want) || ranking.ProfilesIncluded != 3 || ranking.ProfilesTotal != 4 {
				t.Fatalf("ranking = %+v, error = %v", ranking, err)
			}
		})
	}
}
