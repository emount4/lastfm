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
	SendPhoto(ctx context.Context, chatID, threadID int64, photo []byte, caption string) error
}

type Group struct {
	bot      GroupBot
	rankings Ranker
	search   MusicSearcher
	choices  ChoiceStore
	collage  CollageBuilder
	top      TopRanker
	vibe     VibeDescriber
}

type Ranker interface {
	Rank(context.Context, domain.MusicTarget) (who.Ranking, error)
}

func NewGroup(bot GroupBot, rankings Ranker, search MusicSearcher, choices ChoiceStore, collage CollageBuilder, top TopRanker, vibe VibeDescriber) *Group {
	return &Group{bot: bot, rankings: rankings, search: search, choices: choices, collage: collage, top: top, vibe: vibe}
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
		target := domain.MusicTarget{Kind: domain.Artist, Artist: args}
		if strings.ContainsAny(args, "еЕ") {
			return h.rankWithVariants(ctx, message, target)
		}
		return h.sendRanking(ctx, message, target, "Укажи артиста. Например: wka Radiohead")
	case "wktrk":
		return h.rankByTitle(ctx, message, domain.Track, args, "Формат: wktrk название или wktrk артист - трек")
	case "wkalb":
		return h.rankByTitle(ctx, message, domain.Album, args, "Формат: wkalb название или wkalb артист - альбом")
	case "topart":
		return h.sendTop(ctx, message, "topart", args, domain.Artist, "артистов")
	case "toptrk":
		return h.sendTop(ctx, message, "toptrk", args, domain.Track, "треков")
	case "topalb":
		return h.sendTop(ctx, message, "topalb", args, domain.Album, "альбомов")
	case "topusers":
		return h.sendTopUsers(ctx, message, args)
	case "скроббовайб":
		return h.sendVibe(ctx, message, args)
	}
	return nil
}

func (h *Group) sendHelp(ctx context.Context, message webhook.IncomingMessage) error {
	const help = `<b>Last.fm в этой группе</b>
Пиши команды обычным сообщением без слеша, в любом регистре. <code>info</code> открывает эту справку.

<b>Сейчас играет</b>
<code>playing</code> — коллаж с треками, которые участники слушают прямо сейчас.

<b>Кто сколько слушал</b>
<code>wka Radiohead</code> — скробблы артиста по участникам.
<code>wktrk Radiohead - Creep</code> — скробблы трека.
<code>wkalb Radiohead - OK Computer</code> — скробблы альбома.
Под списком бот покажет общую сумму. Можно указать только название, например <code>wktrk Creep</code>. Если есть несколько вариантов, выбери нужный кнопкой.

<b>Топ 25 группы</b>
<code>topart</code> — артисты
<code>toptrk</code> — треки
<code>topalb</code> — альбомы
<code>topusers</code> — кто больше всего слушал по числу скробблов
Без периода считаем за всё время.
Примеры периодов: <code>10d</code> — 10 дней, <code>2w</code> — 2 недели, <code>3m</code> — 3 месяца, <code>2y</code> — 2 года. Например, <code>topalb 3m</code>.

<b>Скроббовайб</b>
<code>скроббовайб</code> — шуточное описание по твоим прослушиваниям сегодня.
<code>скроббовайб 3d</code> — за три календарных дня, включая сегодня. Дни считаем по Москве.

<b>Регистрация</b>
Отправь свой ник Last.fm в личный чат с ботом. Чтобы убрать свою привязку и выйти из статистики группы, отправь там <code>unlink</code>.`
	reply := help
	if username, err := h.bot.BotUsername(ctx); err == nil && username != "" {
		reply += "\n<a href=\"https://t.me/" + url.PathEscape(username) + "\">Открыть личный чат</a>"
	}

	if err := h.bot.SendHTML(ctx, message.ChatID, message.ThreadID, reply); err != nil {
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
		name := entry.Participant.TelegramName()
		fmt.Fprintf(&reply, "<b>%d</b> ➠ %s\n", entry.Scrobbles, html.EscapeString(name))
	}
	fmt.Fprintf(&reply, "\nВсего скробблов: %d", ranking.Total)
	return reply.String()
}
