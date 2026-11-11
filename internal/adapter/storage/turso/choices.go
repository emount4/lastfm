package turso

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/emount4/lastfm/internal/domain"
)

type ChoiceDB interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type ChoiceStore struct {
	db ChoiceDB
}

func NewChoiceStore(db ChoiceDB) *ChoiceStore {
	return &ChoiceStore{db: db}
}

func (s *ChoiceStore) SaveChoices(ctx context.Context, id string, chatID, threadID int64, options []domain.MusicTarget) error {
	encoded, err := json.Marshal(options)
	if err != nil {
		return fmt.Errorf("save music choices: encode options: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, "DELETE FROM music_choices WHERE expires_at <= unixepoch()"); err != nil {
		return fmt.Errorf("save music choices: clear expired choices: %w", err)
	}
	const insert = `
INSERT INTO music_choices (id, chat_id, thread_id, options, expires_at)
VALUES (?1, ?2, ?3, ?4, unixepoch() + 900)
`
	if _, err := s.db.ExecContext(ctx, insert, id, chatID, threadID, string(encoded)); err != nil {
		return fmt.Errorf("save music choices: %w", err)
	}
	return nil
}

func (s *ChoiceStore) TakeChoice(ctx context.Context, id string, chatID int64, index int) (domain.MusicTarget, int64, bool, error) {
	const take = `
DELETE FROM music_choices
WHERE id = ?1 AND chat_id = ?2 AND expires_at > unixepoch()
  AND ?3 >= 0 AND ?3 < json_array_length(options)
RETURNING json_extract(options, '$[' || ?3 || ']'), thread_id
`
	var encoded string
	var threadID int64
	err := s.db.QueryRowContext(ctx, take, id, chatID, index).Scan(&encoded, &threadID)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.MusicTarget{}, 0, false, nil
	}
	if err != nil {
		return domain.MusicTarget{}, 0, false, fmt.Errorf("take music choice: %w", err)
	}
	var target domain.MusicTarget
	if err := json.Unmarshal([]byte(encoded), &target); err != nil {
		return domain.MusicTarget{}, 0, false, fmt.Errorf("take music choice: decode option: %w", err)
	}
	return target, threadID, true, nil
}
