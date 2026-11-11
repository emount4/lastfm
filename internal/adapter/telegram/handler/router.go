package handler

import (
	"context"

	"github.com/emount4/lastfm/internal/adapter/telegram/webhook"
)

type Router struct {
	private webhook.MessageHandler
	group   webhook.MessageHandler
}

func NewRouter(private, group webhook.MessageHandler) *Router {
	return &Router{private: private, group: group}
}

func (h *Router) HandleMessage(ctx context.Context, message webhook.IncomingMessage) error {
	if message.Private {
		return h.private.HandleMessage(ctx, message)
	}
	return h.group.HandleMessage(ctx, message)
}
