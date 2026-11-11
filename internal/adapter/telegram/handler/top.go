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

type TopRanker interface {
	Rank(context.Context, domain.MusicKind, domain.Period) (top.Ranking, error)
	Users(context.Context, domain.Period) (top.UserRanking, error)
}

type topLine struct {
	name      string
	scrobbles int64
}

type topReply struct {
	lines            []topLine
	profilesIncluded int
	profilesTotal    int
}

func (h *Group) sendTop(ctx context.Context, message webhook.IncomingMessage, command, args string, kind domain.MusicKind, subject string) error {
	return h.sendTopReply(ctx, message, command, args, subject, func(period domain.Period) (topReply, error) {
		ranking, err := h.top.Rank(ctx, kind, period)
		if err != nil {
			return topReply{}, fmt.Errorf("rank %s: %w", kind, err)
		}
		reply := topReply{profilesIncluded: ranking.ProfilesIncluded, profilesTotal: ranking.ProfilesTotal}
		for _, entry := range ranking.Entries {
			name := entry.Artist
			if entry.Title != "" {
				name += " — " + entry.Title
			}
			reply.lines = append(reply.lines, topLine{name: name, scrobbles: entry.Scrobbles})
		}
		return reply, nil
	})
}

func (h *Group) sendTopUsers(ctx context.Context, message webhook.IncomingMessage, args string) error {
	return h.sendTopReply(ctx, message, "topusers", args, "слушателей", func(period domain.Period) (topReply, error) {
		ranking, err := h.top.Users(ctx, period)
		if err != nil {
			return topReply{}, fmt.Errorf("rank group listeners: %w", err)
		}
		reply := topReply{profilesIncluded: ranking.ProfilesIncluded, profilesTotal: ranking.ProfilesTotal}
		for _, entry := range ranking.Entries {
			if name := entry.Participant.TelegramName(); name != "" {
				reply.lines = append(reply.lines, topLine{name: name, scrobbles: entry.Scrobbles})
			}
		}
		return reply, nil
	})
}

func (h *Group) sendTopReply(ctx context.Context, message webhook.IncomingMessage, command, args, subject string, load func(domain.Period) (topReply, error)) error {
	period, err := domain.ParsePeriod(args)
	if err != nil {
		return h.sendTopUsage(ctx, message, command)
	}

	ranking, err := load(period)
	if errors.Is(err, domain.ErrInvalidPeriod) {
		return h.sendTopUsage(ctx, message, command)
	}
	if err != nil {
		slog.ErrorContext(ctx, "Не удалось загрузить топ группы", "command", command, "update_id", message.UpdateID, "error", err)
		if replyErr := h.bot.SendText(ctx, message.ChatID, message.ThreadID, "Не удалось загрузить топ "+subject+". Попробуй ещё раз."); replyErr != nil {
			return errors.Join(err, fmt.Errorf("send %s failure notice: %w", command, replyErr))
		}
		return nil
	}

	var reply strings.Builder
	label := "за всё время"
	if !period.AllTime() {
		label = "за " + period.String()
	}
	fmt.Fprintf(&reply, "Топ %d %s %s:\n\n", top.Limit, subject, label)
	if len(ranking.lines) == 0 {
		reply.WriteString("Нет результатов.\n")
	}
	for index, entry := range ranking.lines {
		fmt.Fprintf(&reply, "%d. <b>%s</b> ➠ <b>%d</b>\n", index+1, html.EscapeString(entry.name), entry.scrobbles)
	}
	if ranking.profilesIncluded < ranking.profilesTotal {
		fmt.Fprintf(&reply, "\nПо данным %d из %d профилей.", ranking.profilesIncluded, ranking.profilesTotal)
	}
	if err := h.bot.SendHTML(ctx, message.ChatID, message.ThreadID, strings.TrimSpace(reply.String())); err != nil {
		return fmt.Errorf("send %s chart: %w", command, err)
	}
	return nil
}

func (h *Group) sendTopUsage(ctx context.Context, message webhook.IncomingMessage, command string) error {
	usage := fmt.Sprintf("Укажи период, например %s 10d, %s 2w, %s 3m или %s 2y. Без периода считаю за всё время.", command, command, command, command)
	if err := h.bot.SendText(ctx, message.ChatID, message.ThreadID, usage); err != nil {
		return fmt.Errorf("send %s usage: %w", command, err)
	}
	return nil
}
