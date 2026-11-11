package turso

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/emount4/lastfm/internal/domain"
)

type DB interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

type ParticipantStore struct {
	db DB
}

func NewParticipantStore(db DB) *ParticipantStore {
	return &ParticipantStore{db: db}
}

// Проверка владельца и сохранение привязки выполняются одним SQL-запросом.
const saveParticipant = `
INSERT INTO users (
	telegram_user_id, telegram_username, telegram_display_name, lastfm_username, lastfm_username_key
)
SELECT ?1, ?2, ?3, ?4, ?5
WHERE NOT EXISTS (
	SELECT 1 FROM users WHERE lastfm_username_key = ?5 AND telegram_user_id <> ?1
)
ON CONFLICT(telegram_user_id) DO UPDATE SET
	telegram_username = excluded.telegram_username,
	telegram_display_name = excluded.telegram_display_name,
	lastfm_username = excluded.lastfm_username,
	lastfm_username_key = excluded.lastfm_username_key,
	updated_at = unixepoch()
`

const deleteParticipant = `DELETE FROM users WHERE telegram_user_id = ?1`

const listParticipants = `
SELECT telegram_user_id, telegram_username, telegram_display_name, lastfm_username
FROM users
ORDER BY telegram_user_id
`

func (s *ParticipantStore) Save(ctx context.Context, participant domain.Participant) error {
	username := strings.TrimSpace(participant.LastFMUsername)
	result, err := s.db.ExecContext(ctx, saveParticipant,
		participant.TelegramUserID,
		participant.TelegramUsername,
		participant.TelegramDisplayName,
		username,
		strings.ToLower(username),
	)
	if err != nil {
		return fmt.Errorf("save participant: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("save participant: read affected rows: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("save participant: %w", domain.ErrLastFMAccountTaken)
	}
	return nil
}

func (s *ParticipantStore) Delete(ctx context.Context, telegramUserID int64) error {
	if _, err := s.db.ExecContext(ctx, deleteParticipant, telegramUserID); err != nil {
		return fmt.Errorf("delete participant: %w", err)
	}
	return nil
}

func (s *ParticipantStore) List(ctx context.Context) ([]domain.Participant, error) {
	rows, err := s.db.QueryContext(ctx, listParticipants)
	if err != nil {
		return nil, fmt.Errorf("list participants: %w", err)
	}
	defer rows.Close()

	var participants []domain.Participant
	for rows.Next() {
		var participant domain.Participant
		if err := rows.Scan(
			&participant.TelegramUserID,
			&participant.TelegramUsername,
			&participant.TelegramDisplayName,
			&participant.LastFMUsername,
		); err != nil {
			return nil, fmt.Errorf("list participants: scan row: %w", err)
		}
		participants = append(participants, participant)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list participants: iterate rows: %w", err)
	}
	return participants, nil
}
