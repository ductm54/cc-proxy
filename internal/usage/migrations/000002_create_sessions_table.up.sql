CREATE TABLE IF NOT EXISTS sessions (
    token      String,
    email      String,
    expires_at DateTime
) ENGINE = MergeTree()
ORDER BY (token)
TTL expires_at + INTERVAL 1 DAY;