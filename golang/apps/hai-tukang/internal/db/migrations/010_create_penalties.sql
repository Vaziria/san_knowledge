-- +goose Up
-- Penalty points and suspensions (acceptance HT-008, HT-010), and the
-- reports and disputes on visits that give points.
ALTER TABLE visits DROP CONSTRAINT IF EXISTS visits_status_check;
ALTER TABLE visits ADD CONSTRAINT visits_status_check
    CHECK (status IN ('proposed', 'confirmed', 'arrived', 'no_show', 'customer_absent', 'cancelled'));
ALTER TABLE visits ADD COLUMN IF NOT EXISTS absent_at TIMESTAMPTZ;
ALTER TABLE visits ADD COLUMN IF NOT EXISTS disputed_at TIMESTAMPTZ;
ALTER TABLE visits ADD COLUMN IF NOT EXISTS dispute_note TEXT NOT NULL DEFAULT '' CHECK (char_length(dispute_note) <= 500);

-- A suspension: number 1 and 2 end (7 and 30 days); 3 and later close the
-- account until an admin reviews it.
CREATE TABLE IF NOT EXISTS suspensions (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    number INT NOT NULL CHECK (number > 0),
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ,
    closed BOOLEAN NOT NULL DEFAULT false,
    CHECK (closed = (ends_at IS NULL)),
    UNIQUE (account_id, number)
);

-- A point event. It counts from counts_from (a report: after the 24-hour
-- dispute window) until expires_at, unless disputed, removed, or used up
-- by the suspension it led to.
CREATE TABLE IF NOT EXISTS penalty_points (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    visit_id BIGINT REFERENCES visits (id) ON DELETE SET NULL,
    reason TEXT NOT NULL CHECK (reason IN ('no_show', 'customer_absent', 'late_cancel', 'late_arrival')),
    points INT NOT NULL CHECK (points > 0),
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disputed', 'removed')),
    created_at TIMESTAMPTZ NOT NULL,
    counts_from TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    suspension_id BIGINT REFERENCES suspensions (id),
    evaluated_at TIMESTAMPTZ,
    UNIQUE (visit_id, reason)
);
CREATE INDEX IF NOT EXISTS penalty_points_account ON penalty_points (account_id, created_at DESC);
CREATE INDEX IF NOT EXISTS penalty_points_due ON penalty_points (counts_from) WHERE evaluated_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS penalty_points;
DROP TABLE IF EXISTS suspensions;
ALTER TABLE visits DROP COLUMN IF EXISTS dispute_note;
ALTER TABLE visits DROP COLUMN IF EXISTS disputed_at;
ALTER TABLE visits DROP COLUMN IF EXISTS absent_at;
-- Fails while visits are customer_absent.
ALTER TABLE visits DROP CONSTRAINT IF EXISTS visits_status_check;
ALTER TABLE visits ADD CONSTRAINT visits_status_check
    CHECK (status IN ('proposed', 'confirmed', 'arrived', 'no_show', 'cancelled'));
