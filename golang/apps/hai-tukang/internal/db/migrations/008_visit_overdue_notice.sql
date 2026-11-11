-- +goose Up
-- When the customer was told that a confirmed visit's window passed
-- without an arrival (once per visit).
ALTER TABLE visits ADD COLUMN IF NOT EXISTS overdue_notified_at TIMESTAMPTZ;
CREATE INDEX IF NOT EXISTS visits_overdue ON visits (window_end) WHERE status = 'confirmed' AND overdue_notified_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS visits_overdue;
ALTER TABLE visits DROP COLUMN IF EXISTS overdue_notified_at;
