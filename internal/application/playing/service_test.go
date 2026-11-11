package playing

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/emount4/lastfm/internal/domain"
)

type listenersFixture []domain.Participant

func (f listenersFixture) List(context.Context) ([]domain.Participant, error) {
	return f, nil
}

type nowPlayingFunc func(context.Context, string) (domain.NowPlayingTrack, bool, error)

func (f nowPlayingFunc) NowPlaying(ctx context.Context, username string) (domain.NowPlayingTrack, bool, error) {
	return f(ctx, username)
}

func TestActiveListenersKeepsParticipantsWithTheSameTrack(t *testing.T) {
	participants := listenersFixture{
		{TelegramUserID: 1, TelegramUsername: "first", LastFMUsername: "first_fm"},
		{TelegramUserID: 2, LastFMUsername: "inactive"},
		{TelegramUserID: 3, LastFMUsername: "unavailable"},
		{TelegramUserID: 4, TelegramDisplayName: "Второй", LastFMUsername: "second_fm"},
	}
	track := domain.NowPlayingTrack{Artist: "Artist", Title: "Track"}
	service := NewService(participants, nowPlayingFunc(func(_ context.Context, username string) (domain.NowPlayingTrack, bool, error) {
		switch username {
		case "inactive":
			return domain.NowPlayingTrack{}, false, nil
		case "unavailable":
			return domain.NowPlayingTrack{}, false, errors.New("Last.fm unavailable")
		default:
			return track, true, nil
		}
	}))

	got, err := service.ActiveListeners(t.Context())
	want := []Listener{
		{Participant: participants[0], Track: track},
		{Participant: participants[3], Track: track},
	}
	if err != nil || !slices.Equal(got, want) {
		t.Fatalf("listeners = %+v, error = %v, want %+v", got, err, want)
	}
}
