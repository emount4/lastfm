package registration

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/emount4/lastfm/internal/domain"
)

var (
	ErrUsernameRequired = errors.New("lastfm username is required")
	ErrProfileNotFound  = errors.New("lastfm profile was not found")
)

type ProfileVerifier interface {
	LookupProfile(ctx context.Context, username string) (name string, found bool, err error)
	CheckScrobbleAccess(ctx context.Context, username string) error
}

type AccountStore interface {
	Save(ctx context.Context, participant domain.Participant) error
}

type Service struct {
	profiles ProfileVerifier
	accounts AccountStore
}

func NewService(profiles ProfileVerifier, accounts AccountStore) *Service {
	return &Service{profiles: profiles, accounts: accounts}
}

// Register проверяет профиль и возвращает сохранённый канонический ник Last.fm.
func (s *Service) Register(ctx context.Context, participant domain.Participant) (string, error) {
	if participant.TelegramUserID <= 0 {
		return "", errors.New("register participant: invalid Telegram user ID")
	}

	username := strings.TrimSpace(participant.LastFMUsername)
	if username == "" {
		return "", fmt.Errorf("register participant: %w", ErrUsernameRequired)
	}

	canonicalName, found, err := s.profiles.LookupProfile(ctx, username)
	if err != nil {
		return "", fmt.Errorf("register participant: look up Last.fm profile: %w", err)
	}
	if !found {
		return "", fmt.Errorf("register participant: %w", ErrProfileNotFound)
	}

	if err := s.profiles.CheckScrobbleAccess(ctx, canonicalName); err != nil {
		return "", fmt.Errorf("register participant: verify Last.fm scrobble access: %w", err)
	}

	participant.LastFMUsername = canonicalName
	if err := s.accounts.Save(ctx, participant); err != nil {
		return "", fmt.Errorf("register participant: save account link: %w", err)
	}
	return canonicalName, nil
}
