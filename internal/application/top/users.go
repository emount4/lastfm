package top

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/emount4/lastfm/internal/domain"
)

type ScrobbleCounter interface {
	TotalScrobbles(ctx context.Context, username string) (int64, error)
	ScrobblesInRange(ctx context.Context, username string, interval domain.ScrobbleRange) (int64, error)
}

type UserEntry struct {
	Participant domain.Participant
	Scrobbles   int64
}

type UserRanking struct {
	Entries          []UserEntry
	ProfilesIncluded int
	ProfilesTotal    int
}

func (s *Service) Users(ctx context.Context, period domain.Period) (UserRanking, error) {
	var count func(context.Context, string) (int64, error)
	if period.AllTime() {
		count = s.counts.TotalScrobbles
	} else {
		interval, err := period.Range(s.now())
		if err != nil {
			return UserRanking{}, fmt.Errorf("calculate user ranking period: %w", err)
		}
		count = func(ctx context.Context, username string) (int64, error) {
			return s.counts.ScrobblesInRange(ctx, username, interval)
		}
	}

	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	participants, err := s.participants.List(ctx)
	if err != nil {
		return UserRanking{}, fmt.Errorf("list participants for user ranking: %w", err)
	}

	profiles := readProfiles(ctx, participants, count)
	ranking := UserRanking{ProfilesTotal: len(participants)}
	var profileErrors []error
	for index, profile := range profiles {
		if profile.err != nil {
			profileErrors = append(profileErrors, profile.err)
			continue
		}
		ranking.ProfilesIncluded++
		if profile.value > 0 {
			ranking.Entries = append(ranking.Entries, UserEntry{Participant: participants[index], Scrobbles: profile.value})
		}
	}
	if len(profileErrors) > 0 && ranking.ProfilesIncluded == 0 {
		return UserRanking{}, fmt.Errorf("load participant scrobble counts: %w", errors.Join(profileErrors...))
	}

	slices.SortFunc(ranking.Entries, func(a, b UserEntry) int {
		if byCount := cmp.Compare(b.Scrobbles, a.Scrobbles); byCount != 0 {
			return byCount
		}
		return cmp.Compare(a.Participant.TelegramUserID, b.Participant.TelegramUserID)
	})
	ranking.Entries = ranking.Entries[:min(Limit, len(ranking.Entries))]
	return ranking, nil
}
