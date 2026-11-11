package turso

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/tursodatabase/libsql-client-go/libsql"

	"github.com/emount4/lastfm/internal/domain"
	"github.com/emount4/lastfm/migrations"
)

func TestParticipantStoreTurso(t *testing.T) {
	databaseURL := os.Getenv("TURSO_TEST_DATABASE_URL")
	authToken := os.Getenv("TURSO_TEST_AUTH_TOKEN")
	if databaseURL == "" && authToken == "" {
		t.Skip("Задайте TURSO_TEST_DATABASE_URL и TURSO_TEST_AUTH_TOKEN для тестовой базы")
	}
	if databaseURL == "" || authToken == "" {
		t.Fatal("Для проверки нужны и адрес тестовой базы, и токен")
	}

	connector, err := libsql.NewConnector(databaseURL, libsql.WithAuthToken(authToken))
	if err != nil {
		t.Fatalf("Создание подключения к Turso: %v", err)
	}
	db := sql.OpenDB(connector)
	defer db.Close()

	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("Начало транзакции: %v", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			t.Errorf("Откат тестовой транзакции: %v", err)
		}
	}()

	if err := migrations.Apply(ctx, tx); err != nil {
		t.Fatalf("Применение миграции: %v", err)
	}

	store := NewParticipantStore(tx)
	participant := domain.Participant{
		TelegramUserID:      900000000001,
		TelegramUsername:    "lastfm_test_user",
		TelegramDisplayName: "Тестовый участник",
		LastFMUsername:      "LastFMIntegrationTest",
	}
	if err := store.Save(ctx, participant); err != nil {
		t.Fatalf("Сохранение привязки: %v", err)
	}

	participants, err := store.List(ctx)
	if err != nil {
		t.Fatalf("Чтение привязок: %v", err)
	}
	if !slices.Contains(participants, participant) {
		t.Fatal("Сохранённая привязка отсутствует в списке")
	}
}
