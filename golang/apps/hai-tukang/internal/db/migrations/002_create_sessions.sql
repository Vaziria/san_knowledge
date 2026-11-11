-- +goose Up
-- A signed-in device. Only a hash of the token is kept, so a leaked
-- database cannot be used to sign in.
CREATE TABLE IF NOT EXISTS sessions (
    token_hash BYTEA PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS sessions_account_id ON sessions (account_id);

-- +goose Down
DROP TABLE IF EXISTS sessions;
