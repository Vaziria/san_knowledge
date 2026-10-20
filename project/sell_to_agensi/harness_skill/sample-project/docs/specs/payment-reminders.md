# Spec: payment reminders

## Schedule

Reminders go out 3 days before the due date, on the due date, and 7 days after. Never on Saturday or Sunday: a weekend reminder moves to Monday 9:00 in the client's time zone.

## Tone

Three templates: friendly, firm and final. The freelancer can edit each one.

## Stopping

Reminders stop when the invoice is paid (Stripe webhook) or marked paid by hand. Each send carries an idempotency key since the [duplicate reminders incident](../ops/incident-2026-09-12.md).
