package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/emount4/lastfm/internal/adapter/telegram/webhook"
	"github.com/emount4/lastfm/internal/application/registration"
	"github.com/emount4/lastfm/internal/domain"
)

const registrationPrompt = "Пришли свой ник Last.fm одним сообщением. Твой ник Telegram и статистика прослушиваний будут видны в группе. Чтобы отвязать профиль, отправь unlink."

type Registrar interface {
	Register(context.Context, domain.Participant) (string, error)
	Unlink(context.Context, int64) error
}

type Messenger interface {
	SendText(ctx context.Context, chatID, threadID int64, text string) error
}

type Registration struct {
	registrar Registrar
	messenger Messenger
	logger    *slog.Logger
}

func NewRegistration(registrar Registrar, messenger Messenger, logger *slog.Logger) *Registration {
	return &Registration{registrar: registrar, messenger: messenger, logger: logger}
}

func (h *Registration) HandleMessage(ctx context.Context, message webhook.IncomingMessage) error {
	if !message.Private {
		return nil
	}

	text := strings.TrimSpace(message.Text)
	command, _, _ := strings.Cut(text, " ")
	switch {
	case text == "", strings.EqualFold(command, "/start"):
		return h.reply(ctx, message, registrationPrompt)
	case strings.EqualFold(text, "unlink"):
		if err := h.registrar.Unlink(ctx, message.UserID); err != nil {
			h.logger.ErrorContext(ctx, "Не удалось отвязать профиль", "update_id", message.UpdateID, "error", err)
			return h.reply(ctx, message, "Не удалось отвязать профиль. Попробуй ещё раз чуть позже.")
		}
		return h.reply(ctx, message, "У тебя больше нет привязки Last.fm. Чтобы привязать профиль снова, пришли свой ник.")
	case strings.HasPrefix(text, "/"):
		return nil
	}

	name, err := h.registrar.Register(ctx, domain.Participant{
		TelegramUserID:      message.UserID,
		TelegramUsername:    message.Username,
		TelegramDisplayName: message.DisplayName,
		LastFMUsername:      text,
	})
	switch {
	case err == nil:
		return h.reply(ctx, message, "Привязал профиль Last.fm: "+name+".")
	case errors.Is(err, registration.ErrUsernameRequired):
		return h.reply(ctx, message, registrationPrompt)
	case errors.Is(err, registration.ErrProfileNotFound):
		return h.reply(ctx, message, "Профиль Last.fm не найден. Проверь ник и пришли его ещё раз.")
	case errors.Is(err, domain.ErrLastFMAccountTaken):
		return h.reply(ctx, message, "Этот профиль Last.fm уже привязан к другому участнику.")
	default:
		h.logger.ErrorContext(ctx, "registration failed", "update_id", message.UpdateID, "error", err)
		return h.reply(ctx, message, "Не удалось привязать профиль. Попробуй ещё раз чуть позже.")
	}
}

func (h *Registration) reply(ctx context.Context, message webhook.IncomingMessage, text string) error {
	if err := h.messenger.SendText(ctx, message.ChatID, message.ThreadID, text); err != nil {
		return fmt.Errorf("reply to registration message: %w", err)
	}
	return nil
}
