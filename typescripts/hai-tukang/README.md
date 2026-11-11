# Hai Tukang frontend

React + TypeScript + Chakra UI v3, built with Vite. Mobile-web first: check every screen at phone width (390px) before larger ones. The Vite dev server proxies the Connect services (`/haitukang.*`), the product photos (`/photos/`) and `/healthz` to the Go backend (`HAI_TUKANG_API`, default `http://127.0.0.1:8087`). It runs on port 5175, fixed (`strictPort`), since 5173 belongs to another project.

## Run locally

Start the database, then the backend and this dev server together, from `golang/apps/hai-tukang` (see its README):

```sh
docker compose up -d --wait
go run ./cmd/hai-tukang dev run
```

Open `http://127.0.0.1:5175`. Or run Vite alone (`npm ci`, `npm run dev`) with the backend served elsewhere: the proxy goes to `HAI_TUKANG_API`, by default `http://127.0.0.1:8087`.

## Language (i18n)

i18next with react-i18next. **Bahasa Indonesia is the default** and the fallback; English is the second language. The ID/EN switch in the header changes it, the choice is kept in localStorage (`hai-tukang-language`), and `<html lang>` follows it.

- All text lives in `src/i18n/id.ts` (the source) and `src/i18n/en.ts`. Never put text straight in a component: use `t('register.title')`.
- Keys are typed (`src/i18next.d.ts`): a misspelled key, or a language missing a key or having an extra one, fails `npm run typecheck`.
- API errors carry a stable reason (`phone_taken`, `invalid_phone`, …); `errorCode()` in `src/api.ts` reads it, and the page shows `errors.<reason>` in the reader's language (`network` when the backend cannot be reached). Keep the code, not the text, in state, so switching language translates a message already shown.
- To add a language: copy `en.ts`, translate it, and add it to `languages` and `resources` in `src/i18n/index.ts`.

## API client

`src/api.ts` makes typed Connect clients (`accounts`, `repairRequests`) from the code Buf generates into `src/gen/` from `golang/apps/hai-tukang/protos/`. Do not edit `src/gen/`; change the `.proto` and run `npm run gen`. The sign-in is an HttpOnly cookie the browser sends itself; when a call finds it gone, the app goes back to the sign-in screen.

## Screens

- Sign up / sign in (`AuthScreen.tsx`): HT-001, HT-002. Signing up takes the details, sends a code to the number on WhatsApp, then needs the code (HT-009); in development the code is in the backend's log. Every sign-up has an unticked "I agree to the Terms and Policies" box (`terms/TermsAgree.tsx`); no code is sent until it is ticked (HT-014).
- Google (`AuthScreen.tsx`, `google.ts`): when the backend has a Google client ID, Google's own "Continue with Google" button sits above the forms. A known Google account signs straight in; a new one fills in role, name (Google's) and phone with its WhatsApp code, and agrees to the terms: no password (HT-013).
- Terms and Policies (`terms/`): the page at `#/syarat`, linked from the sign-up forms and every screen's footer; it shows over the app, which keeps its state. The text is in `terms/content.ts` (both languages), its date in `terms/version.ts`, which must equal the backend's `TermsVersion`. An account that has not agreed to the current version sees only `TermsGate.tsx` until it agrees (HT-014).
- Sign in: an admin gets a WhatsApp code after the password and enters it, every time; customers and repairmen do not.
- Visits (`Visits.tsx`): on each response the customer can schedule a visit; proposals are answered (accept, another time, cancel) on the request card and the repairman's Jadwal tab. The customer sees the 4-digit arrival code; the repairman enters it on arrival. From 30 minutes after the window, the customer can report a no-show (HT-005 to HT-007).
- Penalty points (`Standing.tsx`): the reader's own points and suspension, as a banner (warning at 4 points, suspension) and a list on request. Reports and disputes are on the visit (`Visits.tsx`): the repairman reports a customer not at home, the reported side disputes within 24 hours, and cancelling less than 12 hours before a visit asks first (it costs a point) (HT-008, HT-010).
- Notifications (`PushPrompt.tsx`, `push.ts`, `public/sw.js`): signed-in customers and repairmen can turn on browser push for the device. A push shows a notification and makes open pages reload their visits. Signing out takes the device off the account. On iPhone, push works only once Hai Tukang is added to the Home Screen (`public/manifest.webmanifest`).
- Admin (`AdminHome.tsx`), four tabs: disputed reports (the points stand, or are removed), closed accounts (reopen, or keep closed), find an account by phone to suspend it or lift a suspension, and the log of decisions. Every decision asks for a written reason (HT-011, HT-012).
- A customer's request card shows its newest visit, and below it the reports of earlier visits (`EarlierReports`), so a report can still be disputed and its outcome seen after a new visit is arranged.
- Customer (`CustomerHome.tsx`): find repairmen by kind of work and area and contact them on WhatsApp (HT-003); post a request and see who responded, with WhatsApp (HT-004).
- Repairman (`RepairmanHome.tsx`): profile (starts here until it is filled in), and incoming requests to respond to; after responding, the customer's WhatsApp (HT-003, HT-004).

## Theme

The brand (logo direction 07) is a Chakra system in `src/theme.ts`: `ink` (charcoal, primary actions: `colorPalette="ink"`), `cream` (background) and `ochre` (small accents, not body text). Font: Plus Jakarta Sans, bundled with the app (`@fontsource-variable`). Use these tokens rather than hex colours in components.

## Phone field

Indonesian numbers only for now. The field shows a fixed `+62` and the reader types the rest. `src/phone.ts` drops a typed or pasted `+62`, a leading `0`, spaces and dashes when the field is left and on submit, so `0812-3456-7890` and `+62 812 3456 7890` both become `81234567890`. It cleans the whole value at once, not each keystroke: per keystroke, a typed `+` was dropped before its `62` arrived, and `+62628…` was saved.

## Check

```sh
npm run typecheck
npm run build
```
