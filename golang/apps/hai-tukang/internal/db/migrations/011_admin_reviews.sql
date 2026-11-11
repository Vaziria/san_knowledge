-- +goose Up
-- Admin reviews (acceptance HT-011, HT-012): the outcome of a disputed
-- report, suspensions made or lifted by hand, reviewed closed accounts,
-- and the log of every admin decision with its reason.
ALTER TABLE visits ADD COLUMN IF NOT EXISTS dispute_outcome TEXT CHECK (dispute_outcome IN ('points_stand', 'points_removed'));
ALTER TABLE visits ADD COLUMN IF NOT EXISTS dispute_decided_at TIMESTAMPTZ;

-- kind: 'points' suspensions follow the 7 days / 30 days / closed steps and
-- are numbered; 'admin' ones are made by hand, unnumbered.
ALTER TABLE suspensions ADD COLUMN IF NOT EXISTS kind TEXT NOT NULL DEFAULT 'points' CHECK (kind IN ('points', 'admin'));
ALTER TABLE suspensions ADD COLUMN IF NOT EXISTS reviewed_at TIMESTAMPTZ;
ALTER TABLE suspensions ALTER COLUMN number DROP NOT NULL;
ALTER TABLE suspensions DROP CONSTRAINT IF EXISTS suspensions_number_by_kind;
ALTER TABLE suspensions ADD CONSTRAINT suspensions_number_by_kind CHECK ((kind = 'points') = (number IS NOT NULL));

CREATE TABLE IF NOT EXISTS admin_decisions (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    admin_id BIGINT NOT NULL REFERENCES accounts (id),
    account_id BIGINT REFERENCES accounts (id) ON DELETE SET NULL,
    visit_id BIGINT REFERENCES visits (id) ON DELETE SET NULL,
    action TEXT NOT NULL CHECK (action IN ('points_stand', 'points_removed', 'reopen', 'keep_closed', 'suspend', 'lift')),
    reason TEXT NOT NULL CHECK (char_length(reason) BETWEEN 5 AND 500),
    created_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS admin_decisions_account ON admin_decisions (account_id, id DESC);

-- +goose Down
DROP TABLE IF EXISTS admin_decisions;
ALTER TABLE suspensions DROP CONSTRAINT IF EXISTS suspensions_number_by_kind;
-- Fails while suspensions made by hand exist.
ALTER TABLE suspensions ALTER COLUMN number SET NOT NULL;
ALTER TABLE suspensions DROP COLUMN IF EXISTS reviewed_at;
ALTER TABLE suspensions DROP COLUMN IF EXISTS kind;
ALTER TABLE visits DROP COLUMN IF EXISTS dispute_decided_at;
ALTER TABLE visits DROP COLUMN IF EXISTS dispute_outcome;
