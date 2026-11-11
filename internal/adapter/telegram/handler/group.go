package handler

import (
	"context"
	"fmt"
	"html"
	"net/url"
	"strings"

	"github.com/emount4/lastfm/internal/adapter/telegram"
	"github.com/emount4/lastfm/internal/adapter/telegram/webhook"
	"github.com/emount4/lastfm/internal/application/who"
	"github.com/emount4/lastfm/internal/domain"
)

type GroupBot interface {
	Messenger
	SendHTML(ctx context.Context, chatID, threadID int64, text string) error
	BotUsername(context.Context) (string, error)
	SendChoices(ctx context.Context, chatID, threadID int64, text string, buttons []telegram.Button) error
	AnswerCallback(ctx context.Context, id, text string) error
	DeleteMessage(ctx context.Context, chatID, messageID int64) error
	SendStatus(ctx context.Context, chatID, threadID int64, text string) (int64, error)
	SendPhoto(ctx context.Context, chatID, threadID int64, photo []byte) error
}

type Group struct {
	bot      GroupBot
	rankings Ranker
	search   MusicSearcher
	choices  ChoiceStore
	collage  CollageBuilder
}

type Ranker interface {
	Rank(context.Context, domain.MusicTarget) (who.Ranking, error)
}

func NewGroup(bot GroupBot, rankings Ranker, search MusicSearcher, choices ChoiceStore, collage CollageBuilder) *Group {
	return &Group{bot: bot, rankings: rankings, search: search, choices: choices, collage: collage}
}

func (h *Group) HandleMessage(ctx context.Context, message webhook.IncomingMessage) error {
	text := strings.TrimSpace(message.Text)
	if message.Private || text == "" {
		return nil
	}

	fields := strings.Fields(text)
	args := strings.TrimSpace(text[len(fields[0]):])
	switch strings.ToLower(fields[0]) {
	case "info":
		if args == "" {
			return h.sendHelp(ctx, message)
		}
	case "playing":
		if args == "" {
			return h.sendPlaying(ctx, message)
		}
	case "wka":
		return h.sendRanking(ctx, message, domain.MusicTarget{Kind: domain.Artist, Artist: args}, "Укажи артиста. Например: wka Radiohead")
	case "wktrk":
		return h.rankByTitle(ctx, message, domain.Track, args, "Формат: wktrk название или wktrk артист - трек")
	case "wkalb":
		return h.rankByTitle(ctx, message, domain.Album, args, "Формат: wkalb название или wkalb артист - альбом")
	}
	return nil
}

func (h *Group) sendHelp(ctx context.Context, message webhook.IncomingMessage) error {
	const help = "Last.fm в этой группе\n\n" +
		"• info — показать справку\n" +
		"• playing — кто что слушает сейчас\n" +
		"• wka Radiohead — кто слушал артиста и сколько раз\n" +
		"• wktrk Creep — скробблы трека\n" +
		"• wkalb OK Computer — скробблы альбома\n" +
		"Для точного поиска укажи «артист - название».\n\n"
	reply := help + "Для регистрации открой личный чат с ботом и отправь свой ник Last.fm."
	if username, err := h.bot.BotUsername(ctx); err == nil && username != "" {
		reply = help + "Для регистрации отправь свой ник Last.fm в личный чат с ботом:\nhttps://t.me/" + url.PathEscape(username)
	}

	if err := h.bot.SendText(ctx, message.ChatID, message.ThreadID, reply); err != nil {
		return fmt.Errorf("send group help: %w", err)
	}
	return nil
}

func splitArtistAndTitle(args string) (string, string) {
	if quoted, ok := strings.CutPrefix(args, "\""); ok {
		artist, rest, closed := strings.Cut(quoted, "\"")
		title, separated := strings.CutPrefix(rest, " - ")
		if closed && separated {
			return strings.TrimSpace(artist), strings.TrimSpace(title)
		}
		return "", ""
	}
	artist, title, _ := strings.Cut(args, " - ")
	return strings.TrimSpace(artist), strings.TrimSpace(title)
}

func (h *Group) sendRanking(ctx context.Context, message webhook.IncomingMessage, target domain.MusicTarget, usage string) error {
	if target.Artist != "" && (target.Kind == domain.Artist || target.Title != "") {
		ranking, err := h.rankings.Rank(ctx, target)
		if err != nil {
			return fmt.Errorf("rank %s for Telegram: %w", target.Kind, err)
		}
		return h.sendRankingResult(ctx, message, target, ranking)
	}
	if err := h.bot.SendHTML(ctx, message.ChatID, message.ThreadID, usage); err != nil {
		return fmt.Errorf("send ranking usage: %w", err)
	}
	return nil
}

func (h *Group) sendRankingResult(ctx context.Context, message webhook.IncomingMessage, target domain.MusicTarget, ranking who.Ranking) error {
	title := target.Artist
	if target.Title != "" {
		title += " — " + target.Title
	}
	if err := h.bot.SendHTML(ctx, message.ChatID, message.ThreadID, formatRanking(title, ranking)); err != nil {
		return fmt.Errorf("send %s ranking: %w", target.Kind, err)
	}
	return nil
}

func formatRanking(title string, ranking who.Ranking) string {
	var reply strings.Builder
	fmt.Fprintf(&reply, "Кто слушал «%s»\n\n", html.EscapeString(title))
	if len(ranking.Entries) == 0 {
		reply.WriteString("Нет результатов.")
		return reply.String()
	}
	for _, entry := range ranking.Entries {
		name := entry.Participant.TelegramDisplayName
		if entry.Participant.TelegramUsername != "" {
			name = entry.Participant.TelegramUsername
		}
		fmt.Fprintf(&reply, "<b>%d</b> ➠ %s\n", entry.Scrobbles, html.EscapeString(name))
	}
	fmt.Fprintf(&reply, "\nВсего скробблов: %d", ranking.Total)
	return reply.String()
}
