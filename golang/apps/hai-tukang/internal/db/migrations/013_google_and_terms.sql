-- +goose Up
-- Sign-up with Google (acceptance HT-013): an account has a password, or a
-- Google account (its ID, google_subject), or both. And the Terms and
-- Policies (HT-014): the version an account agreed to, and when.
ALTER TABLE accounts ALTER COLUMN password_hash DROP NOT NULL;
ALTER TABLE accounts ADD COLUMN google_subject TEXT UNIQUE;
ALTER TABLE accounts ADD COLUMN google_email TEXT;
ALTER TABLE accounts ADD CONSTRAINT accounts_can_sign_in CHECK (password_hash IS NOT NULL OR google_subject IS NOT NULL);
ALTER TABLE accounts ADD COLUMN terms_version TEXT;
ALTER TABLE accounts ADD COLUMN terms_accepted_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE accounts DROP COLUMN terms_accepted_at;
ALTER TABLE accounts DROP COLUMN terms_version;
ALTER TABLE accounts DROP CONSTRAINT IF EXISTS accounts_can_sign_in;
ALTER TABLE accounts DROP COLUMN google_email;
ALTER TABLE accounts DROP COLUMN google_subject;
-- Fails while accounts without a password (Google ones) exist.
ALTER TABLE accounts ALTER COLUMN password_hash SET NOT NULL;
