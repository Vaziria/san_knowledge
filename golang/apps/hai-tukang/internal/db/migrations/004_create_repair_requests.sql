-- +goose Up
-- A customer's request for work, and repairmen's responses to it: one each.
CREATE TABLE IF NOT EXISTS repair_requests (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    customer_id BIGINT NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    service TEXT NOT NULL,
    area TEXT NOT NULL CHECK (char_length(area) BETWEEN 2 AND 80),
    description TEXT NOT NULL CHECK (char_length(description) BETWEEN 10 AND 1000),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS repair_requests_customer ON repair_requests (customer_id, created_at DESC);
CREATE INDEX IF NOT EXISTS repair_requests_service ON repair_requests (service, created_at DESC);

CREATE TABLE IF NOT EXISTS request_responses (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    request_id BIGINT NOT NULL REFERENCES repair_requests (id) ON DELETE CASCADE,
    repairman_id BIGINT NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    message TEXT NOT NULL CHECK (char_length(message) BETWEEN 2 AND 500),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (request_id, repairman_id)
);

-- +goose Down
DROP TABLE IF EXISTS request_responses;
DROP TABLE IF EXISTS repair_requests;
