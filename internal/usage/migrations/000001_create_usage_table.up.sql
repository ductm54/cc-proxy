CREATE TABLE IF NOT EXISTS usage (
    email                  String,
    model                  String,
    input_tokens           UInt32,
    output_tokens          UInt32,
    cache_creation_tokens  UInt32,
    cache_read_tokens      UInt32,
    cost_usd               Float64,
    created_at             DateTime DEFAULT now()
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(created_at)
ORDER BY (email, created_at)
TTL created_at + INTERVAL 365 DAY;