package webhook_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/emount4/lastfm/internal/adapter/httpserver"
	"github.com/emount4/lastfm/internal/adapter/telegram/webhook"
)

func TestTelegramWebhook(t *testing.T) {
	const private = `{"update_id":42,"message":{"message_id":7,"text":"Example","chat":{"id":12345678901,"type":"private"},"from":{"id":12345678901,"username":"example","first_name":"Анна","last_name":"Иванова"}}}`
	const group = `{"update_id":43,"message":{"message_id":8,"message_thread_id":15,"text":"playing","chat":{"id":-1001234567890,"type":"supergroup"},"from":{"id":12345678901}}}`
	const secret = "test-webhook-secret"
	const groupID = -1001234567890

	tests := []struct {
		name       string
		method     string
		secret     string
		body       string
		handlerErr error
		wantStatus int
		want       *webhook.IncomingMessage
	}{
		{
			name: "private message", method: "POST", secret: secret, body: private, wantStatus: 204,
			want: &webhook.IncomingMessage{
				UpdateID: 42, MessageID: 7, ChatID: 12345678901, Private: true,
				UserID: 12345678901, Username: "example", DisplayName: "Анна Иванова", Text: "Example",
			},
		},
		{
			name: "allowed group topic", method: "POST", secret: secret, body: group, wantStatus: 204,
			want: &webhook.IncomingMessage{
				UpdateID: 43, MessageID: 8, ThreadID: 15, ChatID: groupID, UserID: 12345678901, Text: "playing",
			},
		},
		{name: "another group", method: "POST", secret: secret, body: strings.Replace(group, "-1001234567890", "-1001234567891", 1), wantStatus: 204},
		{name: "unsupported update", method: "POST", secret: secret, body: `{"update_id":44,"edited_message":{}}`, wantStatus: 204},
		{name: "missing secret", method: "POST", body: private, wantStatus: 403},
		{name: "wrong secret", method: "POST", secret: "wrong", body: private, wantStatus: 403},
		{name: "invalid JSON", method: "POST", secret: secret, body: "{", wantStatus: 400},
		{name: "trailing JSON", method: "POST", secret: secret, body: private + "{}", wantStatus: 400},
		{name: "large update", method: "POST", secret: secret, body: strings.Repeat(" ", (1<<20)+1), wantStatus: 413},
		{name: "wrong method", method: "GET", secret: secret, body: private, wantStatus: 405},
		{
			name: "handler failure", method: "POST", secret: secret, body: private, wantStatus: 500,
			handlerErr: errors.New("temporary failure"),
			want: &webhook.IncomingMessage{
				UpdateID: 42, MessageID: 7, ChatID: 12345678901, Private: true,
				UserID: 12345678901, Username: "example", DisplayName: "Анна Иванова", Text: "Example",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var received []webhook.IncomingMessage
			messages := webhook.MessageHandlerFunc(func(_ context.Context, msg webhook.IncomingMessage) error {
				received = append(received, msg)
				return tt.handlerErr
			})
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			server := httpserver.New(8080, webhook.New(secret, groupID, messages, logger))
			req := httptest.NewRequest(tt.method, "/telegram/webhook", strings.NewReader(tt.body))
			req.Header.Set("X-Telegram-Bot-Api-Secret-Token", tt.secret)
			response := httptest.NewRecorder()
			server.Handler.ServeHTTP(response, req)

			if response.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, tt.wantStatus)
			}
			if tt.want == nil {
				if len(received) != 0 {
					t.Fatalf("unexpected dispatched messages: %+v", received)
				}
			} else if len(received) != 1 || received[0] != *tt.want {
				t.Fatalf("dispatched messages = %+v, want %+v", received, *tt.want)
			}
		})
	}
}
