-- +goose Up
-- Browser push: each device that turned notifications on, and the
-- server's VAPID key pair (one row), which those subscriptions are tied to.
CREATE TABLE IF NOT EXISTS push_subscriptions (
    endpoint TEXT PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    p256dh TEXT NOT NULL,
    auth TEXT NOT NULL,
    language TEXT NOT NULL CHECK (language IN ('id', 'en')),
    time_zone TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS push_subscriptions_account ON push_subscriptions (account_id);

CREATE TABLE IF NOT EXISTS push_keys (
    id SMALLINT PRIMARY KEY CHECK (id = 1),
    public_key TEXT NOT NULL,
    private_key TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS push_keys;
DROP TABLE IF EXISTS push_subscriptions;
