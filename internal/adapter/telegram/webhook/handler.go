package webhook

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

const secretHeader = "X-Telegram-Bot-Api-Secret-Token"
const maxUpdateBytes = 1 << 20

type MessageHandler interface {
	HandleMessage(context.Context, IncomingMessage) error
}

type MessageHandlerFunc func(context.Context, IncomingMessage) error

func (f MessageHandlerFunc) HandleMessage(ctx context.Context, message IncomingMessage) error {
	return f(ctx, message)
}

type Handler struct {
	secret   []byte
	groupID  int64
	messages MessageHandler
	logger   *slog.Logger
}

func New(secret string, groupID int64, messages MessageHandler, logger *slog.Logger) *Handler {
	return &Handler{secret: []byte(secret), groupID: groupID, messages: messages, logger: logger}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if len(h.secret) == 0 || subtle.ConstantTimeCompare([]byte(r.Header.Get(secretHeader)), h.secret) != 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxUpdateBytes))
	if err != nil {
		var limit *http.MaxBytesError
		if errors.As(err, &limit) {
			http.Error(w, "update is too large", http.StatusRequestEntityTooLarge)
		} else {
			http.Error(w, "could not read update", http.StatusBadRequest)
		}
		return
	}

	var incoming update
	if err := json.Unmarshal(body, &incoming); err != nil || incoming.ID <= 0 {
		http.Error(w, "invalid update", http.StatusBadRequest)
		return
	}

	msg := incoming.Message
	if msg == nil || msg.ID <= 0 || msg.From == nil || msg.From.ID <= 0 || msg.From.IsBot || strings.TrimSpace(msg.Text) == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	private := msg.Chat.Type == "private" && msg.Chat.ID == msg.From.ID
	allowedGroup := msg.Chat.ID == h.groupID && (msg.Chat.Type == "group" || msg.Chat.Type == "supergroup")
	if !private && !allowedGroup {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	err = h.messages.HandleMessage(r.Context(), IncomingMessage{
		UpdateID:    incoming.ID,
		MessageID:   msg.ID,
		ThreadID:    msg.ThreadID,
		ChatID:      msg.Chat.ID,
		Private:     private,
		UserID:      msg.From.ID,
		Username:    msg.From.Username,
		DisplayName: strings.TrimSpace(msg.From.FirstName + " " + msg.From.LastName),
		Text:        msg.Text,
	})
	if err != nil {
		h.logger.ErrorContext(r.Context(), "Telegram message handling failed", "update_id", incoming.ID, "error", err)
		http.Error(w, "could not handle update", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
