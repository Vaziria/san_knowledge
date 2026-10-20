# Payment reminders

Build the [payment reminders](../../docs/specs/payment-reminders.md) spec.

Taken: 2026-09-05 14:40

## Done

- The worker in `reminders/worker.go` sends three reminders per invoice and skips weekends.
- Checked: `make test` passed; a dry run over 1,000 seeded invoices sent no weekend reminder.
- Follow-up: duplicate sends were fixed after the [incident of 12 September](../../docs/ops/incident-2026-09-12.md).
