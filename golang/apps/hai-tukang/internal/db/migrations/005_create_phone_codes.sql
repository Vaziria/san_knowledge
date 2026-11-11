-- +goose Up
-- One-time codes sent to a phone number, to prove it is the reader's
-- (acceptance HT-009). Only a hash of each code is kept; the newest code
-- for a number and purpose is the only one that works.
CREATE TABLE IF NOT EXISTS phone_codes (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    phone TEXT NOT NULL,
    purpose TEXT NOT NULL,
    code_hash BYTEA NOT NULL,
    attempts INT NOT NULL DEFAULT 0,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS phone_codes_phone ON phone_codes (phone, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS phone_codes;
