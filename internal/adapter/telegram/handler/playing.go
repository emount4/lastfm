package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/emount4/lastfm/internal/adapter/telegram/webhook"
)

type CollageBuilder interface {
	Build(context.Context) ([]byte, error)
}

func (h *Group) sendPlaying(ctx context.Context, message webhook.IncomingMessage) error {
	statusID, statusErr := h.bot.SendStatus(ctx, message.ChatID, message.ThreadID, "Генерирую коллаж…")
	if statusErr != nil {
		slog.WarnContext(ctx, "Не удалось отправить статус коллажа", "update_id", message.UpdateID, "error", statusErr)
	}

	photo, buildErr := h.collage.Build(ctx)
	if statusID > 0 {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		if err := h.bot.DeleteMessage(cleanupCtx, message.ChatID, statusID); err != nil {
			slog.WarnContext(ctx, "Не удалось удалить статус коллажа", "update_id", message.UpdateID, "error", err)
		}
		cancel()
	}

	if buildErr == nil && len(photo) != 0 {
		if err := h.bot.SendPhoto(ctx, message.ChatID, message.ThreadID, photo); err != nil {
			return fmt.Errorf("send playing collage: %w", err)
		}
		return nil
	}

	reply := "Нет данных о текущем прослушивании."
	if buildErr != nil {
		slog.ErrorContext(ctx, "Не удалось собрать коллаж", "update_id", message.UpdateID, "error", buildErr)
		reply = "Не удалось собрать коллаж. Попробуй ещё раз."
	}
	if err := h.bot.SendText(ctx, message.ChatID, message.ThreadID, reply); err != nil {
		return errors.Join(buildErr, fmt.Errorf("send playing reply: %w", err))
	}
	return nil
}
