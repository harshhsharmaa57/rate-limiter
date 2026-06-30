CREATE TABLE IF NOT EXISTS rules (
    id           VARCHAR(64)  PRIMARY KEY,
    limit_count  BIGINT       NOT NULL,
    window_ms    BIGINT       NOT NULL,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

INSERT INTO rules (id, limit_count, window_ms) VALUES
    ('free-tier',  10,     60000),
    ('pro-tier',   1000,   60000),
    ('admin',      999999, 60000)
ON CONFLICT (id) DO NOTHING;