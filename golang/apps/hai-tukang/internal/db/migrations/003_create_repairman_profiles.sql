-- +goose Up
-- What a repairman does and where: what customers search on.
CREATE TABLE IF NOT EXISTS repairman_profiles (
    account_id BIGINT PRIMARY KEY REFERENCES accounts (id) ON DELETE CASCADE,
    services TEXT[] NOT NULL CHECK (cardinality(services) > 0),
    area TEXT NOT NULL CHECK (char_length(area) BETWEEN 2 AND 80),
    about TEXT NOT NULL DEFAULT '' CHECK (char_length(about) <= 500),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS repairman_profiles;
