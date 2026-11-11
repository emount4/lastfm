package handler_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/emount4/lastfm/internal/adapter/httpserver"
	"github.com/emount4/lastfm/internal/adapter/telegram/client"
	"github.com/emount4/lastfm/internal/adapter/telegram/handler"
	"github.com/emount4/lastfm/internal/adapter/telegram/webhook"
	"github.com/emount4/lastfm/internal/application/playing"
)

type collageFunc func(context.Context) (playing.CollageResult, error)

func (f collageFunc) Build(ctx context.Context) (playing.CollageResult, error) {
	return f(ctx)
}

func TestPlayingRemovesStatusBeforeReply(t *testing.T) {
	for _, tt := range []struct {
		name  string
		photo []byte
		count int
		last  string
	}{
		{name: "photo", photo: []byte("test jpeg"), count: 3, last: "photo"},
		{name: "no active listeners", last: "empty"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var calls []string
			httpClient := &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
				body := `{"ok":true}`
				switch req.URL.Path {
				case "/bot123:test-token/sendMessage", "/bot123:test-token/deleteMessage":
					var payload struct {
						ChatID    int64  `json:"chat_id"`
						ThreadID  int64  `json:"message_thread_id"`
						MessageID int64  `json:"message_id"`
						Text      string `json:"text"`
					}
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil || payload.ChatID != -100123 {
						t.Fatalf("unexpected request: %+v, %v", payload, err)
					}
					switch {
					case payload.MessageID == 55:
						calls = append(calls, "delete")
					case payload.ThreadID == 15 && payload.Text == "Генерирую коллаж…":
						calls = append(calls, "status")
						body = `{"ok":true,"result":{"message_id":55}}`
					case payload.ThreadID == 15 && payload.Text == "Нет данных о текущем прослушивании.":
						calls = append(calls, "empty")
					default:
						t.Fatalf("unexpected payload: %+v", payload)
					}
				case "/bot123:test-token/sendPhoto":
					calls = append(calls, "photo")
					_, params, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
					if err != nil {
						t.Fatal(err)
					}
					form, err := multipart.NewReader(req.Body, params["boundary"]).ReadForm(1024)
					if err != nil {
						t.Fatal(err)
					}
					defer form.RemoveAll()
					if !slices.Equal(form.Value["chat_id"], []string{"-100123"}) ||
						!slices.Equal(form.Value["message_thread_id"], []string{"15"}) ||
						!slices.Equal(form.Value["caption"], []string{"🎧 Что сейчас играет в группе\nСлушателей: 3"}) ||
						len(form.File["photo"]) != 1 || form.File["photo"][0].Size != int64(len(tt.photo)) {
						t.Fatalf("unexpected photo form: %+v", form)
					}
				default:
					t.Fatalf("unexpected Telegram request: %s", req.URL.Path)
				}
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
			})}
			builder := collageFunc(func(context.Context) (playing.CollageResult, error) {
				calls = append(calls, "build")
				return playing.CollageResult{Photo: tt.photo, ListenerCount: tt.count}, nil
			})
			group := handler.NewGroup(client.New(httpClient, "123:test-token"), nil, nil, nil, builder, nil, nil)
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			server := httpserver.New(8080, webhook.New("test-secret", -100123, group, group, logger))
			const body = `{"update_id":42,"message":{"message_id":7,"message_thread_id":15,"text":" PlAyInG ","chat":{"id":-100123,"type":"supergroup"},"from":{"id":123}}}`
			req := httptest.NewRequest(http.MethodPost, "/telegram/webhook", strings.NewReader(body))
			req.Header.Set("X-Telegram-Bot-Api-Secret-Token", "test-secret")
			response := httptest.NewRecorder()
			server.Handler.ServeHTTP(response, req)
			if response.Code != http.StatusNoContent || !slices.Equal(calls, []string{"status", "build", "delete", tt.last}) {
				t.Fatalf("status = %d, calls = %v", response.Code, calls)
			}
		})
	}
}
