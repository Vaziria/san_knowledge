# Hai Tukang — business acceptance

**Status:** Draft — HT-001 to HT-004 built and passing the automated acceptance test; business rules below are proposed defaults awaiting approval  
**Business approver:** TBD  
**Last reviewed:** TBD

## Business purpose

Hai Tukang connects people who need home repairs or similar services with repairmen who can do the work. The intended outcome is to help both sides find one another and make contact.

## Users and stakeholders

- People who need home repairs or similar services (customers): specific needs and workflow TBD.
- Repairmen / service providers: how they join, describe their services, and receive requests is TBD.
- Business owner / approver: TBD.
- Admins (Hai Tukang staff): review disputed reports and closed accounts, and can suspend or lift a suspension.
- Partner stores (toko bangunan, toko listrik, ...): sell repair materials through Hai Tukang (agreed; details proposed, see "Marketplace").

## Scope

### Included

- Connecting people looking for home repair or similar services with repairmen.
- The first version supports both discovery flows: customers can post repair requests for repairmen to respond to, and browse repairmen profiles to contact them.
- A marketplace for repair materials sold by partner stores, bought on their own or with a repair (agreed: the owner's idea, option 2 "Partner stores"; not built).

### Excluded

- TBD.

## Platform priority

- Initial development is mobile-web-first. Design and validate the core customer and repairman flows at phone sizes before expanding the experience to larger screens.
- The app is internationalised (i18n). **Bahasa Indonesia is the default language**; English is available through a language switch. Every user-facing text, including error messages, must exist in both.
- Phone numbers are Indonesian (+62) for now.
- Notifications are browser push (Web Push). On iPhone this needs the site added to the Home Screen (iOS 16.4+), so the app also shows the current state whenever it is opened; nothing may depend on a push arriving.

## Technical direction

- Backend: Go.
- Frontend: React + TypeScript with Chakra UI.
- Primary database: PostgreSQL.
- API: Connect RPC, defined with Buf (`golang/apps/hai-tukang/protos/`), like San Tower.
- Hosting (agreed 2026-11-11): a small VPS in Indonesia for the pilot, running Postgres, the app and Caddy (automatic HTTPS). The owner: "And it bigger, there is possible to b": built so it can move to a managed platform later. Settings come from the environment, product photos go through a storage interface (the VPS's disk now, S3-compatible storage such as Cloudflare R2 later), and the database is backed up with pg_dump, which a managed Postgres can restore.

## Business rules

Proposed defaults, built so the scenarios can be checked end to end; each awaits approval:

- Sign-in is phone number + password; registering also signs you in. One account per phone number, as either a customer or a repairman.
- Kinds of work: Listrik, Pipa & air, Atap & bocor, Cat, AC, Bangunan & renovasi, Kayu & furnitur, Lainnya.
- A repairman's profile has one or more kinds of work, a work area (free text, e.g. "Jakarta Selatan"), and an optional "about". Without a profile he cannot be found and cannot respond to requests.
- Location is free text, matched loosely ("jakarta" finds "Jakarta Selatan"). No maps or distance yet.
- A repair request has a kind of work, an area and a description (10–1000 characters). Each repairman can respond once, with a message (2–500 characters).
- Contact happens outside the app, on WhatsApp, with a prefilled message. A customer sees the number of each repairman shown or responding; a repairman sees a customer's number only after responding to their request.
- Repairmen are not called "verified": there is no verification of skills or identity. Once phone verification exists, the app may say the phone number is verified ("Nomor HP terverifikasi"), never that the repairman is.

### Phone verification (OTP)

- Signing up needs a one-time code sent to the phone number; without it no account is created. This comes before penalty points, so a suspended account cannot simply sign up again with an unverified number.
- Proposed: 6-digit code, valid 5 minutes, at most 5 wrong tries, a new code after 60 seconds, at most 5 codes per number per hour.
- Proposed: the same code flow lets a person reset a forgotten password.
- Channel: WhatsApp. **Provider agreed (2026-11-11): Twilio**, over Meta's official WhatsApp Business platform, from Hai Tukang's own WhatsApp sender (a number not on the WhatsApp app) with an approved authentication template. Proposed: Hai Tukang keeps making and checking the codes (the rules above) and Twilio only sends them, through its Messaging API; Twilio Verify, which makes and checks codes itself, stays an option. Until the account is ready, the app keeps the mock: a sender that only writes the code to the server log (the owner, 2026-11-11: "I don't have twillio account, can we mock first"; the Twilio sender is written once there is an account).

### Visits (first release: agreed time, arrival code, no-show report)

- The customer chooses one repairman who responded and proposes a visit window (date and time range). The repairman accepts it or proposes another window, which the customer accepts. A request has at most one confirmed visit at a time.
- Either side can cancel a confirmed visit; the request then reopens to the other repairmen who responded.
- The customer's screen shows a 4-digit arrival code for the visit. The repairman enters it when he arrives, which records the arrival and its time. The code is never shown to the repairman.
- If 30 minutes after the window ends there is no arrival, the customer can report "Tukang tidak datang". The visit is recorded as a no-show, and the request reopens to the other repairmen who responded.
- Browser push tells the repairman about a proposed window, the customer about an accepted one, and the customer when the window has passed without an arrival. Reminders before the visit and reconfirmation are later work.

### Penalty points (agreed)

Penalty points start only once phone verification (HT-009) is in place.

Details chosen while building (awaiting approval):

- A report's points (no-show, not at home) count only after its 24-hour dispute window; a disputed report's never count until an admin reviews it. Late cancellations and late arrivals count at once.
- Reaching 6 points starts a suspension and uses those points up: after it, the account starts again from 0. Suspensions are numbered over the account's whole life (7 days, 30 days, then closed).
- During a suspension the account cannot arrange new visits (propose, answer, accept); a repairman also cannot respond to requests and is hidden from search, a customer cannot post requests. Proposing to a suspended repairman says he cannot take visits now, not why. Confirmed visits go ahead (arrive, report, cancel).
- The reported side gets a push notification about the report, so the 24 hours are not missed. The other side sees the dispute and its note.
- Each account sees only its own points: reason, date, and state (pending, counting, disputed, used up, expired, removed).

Repairmen:

- No-show: 2 points. Cancelling a confirmed visit less than 12 hours before it: 1 point. Arriving more than 30 minutes after the window ends: 1 point.
- Points expire 90 days after the event. The repairman sees his own points, each with its reason and date; customers never see them.
- At 4 points the repairman is warned. At 6 points (three no-shows) he is suspended: hidden from search and unable to respond to requests or accept visits. First suspension 7 days, second 30 days, third: account closed pending admin review. Visits already confirmed are kept.

Customers (values proposed, mirroring the repairmen's):

- Not home for a confirmed visit (reported by the repairman during or up to 30 minutes after the window): 2 points. Cancelling a confirmed visit less than 12 hours before it: 1 point.
- The same expiry, privacy, warning and suspension steps as repairmen. A suspended customer cannot post requests or confirm visits; visits already confirmed are kept.

Disputes:

- Either side can answer a report against them within 24 hours ("Saya datang, pelanggan tidak ada" / "Saya ada di rumah"). A disputed report gives no points until an admin reviews it.

### Admin role

Built: the command that makes an admin, and admin sign-in with password and WhatsApp code, lasting 12 hours; the reviews below (HT-011, HT-012).

- A third kind of account, not offered at sign-up. Admins are created from the server's command line by whoever runs the server; the first is the business owner.
- Admins sign in on the same page, with password and a WhatsApp code every time.
- Admins see disputed reports with both sides' statements, the visit window and the arrival record, and decide: the points stand, or are removed. Both sides see the outcome.
- Admins review closed accounts: reopen (points reset) or keep closed. They can also suspend an account or lift a suspension.
- Every admin decision needs a written reason, and is recorded with the admin and the time. Admins never see passwords.

Details chosen while building the reviews (awaiting approval):

- A reason is 5 to 500 characters. The log shows each decision with the admin, the time, the reason and who it was about; the decision on a dispute shows on the visit to both sides ("the points stand" or "the points were removed"), the reason only in the log.
- Points that stand count at once, rather than 24 hours after the report.
- Reopening a closed account removes the points still counting. The suspension count stays: the next suspension closes it again.
- "Keep closed" marks the account reviewed and keeps it in the list, with the date and reason.
- A suspension by hand lasts 1 to 365 days and is not a step on the 7 days / 30 days / closed ladder. An admin cannot suspend another admin, and finds accounts by phone number.
- The dispute review shows the request, the window, when the visit was confirmed, wrong arrival codes, both statements and WhatsApp links to both sides.

### Sign-up with Google (agreed: customers and repairmen may register with Google)

Details proposed (awaiting approval):

- "Lanjutkan dengan Google" sits next to the phone-and-password forms, which stay: Google sign-in does not work inside some apps' built-in browsers (WhatsApp, Instagram links), where many people will open Hai Tukang.
- Google replaces the password, not the phone number: signing up with Google still needs the role, a name (taken from Google, editable) and the Indonesian phone number with its WhatsApp code (HT-009), so contact and penalty points work as for everyone. One account per phone number and one per Google account.
- After that, "Lanjutkan dengan Google" signs straight in, without a code. A Google account has no password: the phone-and-password form does not sign it in.
- A phone number already registered (with a password) cannot sign up again with Google: it signs in with its password. Linking Google to an existing account is later work.
- Admins never use Google: they keep password and WhatsApp code.
- Kept from Google: its account ID, the email address and the name given. The email is not shown to other users. Nothing is posted to Google.
- Needs a Google Cloud "OAuth client ID" (web application) for the site's address, made by the business owner; until it is set, the Google button does not show.

### Terms and policies (agreed: users must be aware of them)

Details proposed (awaiting approval):

- One page, "Syarat dan Kebijakan" (Terms and Policies), in Bahasa Indonesia and English, open to anyone, linked from the sign-up forms and from every screen's footer. It covers what Hai Tukang is and is not (it connects customers and repairmen; it is not party to the work, does not employ or verify repairmen, and does not handle payment), accounts, conduct, visits and arrival codes, penalty points and suspensions with their actual numbers, disputes and admin review, and privacy: what is kept, who sees what, and how to ask for an account to be deleted.
- No account without agreeing: both sign-up forms (password and Google) have an unticked box "Saya menyetujui Syarat dan Kebijakan", and the server refuses a sign-up without it. The version agreed to and the time are recorded.
- The page has a version (its date). When it changes, everyone who agreed to an older version sees, at their next visit, that it changed and must agree before using the app again. Admins are staff and are not asked.
- The text is a draft written from the app's actual rules. It must be reviewed (legally too, including Indonesia's personal data protection law, UU PDP No. 27/2022) and the operator's name and contact filled in before launch.

Chosen while building (awaiting approval):

- The page is at `#/syarat` (shareable), and shows over the app without closing it: a half-filled sign-up is still there on "Kembali". It carries a visible "Draf untuk masa uji coba" note until reviewed.
- No WhatsApp code is sent before the box is ticked, so no code is wasted.
- Accounts made before this (all current ones) have agreed to no version, so each is asked once at its next visit. Admins are not asked.
- While an account has not agreed, the server refuses everything else it asks (`terms_not_accepted`), not only the screen.
- Google: the button is Google's own ("Lanjutkan dengan Google", in the reader's language). The server checks each Google ID token's signature against Google's keys, that it is for Hai Tukang's client ID, from Google, and unexpired. Set with `--google-client-id` / `HAI_TUKANG_GOOGLE_CLIENT_ID`; the client's "authorized JavaScript origins" must include the site's address (`http://localhost:5175` in development).
- Not verified with real Google: there is no client ID yet. The flow was tested with a stand-in for Google's button and token check.

### Marketplace (agreed: partner stores sell materials, with a repair or on their own)

The owner's idea: "it can be bundle with repair when repairman come, or user just shop the material", choosing partner stores over a repairman's own material list, Shopee/Tokopedia links, or Hai Tukang selling from its own stock.

Details proposed (awaiting approval; the open ones are also asked as decisions):

- A store is a third kind of account, "Toko". **Agreed:** stores sign up themselves, like repairmen (phone number with its WhatsApp code, the terms), and an admin can suspend a bad one; no approval before they show. Its profile: store name, area (free text, as repairmen's), address, and how it hands over. **Agreed:** pickup at the store, plus the store's own delivery, with the fee and the areas it serves set by the store; courier booking comes later. Like repairmen, stores are not called "verified".
- A store lists products: name, kind of work (the same list: Listrik, Pipa & air, ...), price in rupiah, unit (pcs, meter, sak, kaleng, ...), available or sold out, and an optional short description. **Agreed:** up to 5 photos per product (the app gets image upload and storage). Stock counts come later.
- A customer finds materials by kind of work, words and area, and orders from one store at a time (a store's items in one order), choosing pickup or the store's delivery.
- An order goes: placed, then the store accepts it (it may say an item is sold out, which the customer sees before going on) or rejects it, then ready for pickup or out for delivery, then received, confirmed by the customer. Either side can cancel before it is ready.
- With a repair: a repairman's response can carry a material list made from store products; the customer orders it with one tap, attached to the confirmed visit, and chooses "the repairman picks it up" (he shows the order's code at the store) or "the store delivers before the visit". The customer sees materials (store prices) and labour as separate lines.
- **Agreed:** payment stays outside the app for now, as for repairs; later it moves into the app through DOKU (the owner: "Later, we use doku"). For now: the customer pays the store (cash on pickup or delivery, transfer, or the store's QRIS), and the app records the order and its total, not the money. The Terms and Policies get a new version saying so (Hai Tukang is not a party to the sale; the store is responsible for its products), which asks everyone to agree again (HT-014).
- No commission during the pilot.
- Admins can suspend a store, as any account, and hide a product.
- **Agreed build order:** side by side. The owner sets up the pilot's accounts (domain and server, a WhatsApp Business number, a Google client ID), which mostly means waiting for approvals; meanwhile the marketplace is built: stores and products (HT-015), then orders (HT-016), then materials with a repair (HT-017).
- Details chosen while building HT-015 (awaiting approval):
  - Store profile: name 2–80 characters, area 2–80, address 5–300, pickup and/or delivery (at least one); delivery fee Rp 0–10.000.000 and delivery areas up to 200 characters, dropped when the store does not deliver.
  - Product: name 2–120 characters, one kind of work, price Rp 1–1.000.000.000 per unit, unit 1–20 characters (free text), available or sold out, description up to 1000 characters. No limit yet on the number of products.
  - Photos: JPEG, PNG or WebP, up to 10 MB and 40 million pixels; the phone shrinks a photo before upload and the server keeps it as JPEG, at most 1600 pixels on its longer side, without anything else the file carried (a phone photo's GPS position). Photos are served at a random, unguessable address that anyone with the link can open.
  - Search needs a sign-in, as for repairmen: available products first, then sold out, newest change first, at most 50. A hidden product, or any product of a suspended store, is not found; the store still sees it, marked "Disembunyikan admin".
  - Hiding or showing a product is an admin decision with a reason, logged with the product and its store; the admin sees a store's profile and all its products when looking the store up.
  - Until orders (HT-016), customers ask the store on WhatsApp from the product page. Customers shop in a "Belanja bahan" tab; repairmen do not have it yet (HT-017).
  - Stores do not see the push-notification prompt (it is about visits) and have no penalty points; they can be suspended by hand.
  - The Terms and Policies got a new version (11 November 2026) with a "Toko dan bahan" section; everyone agrees again.
- Later, not in this step: payment in the app (DOKU), courier booking (GoSend/GrabExpress), stock counts, ratings.

## Acceptance scenarios

Add one scenario per agreed business outcome. Each criterion should be observable and independently verifiable. All four are checked automatically by `golang/apps/hai-tukang/internal/rpc/acceptance_test.go` (real API and PostgreSQL), and were walked through in the browser at 390px in both languages.

| ID | Scenario / Given | When | Then (acceptance criterion) | Status |
| --- | --- | --- | --- | --- |
| HT-001 | A person needs home repair or similar services | They register as a customer | A customer account is created | Built: name, phone, password; one account per phone number; sign-in and sign-out; phone verified by WhatsApp code (HT-009) — awaiting approval |
| HT-002 | A repairman wants to offer services | They register as a repairman | A repairman account is created | Built: as HT-001, then asked to complete a profile — awaiting approval |
| HT-003 | A customer needs a home repair and a suitable repairman has a profile | The customer browses listings and selects that repairman | The customer can contact the repairman about arranging the work | Built: search by kind of work and area; profile card with WhatsApp contact — awaiting approval |
| HT-004 | A customer needs a home repair | The customer posts a repair request and a repairman responds | The customer and repairman can connect to discuss arranging the work | Built: request → matching repairmen see it and respond → customer sees responses with WhatsApp; repairman gets the customer's WhatsApp after responding — awaiting approval |
| HT-005 | A repairman has responded to the customer's request | The customer chooses him and proposes a visit window, and he accepts (or proposes another window that the customer accepts) | Both see the same confirmed visit time | Built: propose, counter, accept, cancel; push to the side whose turn it is — awaiting approval |
| HT-006 | A visit is confirmed | The repairman arrives and enters the customer's arrival code | The visit is marked as arrived, with the time, for both | Built: 4-digit code shown to the customer only; 5 wrong tries lock it — awaiting approval |
| HT-007 | A confirmed visit's window ended more than 30 minutes ago without an arrival | The customer reports that the repairman did not come | The visit is recorded as a no-show, and the customer can choose another repairman who responded | Built: push to the customer when the 30 minutes pass; request reopens — awaiting approval |
| HT-008 | A repairman misses or cancels visits late | Penalty points add up to 6 within 90 days | He is suspended: not shown in search and cannot respond or accept visits, for 7 days the first time | Built: points, warning at 4, suspensions 7 days / 30 days / closed; hidden from search, no responses or new visits while suspended — awaiting approval |
| HT-009 | A person signs up as a customer or a repairman | They enter the code sent to their phone number | The account is created with a verified phone number; without the code, no account is created | Built: code rules as above; codes go to the server log until a WhatsApp provider is chosen — awaiting approval |
| HT-010 | A customer is not home for confirmed visits or cancels them late | Penalty points add up to 6 within 90 days | The customer is suspended: cannot post requests or confirm visits, for 7 days the first time | Built: "not at home" report by the repairman, late cancellations; no requests or new visits while suspended; disputes within 24 hours — awaiting approval |
| HT-011 | A no-show or not-home report has been disputed | An admin reviews both sides and decides | The points stand or are removed, with the reason recorded, and both sides see the outcome | Built — awaiting approval |
| HT-012 | An account was closed after its third suspension | An admin reviews it | The account is reopened or stays closed, with the reason recorded | Built, with suspending and lifting by hand — awaiting approval |
| HT-013 | A person wants to sign up without making a password | They continue with Google, choose customer or repairman, and verify their phone number by WhatsApp code | The account is created and signed in; later, "Lanjutkan dengan Google" signs them straight in | Built — awaiting approval |
| HT-014 | A person signs up, or the terms change | They read the Terms and Policies and agree | No account is created without agreeing; the agreed version and time are recorded; after a change, the app asks again before it can be used | Built — awaiting approval |
| HT-015 | A store owner wants to sell repair materials | They sign up as a store and list products with prices | Customers in its area can find those products | Built: store sign-up (third choice "Jual bahan"), store profile with pickup and/or its own delivery (fee, areas), products with up to 5 photos, search by words, kind of work and area (the store's or where it delivers), product page with WhatsApp to the store; an admin can hide a product or suspend the store — awaiting approval |
| HT-016 | A customer needs materials but no repairman | They find a product, order it from the store for pickup or delivery, and the store accepts | The order goes through ready or delivered to received, with its total shown to both; payment happens between them | Agreed — details proposed, not built |
| HT-017 | A repairman responds to a request and the job needs materials | He adds a material list from a store; the customer orders it with the visit | The store prepares it, the repairman picks it up with the order code (or the store delivers before the visit), and the customer sees materials and labour as separate lines | Agreed — details proposed, not built |

## Open questions

- Which fields are required for customer registration and repairman registration?
- How will accounts be verified, especially repairman profiles?
- Can one account register as both a customer and a repairman?
- What information should a repair request include, and how can repairmen respond?
- What details should a repairman's profile show, and how can a customer contact them?
- How are repairmen and their skills or services represented and verified?
- How should location and availability affect which repairmen are shown?
- Does Hai Tukang support messaging, quotes, scheduling, or booking, or only introduce both sides?
- Are payments, fees, reviews, or dispute handling part of the app?
- What kinds of work count as "similar" to home repair?
- What specific outcome will the business approver use to accept the first release?
- Which business number sends the WhatsApp codes through Twilio? Each message costs money (Meta's fee plus Twilio's).
