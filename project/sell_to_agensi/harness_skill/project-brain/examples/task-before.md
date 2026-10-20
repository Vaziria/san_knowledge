# Pricing page shows the Ramadan discount

The Ramadan offer starts with the launch campaign, and the pricing page still shows last year's numbers. Visitors should see the discount on the plans it applies to.

## Do
1. On the pricing page, show "Ramadan: 25% off" on the annual Basic, Pro and Team plans.
2. Monthly plans show no discount.
3. Take the numbers from `docs/pricing.md`, not from the old page.

## Check
- `npm test` passes.
- The page at `/pricing` shows the discount on the three annual plans only.

After: 02-pricing-data.md
Commit: yes, "pricing page: Ramadan discount on annual plans"
