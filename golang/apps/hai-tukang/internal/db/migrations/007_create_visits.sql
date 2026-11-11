-- +goose Up
-- A repairman's visit for a repair request (acceptance HT-005 to HT-007).
-- A request has at most one active (proposed or confirmed) visit.
CREATE TABLE IF NOT EXISTS visits (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    request_id BIGINT NOT NULL REFERENCES repair_requests (id) ON DELETE CASCADE,
    repairman_id BIGINT NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    window_start TIMESTAMPTZ NOT NULL,
    window_end TIMESTAMPTZ NOT NULL CHECK (window_end > window_start),
    proposed_by TEXT NOT NULL CHECK (proposed_by IN ('customer', 'repairman')),
    status TEXT NOT NULL CHECK (status IN ('proposed', 'confirmed', 'arrived', 'no_show', 'cancelled')),
    -- arrival_code is shown to the customer only, and given to the repairman
    -- in person on arrival.
    arrival_code TEXT,
    arrival_attempts INT NOT NULL DEFAULT 0,
    confirmed_at TIMESTAMPTZ,
    arrived_at TIMESTAMPTZ,
    no_show_at TIMESTAMPTZ,
    cancelled_at TIMESTAMPTZ,
    cancelled_by TEXT CHECK (cancelled_by IN ('customer', 'repairman')),
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS visits_one_active_per_request ON visits (request_id) WHERE status IN ('proposed', 'confirmed');
CREATE INDEX IF NOT EXISTS visits_repairman ON visits (repairman_id, window_start DESC);
CREATE INDEX IF NOT EXISTS visits_request ON visits (request_id);

-- +goose Down
DROP TABLE IF EXISTS visits;
