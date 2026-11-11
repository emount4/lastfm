package handler

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/emount4/lastfm/internal/adapter/telegram"
	"github.com/emount4/lastfm/internal/adapter/telegram/webhook"
	"github.com/emount4/lastfm/internal/domain"
)

type MusicSearcher interface {
	Search(context.Context, domain.MusicKind, string) ([]domain.MusicTarget, error)
	FindVariants(context.Context, domain.MusicTarget) ([]domain.MusicTarget, error)
}

type ChoiceStore interface {
	SaveChoices(ctx context.Context, id string, chatID, threadID int64, options []domain.MusicTarget) error
	TakeChoice(ctx context.Context, id string, chatID int64, index int) (domain.MusicTarget, int64, bool, error)
}

func (h *Group) rankByTitle(ctx context.Context, message webhook.IncomingMessage, kind domain.MusicKind, args, usage string) error {
	artist, title := splitArtistAndTitle(args)
	if artist != "" && title != "" {
		return h.rankWithVariants(ctx, message, domain.MusicTarget{Kind: kind, Artist: artist, Title: title})
	}

	query := strings.TrimSpace(strings.Trim(args, "\""))
	if query == "" {
		return h.sendRanking(ctx, message, domain.MusicTarget{Kind: kind}, usage)
	}
	options, err := h.search.Search(ctx, kind, query)
	if err != nil {
		return fmt.Errorf("find %s for Telegram: %w", kind, err)
	}
	switch len(options) {
	case 0:
		if err := h.bot.SendText(ctx, message.ChatID, message.ThreadID, "На Last.fm ничего не найдено. Проверь название или укажи артиста."); err != nil {
			return fmt.Errorf("send empty music search result: %w", err)
		}
		return nil
	case 1:
		return h.sendRanking(ctx, message, options[0], usage)
	}
	return h.sendChoices(ctx, message, options)
}

func (h *Group) rankWithVariants(ctx context.Context, message webhook.IncomingMessage, target domain.MusicTarget) error {
	ranking, err := h.rankings.Rank(ctx, target)
	if err != nil {
		return fmt.Errorf("rank %s for Telegram: %w", target.Kind, err)
	}
	if len(ranking.Entries) != 0 {
		return h.sendRankingResult(ctx, message, target, ranking)
	}
	options, err := h.search.FindVariants(ctx, target)
	if err != nil {
		return fmt.Errorf("find music variants for Telegram: %w", err)
	}
	switch len(options) {
	case 0:
		return h.sendRankingResult(ctx, message, target, ranking)
	case 1:
		return h.sendRanking(ctx, message, options[0], "")
	default:
		return h.sendChoices(ctx, message, options)
	}
}

func (h *Group) sendChoices(ctx context.Context, message webhook.IncomingMessage, options []domain.MusicTarget) error {
	id := rand.Text()
	if err := h.choices.SaveChoices(ctx, id, message.ChatID, message.ThreadID, options); err != nil {
		return fmt.Errorf("prepare music selection: %w", err)
	}
	buttons := make([]telegram.Button, 0, len(options))
	for index, option := range options {
		label := option.Artist
		if option.Title != "" {
			label += " — " + option.Title
		}
		buttons = append(buttons, telegram.Button{
			Text: label,
			Data: "wk:" + id + ":" + strconv.Itoa(index),
		})
	}
	if err := h.bot.SendChoices(ctx, message.ChatID, message.ThreadID, "Нашёл несколько совпадений. Выбери нужный вариант:", buttons); err != nil {
		return fmt.Errorf("send music choices: %w", err)
	}
	return nil
}

func (h *Group) HandleCallback(ctx context.Context, callback webhook.IncomingCallback) error {
	data, ok := strings.CutPrefix(callback.Data, "wk:")
	if !ok {
		return nil
	}
	id, option, _ := strings.Cut(data, ":")
	index, err := strconv.Atoi(option)
	if err != nil || id == "" || index < 0 {
		return h.bot.AnswerCallback(ctx, callback.ID, "Этот вариант недоступен.")
	}

	target, threadID, found, err := h.choices.TakeChoice(ctx, id, callback.ChatID, index)
	if err != nil {
		return fmt.Errorf("select music option: %w", err)
	}
	if !found {
		return h.bot.AnswerCallback(ctx, callback.ID, "Выбор уже сделан или срок истёк.")
	}

	answerErr := h.bot.AnswerCallback(ctx, callback.ID, "")
	deleteErr := h.bot.DeleteMessage(ctx, callback.ChatID, callback.MessageID)
	rankingErr := h.sendRanking(ctx, webhook.IncomingMessage{ChatID: callback.ChatID, ThreadID: threadID}, target, "")
	if rankingErr != nil {
		if err := h.bot.SendText(ctx, callback.ChatID, threadID, "Не удалось получить рейтинг. Попробуй повторить команду."); err != nil {
			rankingErr = errors.Join(rankingErr, fmt.Errorf("send ranking failure notice: %w", err))
		}
	}
	return errors.Join(answerErr, deleteErr, rankingErr)
}
