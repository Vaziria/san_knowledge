# Deploy

## Environments

Staging on every merge to main; production from tagged releases.

## Rollback

`make rollback` returns to the previous tag. Migrations only go forward ([decision 0001](../decisions/0001-postgres-over-sqlite.md)).
