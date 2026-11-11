-- +goose Up
-- Admins (Hai Tukang staff) review disputes and closed accounts. They are
-- made from the server's command line, never by signing up.
ALTER TABLE accounts DROP CONSTRAINT IF EXISTS accounts_role_check;
ALTER TABLE accounts ADD CONSTRAINT accounts_role_check CHECK (role IN ('customer', 'repairman', 'admin'));

-- +goose Down
-- Fails while admin accounts exist.
ALTER TABLE accounts DROP CONSTRAINT IF EXISTS accounts_role_check;
ALTER TABLE accounts ADD CONSTRAINT accounts_role_check CHECK (role IN ('customer', 'repairman'));
