CREATE TABLE IF NOT EXISTS music_choices (
    id TEXT PRIMARY KEY,
    chat_id INTEGER NOT NULL,
    thread_id INTEGER NOT NULL,
    options TEXT NOT NULL,
    expires_at INTEGER NOT NULL
);
