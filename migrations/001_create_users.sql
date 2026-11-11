CREATE TABLE IF NOT EXISTS users (
    telegram_user_id INTEGER PRIMARY KEY CHECK (telegram_user_id > 0),
    telegram_username TEXT NOT NULL,
    telegram_display_name TEXT NOT NULL,
    lastfm_username TEXT NOT NULL CHECK (length(lastfm_username) > 0),
    lastfm_username_key TEXT NOT NULL UNIQUE CHECK (length(lastfm_username_key) > 0),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch())
);
