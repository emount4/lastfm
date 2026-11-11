package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/emount4/lastfm/internal/adapter/httpserver"
	"github.com/emount4/lastfm/internal/adapter/telegram/client"
	"github.com/emount4/lastfm/internal/adapter/telegram/handler"
	"github.com/emount4/lastfm/internal/adapter/telegram/webhook"
	"github.com/emount4/lastfm/internal/application/registration"
	"github.com/emount4/lastfm/internal/domain"
)

type registrarStub struct {
	register func(context.Context, domain.Participant) (string, error)
	unlink   func(context.Context, int64) error
}

func (s registrarStub) Register(ctx context.Context, participant domain.Participant) (string, error) {
	return s.register(ctx, participant)
}

func (s registrarStub) Unlink(ctx context.Context, userID int64) error {
	return s.unlink(ctx, userID)
}

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestPrivateRegistrationThroughWebhook(t *testing.T) {
	tests := []struct {
		name, message, reply string
		registerErr          error
		registers            bool
		unlinks              bool
		unlinkErr            error
		sendFails            bool
	}{
		{name: "start", message: "/START invite", reply: "Пришли свой ник Last.fm"},
		{name: "registered", message: "example", reply: "Привязал профиль Last.fm: Example.", registers: true},
		{
			name: "profile not found", message: "example", reply: "Профиль Last.fm не найден",
			registers: true, registerErr: fmt.Errorf("registration: %w", registration.ErrProfileNotFound),
		},
		{
			name: "conflict", message: "example", reply: "уже привязан к другому участнику",
			registers: true, registerErr: fmt.Errorf("registration: %w", domain.ErrLastFMAccountTaken),
		},
		{name: "send failed", message: "example", registers: true, sendFails: true},
		{name: "unlink", message: " UnLiNk ", reply: "У тебя больше нет привязки Last.fm.", unlinks: true},
		{name: "unlink failed", message: "unlink", reply: "Не удалось отвязать профиль.", unlinks: true, unlinkErr: errors.New("database unavailable")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registered, unlinked, sent := false, false, false
			registrar := registrarStub{register: func(_ context.Context, participant domain.Participant) (string, error) {
				registered = true
				want := domain.Participant{
					TelegramUserID: 123, TelegramUsername: "tg_example",
					TelegramDisplayName: "Анна Иванова", LastFMUsername: "example",
				}
				if participant != want {
					t.Fatalf("participant = %+v, want %+v", participant, want)
				}
				return "Example", tt.registerErr
			}, unlink: func(_ context.Context, userID int64) error {
				unlinked = true
				if userID != 123 {
					t.Fatalf("unlink user ID = %d, want 123", userID)
				}
				return tt.unlinkErr
			}}
			httpClient := &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
				sent = true
				if req.Method != http.MethodPost || req.URL.Host != "api.telegram.org" ||
					req.URL.Path != "/bot123:test-token/sendMessage" || req.Header.Get("Content-Type") != "application/json" {
					t.Fatalf("unexpected Telegram request: %s %s", req.Method, req.URL)
				}
				var payload struct {
					ChatID int64  `json:"chat_id"`
					Text   string `json:"text"`
				}
				if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
					t.Fatalf("decode Telegram request: %v", err)
				}
				if payload.ChatID != 123 || !strings.Contains(payload.Text, tt.reply) {
					t.Fatalf("unexpected Telegram payload: %+v", payload)
				}
				if tt.sendFails {
					return nil, errors.New("temporary network failure")
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"ok":true}`))}, nil
			})}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			messages := handler.NewRegistration(registrar, client.New(httpClient, "123:test-token"), logger)
			server := httpserver.New(8080, webhook.New("test-secret", -100123, messages, nil, logger))
			body := fmt.Sprintf(`{"update_id":42,"message":{"message_id":1,"text":%q,"chat":{"id":123,"type":"private"},"from":{"id":123,"username":"tg_example","first_name":"Анна","last_name":"Иванова"}}}`, tt.message)
			req := httptest.NewRequest(http.MethodPost, "/telegram/webhook", strings.NewReader(body))
			req.Header.Set("X-Telegram-Bot-Api-Secret-Token", "test-secret")
			response := httptest.NewRecorder()
			server.Handler.ServeHTTP(response, req)

			wantStatus := http.StatusNoContent
			if tt.sendFails {
				wantStatus = http.StatusInternalServerError
			}
			if response.Code != wantStatus || registered != tt.registers || unlinked != tt.unlinks || !sent {
				t.Fatalf("status = %d, registered = %t, unlinked = %t, sent = %t", response.Code, registered, unlinked, sent)
			}
		})
	}
}
