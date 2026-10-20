# Agent instructions

Invoicing app for freelancers. Go API, Postgres, React web app.

## Commands

    make dev     # API on :8080, web app on :3000
    make test    # all tests

## Knowledge

- Every markdown file is in the knowledge graph. Before answering a question about the product or a past decision, call knowledge_explain and cite path:line.
- After editing markdown, sync once. Ask before writing summaries.

## Tasks

tasks/ is a queue shared by every session: take the next file by moving it into tasks/doing/, finish with a Done section in tasks/done/.

## Rules

- **Never send reminder emails on weekends** (the user, 2026-09-03: "nobody wants a payment nag on Sunday"). See [payment reminders](docs/specs/payment-reminders.md#schedule).
- **Prices are in USD and IDR only for now** (the user, 2026-09-20). See [pricing](docs/product/pricing.md#plans).
