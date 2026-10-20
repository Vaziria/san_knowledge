# Spec: invoice PDF

## Layout

A4 and Letter. Logo top left, client block top right, then line items, totals and the payment link.

## Numbering

INV-2026-0001 style, per freelancer, never reused, even after a delete.

## Payment link

Each PDF carries a Stripe Checkout link and a QR code ([decision 0002](../decisions/0002-stripe-for-payments.md)).
