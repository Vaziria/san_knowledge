# 0001 Postgres over SQLite

Date: 2026-08-02. Status: accepted.

## Context

The API and the reminder worker both write invoices. SQLite locked up under two writers in load tests (p95 4.2 s).

## Decision

Postgres 16 on the managed plan. Migrations live in `db/migrations` and only go forward.

## Consequences

One more service to run locally: `make dev` starts it in Docker. Rollbacks follow [deploy](../ops/deploy.md#rollback).
