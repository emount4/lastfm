package handler

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"strings"

	"github.com/emount4/lastfm/internal/adapter/telegram/webhook"
	"github.com/emount4/lastfm/internal/application/top"
	"github.com/emount4/lastfm/internal/domain"
)

type ArtistTop interface {
	Artists(context.Context, domain.Period) (top.ArtistRanking, error)
}

func (h *Group) sendTopArtists(ctx context.Context, message webhook.IncomingMessage, args string) error {
	period, err := domain.ParsePeriod(args)
	if err != nil {
		return h.sendTopUsage(ctx, message)
	}

	ranking, err := h.top.Artists(ctx, period)
	if errors.Is(err, domain.ErrInvalidPeriod) {
		return h.sendTopUsage(ctx, message)
	}
	if err != nil {
		slog.ErrorContext(ctx, "Не удалось загрузить топ артистов", "update_id", message.UpdateID, "error", err)
		if replyErr := h.bot.SendText(ctx, message.ChatID, message.ThreadID, "Не удалось загрузить топ артистов. Попробуй ещё раз."); replyErr != nil {
			return errors.Join(err, fmt.Errorf("send topart failure notice: %w", replyErr))
		}
		return nil
	}

	var reply strings.Builder
	label := "за всё время"
	if !period.AllTime() {
		label = "за " + period.String()
	}
	fmt.Fprintf(&reply, "Топ артистов группы · %s\n\n", label)
	if len(ranking.Entries) == 0 {
		reply.WriteString("Нет результатов.\n")
	}
	for _, entry := range ranking.Entries {
		fmt.Fprintf(&reply, "<b>%d</b> ➠ %s\n", entry.Scrobbles, html.EscapeString(entry.Name))
	}
	if ranking.ProfilesIncluded < ranking.ProfilesTotal {
		fmt.Fprintf(&reply, "\nПо данным %d из %d профилей.", ranking.ProfilesIncluded, ranking.ProfilesTotal)
	}
	if err := h.bot.SendHTML(ctx, message.ChatID, message.ThreadID, strings.TrimSpace(reply.String())); err != nil {
		return fmt.Errorf("send group artist chart: %w", err)
	}
	return nil
}

func (h *Group) sendTopUsage(ctx context.Context, message webhook.IncomingMessage) error {
	const usage = "Укажи период, например topart 10d, topart 2w, topart 3m или topart 2y. Без периода считаю за всё время."
	if err := h.bot.SendText(ctx, message.ChatID, message.ThreadID, usage); err != nil {
		return fmt.Errorf("send topart usage: %w", err)
	}
	return nil
}
