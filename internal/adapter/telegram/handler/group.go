package handler

import (
	"context"
	"fmt"
	"html"
	"net/url"
	"strings"

	"github.com/emount4/lastfm/internal/adapter/telegram/webhook"
	"github.com/emount4/lastfm/internal/application/who"
)

type GroupBot interface {
	Messenger
	SendHTML(ctx context.Context, chatID, threadID int64, text string) error
	BotUsername(context.Context) (string, error)
}

type Group struct {
	bot     GroupBot
	artists ArtistRanker
}

type ArtistRanker interface {
	RankArtist(context.Context, string) (who.Ranking, error)
}

func NewGroup(bot GroupBot, artists ArtistRanker) *Group {
	return &Group{bot: bot, artists: artists}
}

func (h *Group) HandleMessage(ctx context.Context, message webhook.IncomingMessage) error {
	text := strings.TrimSpace(message.Text)
	if message.Private || text == "" {
		return nil
	}

	fields := strings.Fields(text)
	command := fields[0]
	args := strings.TrimSpace(text[len(command):])
	switch {
	case strings.EqualFold(command, "info") && args == "":
		return h.sendHelp(ctx, message)
	case strings.EqualFold(command, "wka"):
		return h.sendArtistRanking(ctx, message, args)
	}
	return nil
}

func (h *Group) sendHelp(ctx context.Context, message webhook.IncomingMessage) error {
	const help = "Last.fm в этой группе\n\n" +
		"• info — показать справку\n" +
		"• wka <артист> — кто слушал артиста и сколько раз\n" +
		"  Например: wka Radiohead\n\n"
	reply := help + "Для регистрации открой личный чат с ботом и отправь свой ник Last.fm."
	if username, err := h.bot.BotUsername(ctx); err == nil && username != "" {
		reply = help + "Для регистрации отправь свой ник Last.fm в личный чат с ботом:\nhttps://t.me/" + url.PathEscape(username)
	}

	if err := h.bot.SendText(ctx, message.ChatID, message.ThreadID, reply); err != nil {
		return fmt.Errorf("send group help: %w", err)
	}
	return nil
}

func (h *Group) sendArtistRanking(ctx context.Context, message webhook.IncomingMessage, artist string) error {
	reply := "Укажи артиста. Например: wka Radiohead"
	if artist != "" {
		ranking, err := h.artists.RankArtist(ctx, artist)
		if err != nil {
			return fmt.Errorf("rank artist for Telegram: %w", err)
		}
		reply = formatArtistRanking(artist, ranking)
	}

	if err := h.bot.SendHTML(ctx, message.ChatID, message.ThreadID, reply); err != nil {
		return fmt.Errorf("send artist ranking: %w", err)
	}
	return nil
}

func formatArtistRanking(artist string, ranking who.Ranking) string {
	var reply strings.Builder
	fmt.Fprintf(&reply, "Кто слушал «%s»\n\n", html.EscapeString(artist))
	if len(ranking.Entries) == 0 {
		reply.WriteString("Нет результатов.")
		return reply.String()
	}
	for i, entry := range ranking.Entries {
		name := entry.Participant.TelegramDisplayName
		if entry.Participant.TelegramUsername != "" {
			name = entry.Participant.TelegramUsername
		}
		fmt.Fprintf(&reply, "<b>%d</b> ➠ %s — %d\n", i+1, html.EscapeString(name), entry.Scrobbles)
	}
	fmt.Fprintf(&reply, "\nВсего скробблов: %d", ranking.Total)
	return reply.String()
}
