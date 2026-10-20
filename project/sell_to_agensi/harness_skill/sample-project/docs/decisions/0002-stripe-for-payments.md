# 0002 Stripe for payments

Date: 2026-08-10. Status: accepted.

## Context

Interviewed freelancers want card and bank transfer on every invoice ([user interviews](../research/user-interviews.md#what-they-would-pay-for)).

## Decision

A Stripe Checkout link on every invoice; webhooks mark invoices paid. See the [invoice PDF](../specs/invoice-pdf.md#payment-link).

## Consequences

Card fees (2.9% + 30 cents) are shown to the freelancer before sending.
