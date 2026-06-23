package messaging

const schema = `
CREATE TABLE IF NOT EXISTS messages (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    team        TEXT NOT NULL,
    from_agent  TEXT NOT NULL,
    to_agent    TEXT NOT NULL,
    body        TEXT NOT NULL,
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    read_at     TEXT
);
CREATE INDEX IF NOT EXISTS idx_unread ON messages(team, to_agent, read_at) WHERE read_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_history ON messages(team, created_at DESC);
`
