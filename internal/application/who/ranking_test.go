package who

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/emount4/lastfm/internal/domain"
)

type rankingFixture struct {
	participants []domain.Participant
}

func (f rankingFixture) List(context.Context) ([]domain.Participant, error) {
	return f.participants, nil
}

func (f rankingFixture) PlayCount(_ context.Context, username string, _ domain.MusicTarget) (int64, error) {
	switch username {
	case "first":
		return 2, nil
	case "second":
		return 5, nil
	case "unavailable":
		return 0, errors.New("profile unavailable")
	default:
		return 0, nil
	}
}

func TestRankArtistSkipsMissingCountsAndSorts(t *testing.T) {
	fixture := rankingFixture{participants: []domain.Participant{
		{LastFMUsername: "first"},
		{LastFMUsername: "unavailable"},
		{LastFMUsername: "zero"},
		{LastFMUsername: "second"},
	}}
	got, err := NewService(fixture, fixture).Rank(t.Context(), domain.MusicTarget{Kind: domain.Artist, Artist: "Artist"})
	if err != nil {
		t.Fatal(err)
	}
	want := Ranking{
		Entries: []Entry{
			{Participant: fixture.participants[3], Scrobbles: 5},
			{Participant: fixture.participants[0], Scrobbles: 2},
		},
		Total: 7,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ranking = %+v, want %+v", got, want)
	}
}
