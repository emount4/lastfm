package migrations

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
)

//go:embed 001_create_users.sql
var createUsers string

//go:embed 002_create_music_choices.sql
var createMusicChoices string

type Executor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func Apply(ctx context.Context, db Executor) error {
	if _, err := db.ExecContext(ctx, createUsers); err != nil {
		return fmt.Errorf("apply migration 001_create_users: %w", err)
	}
	if _, err := db.ExecContext(ctx, createMusicChoices); err != nil {
		return fmt.Errorf("apply migration 002_create_music_choices: %w", err)
	}
	return nil
}
