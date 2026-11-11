package playing

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/emount4/lastfm/internal/domain"
)

type ParticipantReader interface {
	List(context.Context) ([]domain.Participant, error)
}

type TrackReader interface {
	NowPlaying(ctx context.Context, username string) (domain.NowPlayingTrack, bool, error)
}

type Listener struct {
	Participant domain.Participant
	Track       domain.NowPlayingTrack
}

type Service struct {
	participants ParticipantReader
	tracks       TrackReader
}

func NewService(participants ParticipantReader, tracks TrackReader) *Service {
	return &Service{participants: participants, tracks: tracks}
}

// ActiveListeners возвращает участников, для которых Last.fm сообщает о текущем прослушивании.
func (s *Service) ActiveListeners(ctx context.Context) ([]Listener, error) {
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()

	participants, err := s.participants.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("find active listeners: list participants: %w", err)
	}

	results := make([]*Listener, len(participants))
	slots := make(chan struct{}, 4)
	var requests sync.WaitGroup
	for index, participant := range participants {
		requests.Go(func() {
			select {
			case slots <- struct{}{}:
				defer func() { <-slots }()
			case <-ctx.Done():
				return
			}
			track, active, err := s.tracks.NowPlaying(ctx, participant.LastFMUsername)
			if err == nil && active {
				results[index] = &Listener{Participant: participant, Track: track}
			}
		})
	}
	requests.Wait()

	listeners := make([]Listener, 0, len(results))
	for _, result := range results {
		if result != nil {
			listeners = append(listeners, *result)
		}
	}
	return listeners, nil
}
