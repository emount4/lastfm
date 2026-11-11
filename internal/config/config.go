package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port                   uint16
	Telegram               Telegram
	LastFMAPIKey           string
	OpenRouterAPIKey       string
	OpenRouterFreeFallback bool
	Turso                  Turso
}

type Telegram struct {
	BotToken      string
	WebhookSecret string
	GroupID       int64
}

type Turso struct {
	DatabaseURL string
	AuthToken   string
}

func Load() (Config, error) {
	var problems []error
	required := func(name string) string {
		value := strings.TrimSpace(os.Getenv(name))
		if value == "" {
			problems = append(problems, fmt.Errorf("%s is required", name))
		}
		return value
	}

	cfg := Config{
		Port: 8080,
		Telegram: Telegram{
			BotToken:      required("TELEGRAM_BOT_TOKEN"),
			WebhookSecret: required("TELEGRAM_WEBHOOK_SECRET"),
		},
		LastFMAPIKey:     required("LASTFM_API_KEY"),
		OpenRouterAPIKey: strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY")),
		Turso: Turso{
			DatabaseURL: required("TURSO_DATABASE_URL"),
			AuthToken:   required("TURSO_AUTH_TOKEN"),
		},
	}

	if rawPort := strings.TrimSpace(os.Getenv("PORT")); rawPort != "" {
		port, err := strconv.ParseUint(rawPort, 10, 16)
		if err != nil || port == 0 {
			problems = append(problems, errors.New("PORT must be an integer from 1 to 65535"))
		} else {
			cfg.Port = uint16(port)
		}
	}
	if rawFallback := strings.TrimSpace(os.Getenv("OPENROUTER_FREE_FALLBACK")); rawFallback != "" {
		enabled, err := strconv.ParseBool(rawFallback)
		if err != nil {
			problems = append(problems, fmt.Errorf("parse OPENROUTER_FREE_FALLBACK: %w", err))
		} else {
			cfg.OpenRouterFreeFallback = enabled
		}
	}

	if rawID := required("TELEGRAM_GROUP_ID"); rawID != "" {
		id, err := strconv.ParseInt(rawID, 10, 64)
		if err != nil || id >= 0 {
			problems = append(problems, errors.New("TELEGRAM_GROUP_ID must be a negative chat ID"))
		} else {
			cfg.Telegram.GroupID = id
		}
	}

	if secret := cfg.Telegram.WebhookSecret; secret != "" {
		const allowed = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_-"
		if len(secret) > 256 || strings.Trim(secret, allowed) != "" {
			problems = append(problems, errors.New("TELEGRAM_WEBHOOK_SECRET must contain 1 to 256 letters, digits, underscores or hyphens"))
		}
	}

	if rawURL := cfg.Turso.DatabaseURL; rawURL != "" {
		dbURL, err := url.Parse(rawURL)
		if err != nil || dbURL.Hostname() == "" || (dbURL.Scheme != "libsql" && dbURL.Scheme != "https") ||
			dbURL.User != nil || strings.ContainsAny(rawURL, "?#") {
			problems = append(problems, errors.New("TURSO_DATABASE_URL must be a libsql:// or https:// URL without credentials, query or fragment"))
		}
	}

	if err := errors.Join(problems...); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
