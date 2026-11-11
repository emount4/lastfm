package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tursodatabase/libsql-client-go/libsql"

	"github.com/emount4/lastfm/internal/adapter/artwork"
	"github.com/emount4/lastfm/internal/adapter/collage"
	"github.com/emount4/lastfm/internal/adapter/httpserver"
	"github.com/emount4/lastfm/internal/adapter/lastfm"
	"github.com/emount4/lastfm/internal/adapter/storage/turso"
	telegram "github.com/emount4/lastfm/internal/adapter/telegram/client"
	"github.com/emount4/lastfm/internal/adapter/telegram/handler"
	"github.com/emount4/lastfm/internal/adapter/telegram/webhook"
	"github.com/emount4/lastfm/internal/application/playing"
	"github.com/emount4/lastfm/internal/application/registration"
	"github.com/emount4/lastfm/internal/application/who"
	"github.com/emount4/lastfm/internal/config"
	"github.com/emount4/lastfm/migrations"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	renderer, err := collage.NewRenderer()
	if err != nil {
		logger.Error("не удалось подготовить шрифты коллажа", "error", err)
		os.Exit(1)
	}
	connector, err := libsql.NewConnector(cfg.Turso.DatabaseURL, libsql.WithAuthToken(cfg.Turso.AuthToken))
	if err != nil {
		logger.Error("invalid Turso configuration", "error", fmt.Errorf("create Turso connector: %w", err))
		os.Exit(1)
	}
	db := sql.OpenDB(connector)

	httpClient := &http.Client{Timeout: 5 * time.Second}
	profiles := lastfm.NewClient(httpClient, cfg.LastFMAPIKey)
	accounts := turso.NewParticipantStore(db)
	registrationService := registration.NewService(profiles, accounts)
	messenger := telegram.New(httpClient, cfg.Telegram.BotToken)
	privateMessages := handler.NewRegistration(registrationService, messenger, logger)
	rankings := who.NewService(accounts, profiles)
	choices := turso.NewChoiceStore(db)
	mediaHTTP := &http.Client{
		Timeout: 3 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	images := artwork.NewLoader(mediaHTTP)
	listeners := playing.NewService(accounts, profiles)
	playingCollage := playing.NewCollage(listeners, profiles, messenger, images, renderer)
	groupMessages := handler.NewGroup(messenger, rankings, profiles, choices, playingCollage)
	messages := handler.NewRouter(privateMessages, groupMessages)
	telegramWebhook := webhook.New(cfg.Telegram.WebhookSecret, cfg.Telegram.GroupID, messages, groupMessages, logger)
	server := httpserver.New(cfg.Port, telegramWebhook)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	migrationCtx, cancelMigration := context.WithTimeout(ctx, 5*time.Second)
	err = migrations.Apply(migrationCtx, db)
	cancelMigration()
	if err == nil {
		err = serve(ctx, server, logger)
	}
	stop()
	if closeErr := db.Close(); closeErr != nil {
		err = errors.Join(err, fmt.Errorf("close Turso connection pool: %w", closeErr))
	}
	if err != nil {
		logger.Error("bot stopped with an error", "error", err)
		os.Exit(1)
	}
}

func serve(ctx context.Context, server *http.Server, logger *slog.Logger) error {
	serverErr := make(chan error, 1)
	go func() {
		logger.Info("starting HTTP server", "address", server.Addr)
		serverErr <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shut down HTTP server: %w", err)
		}
		err := <-serverErr
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
