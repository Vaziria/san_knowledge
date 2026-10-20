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

Taken: 2026-10-19 14:05

## Done

- `src/pages/pricing.tsx` reads the discount from `src/data/pricing.ts` (added in 02-pricing-data) and shows a "Ramadan: 25% off" badge on annual plans; monthly cards are unchanged.
- The old hard-coded 20% in `src/components/PlanCard.tsx` is gone.
- Checked: `npm test` passed (48 tests, 2 new for the badge). Opened `/pricing` in the dev server: the badge shows on Basic, Pro and Team annual, not on monthly. Committed as "pricing page: Ramadan discount on annual plans".
- Left for the user: the badge text is English; `docs/pricing.md` doesn't say whether the Indonesian page should read "Diskon Ramadan 25%". Suggested as a follow-up task, not created.
