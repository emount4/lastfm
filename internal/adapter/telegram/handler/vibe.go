package handler

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"strings"

	"github.com/emount4/lastfm/internal/adapter/telegram/webhook"
	"github.com/emount4/lastfm/internal/application/vibe"
)

type VibeDescriber interface {
	Describe(context.Context, int64, int) (string, error)
}

func (h *Group) sendVibe(ctx context.Context, message webhook.IncomingMessage, args string) error {
	reply := func(text string) error {
		if err := h.bot.SendText(ctx, message.ChatID, message.ThreadID, text); err != nil {
			return fmt.Errorf("send scrobble vibe notice: %w", err)
		}
		return nil
	}
	days, err := vibe.ParseDays(args)
	if err != nil {
		return reply(fmt.Sprintf("Напиши «скроббовайб» за сегодня или «скроббовайб 3d» за три дня. Можно указать от 1 до %d дней.", vibe.MaxDays))
	}
	if h.vibe == nil {
		return reply("Скроббовайб пока не подключён.")
	}

	description, err := h.vibe.Describe(ctx, message.UserID, days)
	switch {
	case errors.Is(err, vibe.ErrNotRegistered):
		return reply("Сначала отправь свой ник Last.fm в личку боту. Ссылка есть в info.")
	case errors.Is(err, vibe.ErrNoScrobbles):
		return reply("За этот период у тебя пока нет скробблов.")
	case err != nil:
		slog.ErrorContext(ctx, "Не удалось составить скроббовайб", "update_id", message.UpdateID, "error", err)
		if errors.Is(err, vibe.ErrRateLimited) {
			return reply("Сработал лимит запросов к Gemma. Попробуй чуть позже.")
		}
		return reply("Не получилось составить скроббовайб. Попробуй чуть позже.")
	}

	period := "сегодня"
	if days > 1 {
		period = fmt.Sprintf("за %d дн.", days)
	}
	title := "Твой скроббовайб " + period
	name := strings.TrimSpace(message.DisplayName)
	if name == "" {
		name = strings.TrimSpace(message.Username)
	}
	if name != "" {
		title = name + ", твой скроббовайб " + period
	}
	text := fmt.Sprintf("🎧 <b>%s</b>\n\n%s", html.EscapeString(title), html.EscapeString(description))
	if err := h.bot.SendHTML(ctx, message.ChatID, message.ThreadID, text); err != nil {
		return fmt.Errorf("send scrobble vibe: %w", err)
	}
	return nil
}
