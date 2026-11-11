package handler

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/emount4/lastfm/internal/adapter/telegram/webhook"
)

type GroupBot interface {
	Messenger
	BotUsername(context.Context) (string, error)
}

type Group struct {
	bot GroupBot
}

func NewGroup(bot GroupBot) *Group {
	return &Group{bot: bot}
}

func (h *Group) HandleMessage(ctx context.Context, message webhook.IncomingMessage) error {
	if message.Private || !strings.EqualFold(strings.TrimSpace(message.Text), "info") {
		return nil
	}

	const help = "Команды бота\ninfo — показать справку.\n\n"
	reply := help + "Для регистрации открой личный чат с ботом и отправь свой ник Last.fm."
	if username, err := h.bot.BotUsername(ctx); err == nil && username != "" {
		reply = help + "Для регистрации отправь свой ник Last.fm в личный чат с ботом:\nhttps://t.me/" + url.PathEscape(username)
	}

	if err := h.bot.SendText(ctx, message.ChatID, message.ThreadID, reply); err != nil {
		return fmt.Errorf("send group help: %w", err)
	}
	return nil
}
