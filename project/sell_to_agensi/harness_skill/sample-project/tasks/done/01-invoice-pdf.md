# Invoice PDF with a payment link

Build the PDF described in [the spec](../../docs/specs/invoice-pdf.md).

## Do

1. A4 and Letter layouts.
2. A Stripe Checkout link and a QR code on every invoice.

## Check

- `make test` passes.
- A test invoice opens in Acrobat and Preview.

Taken: 2026-09-02 10:14

## Done

- `api/invoice/pdf.go` renders both sizes; the QR code opens the Checkout session.
- Checked: `make test` passed (112 tests, 9 new). INV-2026-0001 opens in Acrobat and Preview, and the QR code scans on iOS and Android.
- Left for the user: logos are cropped to a square; [decision 0002](../../docs/decisions/0002-stripe-for-payments.md) doesn't say whether wide logos are allowed.
