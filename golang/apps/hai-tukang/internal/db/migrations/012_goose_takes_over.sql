-- +goose Up
-- Migrations were applied by a runner of our own, recorded in
-- schema_migrations; goose (goose_db_version) applies them now. Every
-- migration before this one can run again on a database that has it, so
-- goose ran them once more on such a database, changing nothing; the old
-- record goes.
DROP TABLE IF EXISTS schema_migrations;

-- +goose Down
SELECT 1;
