package handler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/emount4/lastfm/internal/adapter/httpserver"
	"github.com/emount4/lastfm/internal/adapter/telegram"
	"github.com/emount4/lastfm/internal/adapter/telegram/client"
	"github.com/emount4/lastfm/internal/adapter/telegram/handler"
	"github.com/emount4/lastfm/internal/adapter/telegram/webhook"
	"github.com/emount4/lastfm/internal/application/who"
	"github.com/emount4/lastfm/internal/domain"
)

type searchFunc func(context.Context, domain.MusicKind, string) ([]domain.MusicTarget, error)

func (f searchFunc) Search(ctx context.Context, kind domain.MusicKind, title string) ([]domain.MusicTarget, error) {
	return f(ctx, kind, title)
}

func (f searchFunc) FindTitleVariants(ctx context.Context, target domain.MusicTarget) ([]domain.MusicTarget, error) {
	return f(ctx, target.Kind, target.Title)
}

type choiceFixture struct {
	id       string
	chatID   int64
	threadID int64
	options  []domain.MusicTarget
}

func (f *choiceFixture) SaveChoices(_ context.Context, id string, chatID, threadID int64, options []domain.MusicTarget) error {
	*f = choiceFixture{id: id, chatID: chatID, threadID: threadID, options: options}
	return nil
}

func (f *choiceFixture) TakeChoice(_ context.Context, id string, chatID int64, index int) (domain.MusicTarget, int64, bool, error) {
	if id != f.id || chatID != f.chatID || index < 0 || index >= len(f.options) {
		return domain.MusicTarget{}, 0, false, nil
	}
	target := f.options[index]
	f.options = nil
	return target, f.threadID, true, nil
}

func TestAlbumChoiceThroughWebhook(t *testing.T) {
	const groupID = -100123
	options := []domain.MusicTarget{
		{Kind: domain.Album, Artist: "A", Title: "Album"},
		{Kind: domain.Album, Artist: "B", Title: "Album"},
	}
	search := searchFunc(func(_ context.Context, kind domain.MusicKind, title string) ([]domain.MusicTarget, error) {
		if kind != domain.Album || title != "Album.." {
			t.Fatalf("search = %s %q", kind, title)
		}
		return options, nil
	})

	var calls []string
	var chosenButton string
	telegramHTTP := &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/bot123:test-token/sendMessage":
			var payload struct {
				ChatID      int64  `json:"chat_id"`
				ThreadID    int64  `json:"message_thread_id"`
				Text        string `json:"text"`
				ReplyMarkup struct {
					Buttons [][]telegram.Button `json:"inline_keyboard"`
				} `json:"reply_markup"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			if payload.ChatID != groupID || payload.ThreadID != 15 {
				t.Fatalf("reply sent to chat %d thread %d", payload.ChatID, payload.ThreadID)
			}
			if len(payload.ReplyMarkup.Buttons) > 0 {
				calls = append(calls, "choices")
				buttons := payload.ReplyMarkup.Buttons
				if len(buttons) != 2 || len(buttons[1]) != 1 || buttons[1][0].Text != "B — Album" {
					t.Fatalf("unexpected buttons: %+v", buttons)
				}
				chosenButton = buttons[1][0].Data
				if len(chosenButton) > 64 {
					t.Fatal("callback data exceeds Telegram limit")
				}
			} else {
				calls = append(calls, "result")
				if !strings.Contains(payload.Text, "Кто слушал «B — Album»") {
					t.Fatalf("unexpected ranking: %s", payload.Text)
				}
			}
		case "/bot123:test-token/answerCallbackQuery":
			calls = append(calls, "answer")
		case "/bot123:test-token/deleteMessage":
			calls = append(calls, "delete")
		default:
			t.Fatalf("unexpected Telegram request: %s", req.URL.Path)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"ok":true}`))}, nil
	})}
	ranker := rankerFunc(func(_ context.Context, target domain.MusicTarget) (who.Ranking, error) {
		if target == (domain.MusicTarget{Kind: domain.Album, Artist: "B", Title: "Album.."}) {
			calls = append(calls, "rank input")
			return who.Ranking{}, nil
		}
		calls = append(calls, "rank")
		if target != options[1] {
			t.Fatalf("ranked target = %+v, want %+v", target, options[1])
		}
		return who.Ranking{}, nil
	})
	group := handler.NewGroup(client.New(telegramHTTP, "123:test-token"), ranker, search, &choiceFixture{}, nil, nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := httpserver.New(8080, webhook.New("test-secret", groupID, group, group, logger))
	deliver := func(body string) {
		req := httptest.NewRequest(http.MethodPost, "/telegram/webhook", strings.NewReader(body))
		req.Header.Set("X-Telegram-Bot-Api-Secret-Token", "test-secret")
		response := httptest.NewRecorder()
		server.Handler.ServeHTTP(response, req)
		if response.Code != http.StatusNoContent {
			t.Fatalf("webhook status = %d, body = %s", response.Code, response.Body)
		}
	}

	deliver(`{"update_id":41,"message":{"message_id":7,"message_thread_id":15,"text":"wkalb B - Album..","chat":{"id":-100123,"type":"supergroup"},"from":{"id":123}}}`)
	if !slices.Equal(calls, []string{"rank input", "choices"}) {
		t.Fatalf("calls before choice = %v", calls)
	}
	callback := fmt.Sprintf(`{"update_id":42,"callback_query":{"id":"choice","data":%q,"message":{"message_id":8,"chat":{"id":-100123,"type":"supergroup"}}}}`, chosenButton)
	deliver(callback)
	deliver(callback)
	if !slices.Equal(calls, []string{"rank input", "choices", "answer", "delete", "rank", "result", "answer"}) {
		t.Fatalf("calls after choice = %v", calls)
	}
}
