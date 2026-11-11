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
	"github.com/emount4/lastfm/internal/application/top"
	"github.com/emount4/lastfm/internal/application/who"
	"github.com/emount4/lastfm/internal/domain"
)

type rankerFunc func(context.Context, domain.MusicTarget) (who.Ranking, error)

func (f rankerFunc) Rank(ctx context.Context, target domain.MusicTarget) (who.Ranking, error) {
	return f(ctx, target)
}

type topFunc func(context.Context, domain.MusicKind, domain.Period) (top.Ranking, error)

func (f topFunc) Rank(ctx context.Context, kind domain.MusicKind, period domain.Period) (top.Ranking, error) {
	return f(ctx, kind, period)
}

func TestGroupInfoThroughWebhook(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		chatID   int64
		getMeErr error
		wantText string
		target   domain.MusicTarget
		topKind  domain.MusicKind
		period   string
	}{
		{name: "case insensitive in allowed group", text: " InFo ", chatID: -100123, wantText: "https://t.me/example_bot"},
		{name: "fallback without username", text: "info", chatID: -100123, getMeErr: errors.New("unavailable"), wantText: "<b>Регистрация</b>\nОтправь свой ник Last.fm в личный чат с ботом."},
		{name: "other group", text: "info", chatID: -100456},
		{name: "ordinary conversation", text: "что такое info?", chatID: -100123},
		{name: "unlink in group", text: "unlink", chatID: -100123},
		{name: "slash command", text: "/info", chatID: -100123},
		{name: "artist ranking", text: " WkA\tA & B ", chatID: -100123,
			target:   domain.MusicTarget{Kind: domain.Artist, Artist: "A & B"},
			wantText: "Кто слушал «A &amp; B»\n\n<b>42</b> ➠ Анна &amp; Ко\n<b>7</b> ➠ listener\n\nВсего скробблов: 49"},
		{name: "track ranking", text: " WkTrK\tA - B - Remix ", chatID: -100123,
			target:   domain.MusicTarget{Kind: domain.Track, Artist: "A", Title: "B - Remix"},
			wantText: "Кто слушал «A — B - Remix»\n\n<b>42</b> ➠ Анна &amp; Ко\n<b>7</b> ➠ listener\n\nВсего скробблов: 49"},
		{name: "album ranking", text: "wkalb \"A - B\" - Album", chatID: -100123,
			target:   domain.MusicTarget{Kind: domain.Album, Artist: "A - B", Title: "Album"},
			wantText: "Кто слушал «A - B — Album»\n\n<b>42</b> ➠ Анна &amp; Ко\n<b>7</b> ➠ listener\n\nВсего скробблов: 49"},
		{name: "missing artist", text: "wka", chatID: -100123, wantText: "Укажи артиста"},
		{name: "group top", text: " ToPaRt ", chatID: -100123, topKind: domain.Artist,
			wantText: "Топ 25 артистов за всё время:\n\n1. <b>A &amp; B</b> ➠ <b>121</b>\n\nПо данным 2 из 3 профилей."},
		{name: "group top for a period", text: "TOPART 2W", chatID: -100123, topKind: domain.Artist, period: "2w",
			wantText: "Топ 25 артистов за 2w:\n\n1. <b>A &amp; B</b> ➠ <b>121</b>"},
		{name: "group album top", text: "topalb", chatID: -100123, topKind: domain.Album,
			wantText: "Топ 25 альбомов за всё время:\n\n1. <b>A &amp; B — Release</b> ➠ <b>121</b>"},
		{name: "group track top", text: "TOPTRK 3M", chatID: -100123, topKind: domain.Track, period: "3m",
			wantText: "Топ 25 треков за 3m:\n\n1. <b>A &amp; B — Song</b> ➠ <b>121</b>"},
		{name: "invalid period", text: "topart 0w", chatID: -100123, wantText: "Укажи период"},
		{name: "invalid album period", text: "topalb 0w", chatID: -100123, wantText: "например topalb 10d"},
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
					command := strings.ToLower(strings.TrimSpace(tt.text))
					if tt.topKind != "" || command == "info" || strings.HasPrefix(command, "wk") {
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
					Entries: []who.Entry{
						{Participant: domain.Participant{TelegramDisplayName: " Анна & Ко ", TelegramUsername: "anna"}, Scrobbles: 42},
						{Participant: domain.Participant{TelegramDisplayName: " ", TelegramUsername: "listener"}, Scrobbles: 7},
					},
					Total: 49,
				}, nil
			})
			groupTop := topFunc(func(_ context.Context, kind domain.MusicKind, period domain.Period) (top.Ranking, error) {
				if kind != tt.topKind || period.String() != tt.period {
					t.Fatalf("unexpected group top request with kind %q and period %q", kind, period.String())
				}
				title := map[domain.MusicKind]string{domain.Track: "Song", domain.Album: "Release"}[kind]
				return top.Ranking{
					Entries:          []domain.ChartEntry{{Artist: "A & B", Title: title, Scrobbles: 121}},
					ProfilesIncluded: 2,
					ProfilesTotal:    3,
				}, nil
			})
			group := handler.NewGroup(client.New(httpClient, "123:test-token"), ranker, nil, nil, nil, groupTop)
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
