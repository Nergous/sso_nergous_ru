CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    pass_hash BLOB NOT NULL,
    steam_url TEXT NOT NULL,
    path_to_photo TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_email ON users (email);

CREATE TABLE IF NOT EXISTS apps (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    secret TEXT NOT NULL UNIQUE
)