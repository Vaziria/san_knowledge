-- +goose Up
CREATE TABLE IF NOT EXISTS accounts (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    role TEXT NOT NULL CHECK (role IN ('customer', 'repairman')),
    full_name TEXT NOT NULL CHECK (char_length(full_name) BETWEEN 2 AND 80),
    phone TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS accounts;
