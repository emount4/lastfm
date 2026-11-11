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
	"github.com/emount4/lastfm/internal/application/who"
	"github.com/emount4/lastfm/internal/domain"
)

type rankerFunc func(context.Context, domain.MusicTarget) (who.Ranking, error)

func (f rankerFunc) Rank(ctx context.Context, target domain.MusicTarget) (who.Ranking, error) {
	return f(ctx, target)
}

func TestGroupInfoThroughWebhook(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		chatID   int64
		getMeErr error
		wantText string
		target   domain.MusicTarget
	}{
		{name: "case insensitive in allowed group", text: " InFo ", chatID: -100123, wantText: "https://t.me/example_bot"},
		{name: "fallback without username", text: "info", chatID: -100123, getMeErr: errors.New("unavailable"), wantText: "открой личный чат"},
		{name: "other group", text: "info", chatID: -100456},
		{name: "ordinary conversation", text: "что такое info?", chatID: -100123},
		{name: "slash command", text: "/info", chatID: -100123},
		{name: "artist ranking", text: " WkA\tA & B ", chatID: -100123,
			target:   domain.MusicTarget{Kind: domain.Artist, Artist: "A & B"},
			wantText: "Кто слушал «A &amp; B»\n\n<b>42</b> ➠ listener\n\nВсего скробблов: 42"},
		{name: "track ranking", text: " WkTrK\tA - B - Remix ", chatID: -100123,
			target:   domain.MusicTarget{Kind: domain.Track, Artist: "A", Title: "B - Remix"},
			wantText: "Кто слушал «A — B - Remix»\n\n<b>42</b> ➠ listener\n\nВсего скробблов: 42"},
		{name: "album ranking", text: "wkalb \"A - B\" - Album", chatID: -100123,
			target:   domain.MusicTarget{Kind: domain.Album, Artist: "A - B", Title: "Album"},
			wantText: "Кто слушал «A - B — Album»\n\n<b>42</b> ➠ listener\n\nВсего скробблов: 42"},
		{name: "missing artist", text: "wka", chatID: -100123, wantText: "Укажи артиста"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var lookups, replies int
			var target domain.MusicTarget
			httpClient := &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
				switch req.URL.Path {
				case "/bot123:test-token/getMe":
					lookups++
					if tt.getMeErr != nil {
						return nil, tt.getMeErr
					}
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader(`{"ok":true,"result":{"username":"example_bot"}}`)),
					}, nil
				case "/bot123:test-token/sendMessage":
					replies++
					var payload struct {
						ChatID    int64  `json:"chat_id"`
						ThreadID  int64  `json:"message_thread_id"`
						Text      string `json:"text"`
						ParseMode string `json:"parse_mode"`
					}
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatalf("decode Telegram request: %v", err)
					}
					if payload.ChatID != tt.chatID || payload.ThreadID != 15 || !strings.Contains(payload.Text, tt.wantText) {
						t.Fatalf("unexpected Telegram payload: %+v", payload)
					}
					wantParseMode := ""
					if strings.HasPrefix(strings.ToLower(strings.TrimSpace(tt.text)), "wk") {
						wantParseMode = "HTML"
					}
					if payload.ParseMode != wantParseMode {
						t.Fatalf("parse mode = %q, want %q", payload.ParseMode, wantParseMode)
					}
					return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"ok":true}`))}, nil
				default:
					t.Fatalf("unexpected Telegram request: %s", req.URL.Path)
					return nil, nil
				}
			})}
			private := webhook.MessageHandlerFunc(func(context.Context, webhook.IncomingMessage) error {
				t.Fatal("group message reached private handler")
				return nil
			})
			ranker := rankerFunc(func(_ context.Context, item domain.MusicTarget) (who.Ranking, error) {
				target = item
				return who.Ranking{
					Entries: []who.Entry{{Participant: domain.Participant{TelegramUsername: "listener"}, Scrobbles: 42}},
					Total:   42,
				}, nil
			})
			group := handler.NewGroup(client.New(httpClient, "123:test-token"), ranker, nil, nil)
			messages := handler.NewRouter(private, group)
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			server := httpserver.New(8080, webhook.New("test-secret", -100123, messages, group, logger))
			body := fmt.Sprintf(`{"update_id":42,"message":{"message_id":7,"message_thread_id":15,"text":%q,"chat":{"id":%d,"type":"supergroup"},"from":{"id":123}}}`, tt.text, tt.chatID)
			req := httptest.NewRequest(http.MethodPost, "/telegram/webhook", strings.NewReader(body))
			req.Header.Set("X-Telegram-Bot-Api-Secret-Token", "test-secret")
			response := httptest.NewRecorder()
			server.Handler.ServeHTTP(response, req)

			wantCalls := 0
			if tt.wantText != "" {
				wantCalls = 1
			}
			wantLookups := 0
			if strings.EqualFold(strings.TrimSpace(tt.text), "info") && wantCalls == 1 {
				wantLookups = 1
			}
			if response.Code != http.StatusNoContent || lookups != wantLookups || replies != wantCalls || target != tt.target {
				t.Fatalf("status = %d, getMe calls = %d, sendMessage calls = %d, target = %+v", response.Code, lookups, replies, target)
			}
		})
	}
}
