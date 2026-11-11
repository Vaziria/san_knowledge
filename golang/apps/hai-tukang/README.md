# Hai Tukang backend

Go API for the Hai Tukang app, served with Connect RPC. Business requirements are in [`docs/acceptances/hai-tukang.md`](../../../docs/acceptances/hai-tukang.md).

## Local database

PostgreSQL runs in Docker Compose, on `127.0.0.1:5434` (5433 is often taken by other projects). The credentials in `docker-compose.yml` are for local development only.

```sh
docker compose up -d --wait   # start Postgres; data is kept in the hai-tukang_postgres-data volume
docker compose down           # stop it (add -v to also delete the data)
```

On first start, `docker/postgres-init/` also creates `hai_tukang_test`, a separate database for the integration tests.

## Run

One command line, `hai-tukang` (urfave/cli; `--help` on any command). Each command takes the database from `--database-url` or `DATABASE_URL`.

```sh
go run ./cmd/hai-tukang dev run                    # the API and the frontend's Vite dev server together
go run ./cmd/hai-tukang serve                      # the API alone (applies pending migrations first)
go run ./cmd/hai-tukang migrate                    # apply pending migrations (same as migrate up)
go run ./cmd/hai-tukang migrate status             # each migration, applied (when) or pending
go run ./cmd/hai-tukang migrate down               # lists what the last migration's undo would drop
go run ./cmd/hai-tukang migrate down --yes         # undo it (--to N: everything after version N)
```

`dev run` defaults to docker-compose.yml's database, applies pending migrations, serves the API on `--listen` and runs Vite from `typescripts/hai-tukang` (found from the current folder up, or `--frontend`), with its proxy pointed at the API (`HAI_TUKANG_API`). Vite listens on `http://127.0.0.1:5175` (fixed in `vite.config.ts`: 5173 belongs to another project; if 5175 is taken, Vite and with it `dev run` stop). A frontend change reloads by itself; a Go change needs a restart. Ctrl+C stops both, and either stopping stops the other. Without `node_modules` it runs `npm ci` first.

`serve` listens on `127.0.0.1:8087` by default (`--listen`, `HAI_TUKANG_ADDR`; 8085 is another project's); `--secure-cookies` (`HAI_TUKANG_SECURE_COOKIES=1`) behind HTTPS; `--push-contact` (`HAI_TUKANG_PUSH_CONTACT`); `--google-client-id` (`HAI_TUKANG_GOOGLE_CLIENT_ID`, also on `dev run`) turns on sign-in with Google; `--photos-dir` (`HAI_TUKANG_PHOTOS_DIR`, also on `dev run`, default `data/photos` under the current folder, made if missing) keeps the product photos: back it up with the database. It stops on Ctrl+C or SIGTERM, letting requests in flight finish (up to 5 seconds).

### Database and migrations

The stores use GORM (`gorm.io/gorm` with its Postgres driver; no code imports pgx). Migrations are SQL files in `internal/db/migrations/`, applied by goose (`-- +goose Up` / `-- +goose Down`; record in `goose_db_version`; a Postgres session lock keeps two migrators apart). One applied is never edited: a change is a new numbered file. A `Down` drops what its `Up` made, data included, and some fail while the data needs them (noted in the file). Before goose, an own runner applied 001-011 (recorded in `schema_migrations`); every one of them can run again on a database that has it, so goose took such databases over by running them once more, and 012 dropped the old record.

Two GORM details the stores rely on: errors are translated (`TranslateError`: `gorm.ErrDuplicatedKey`, `gorm.ErrForeignKeyViolated`), but only through GORM's own operations (`Create`, `Exec`, ...), not `Raw(...).Scan`, so inserts whose constraint errors mean something use `Create`. A `text[]` column goes through `db.TextArray` (`?::text::text[]` to write, `col::text` to read), since GORM expands a `[]string` argument into a list. Named parameters (`@name`) cannot be followed by a Postgres cast (`@name::text`): use `CAST(@name AS text)`.

### Sign-in with Google

Off until a client ID is set. In Google Cloud Console, create an OAuth client ID of type "Web application", with the site's address under "Authorized JavaScript origins" (`http://localhost:5175` for development: open the app at `localhost`, not `127.0.0.1`, as Google takes only `localhost` without HTTPS). Then run with `--google-client-id <id>.apps.googleusercontent.com`. The browser shows Google's button and gets an ID token; `internal/google` checks it (signature against Google's published keys, this client ID, Google as issuer, not expired). No client secret is needed.

### Terms and Policies

Customers and repairmen must agree to the current version to sign up, and again when it changes: until then, every call but `GetMe`, `AcceptTerms` and `Logout` answers `terms_not_accepted`. The version is `accounts.TermsVersion`, the date of the text in `typescripts/hai-tukang/src/terms/`; change the text, `TERMS_VERSION` there and `TermsVersion` together (a test compares the two).

### Admins

Admins (Hai Tukang staff) are made only from the command line, never by signing up:

```sh
DATABASE_URL='…' go run ./cmd/hai-tukang admin create --name 'Full Name' --phone 08…
```

It asks for the password twice (hidden at a terminal; from a pipe, two lines), and refuses a database with migrations pending. The number must be Indonesian and on WhatsApp: every admin sign-in needs a code sent there. An admin's sign-in lasts 12 hours (others' 30 days).

## API

Connect RPC, defined with Buf in `protos/haitukang/v1/` (the same setup as San Tower). The generated code is kept in the repo: Go in `gen/`, TypeScript in `typescripts/hai-tukang/src/gen/`. After changing a `.proto`, run `buf lint && buf generate` here (or `npm run gen` in the frontend).

- `AccountService`: `SendSignUpCode`, `Register`, `Login`, `Logout`, `GetMe`, `SaveProfile`, `ListRepairmen`; sign-up and sign-in with Google, `GetSignInOptions`, `SignInWithGoogle`, `RegisterWithGoogle` (HT-013); and `AcceptTerms` (HT-014).
- `RepairRequestService`: `CreateRepairRequest`, `ListMyRepairRequests`, `ListOpenRepairRequests`, `RespondToRepairRequest`.
- `VisitService`: `ProposeVisit`, `CounterVisit`, `AcceptVisit`, `CancelVisit`, `ConfirmArrival`, `ReportNoShow`, `ListMyVisits`.
- `NotificationService`: `GetPushKey`, `SavePushSubscription`, `DeletePushSubscription`.
- `PenaltyService`: `GetMyStanding`. `VisitService` also has `ReportCustomerAbsent` and `DisputeReport`.
- `AdminService` (admins only): `ListDisputes`, `DecideDispute`, `ListClosedAccounts`, `DecideClosedAccount`, `FindAccount`, `SuspendAccount`, `LiftSuspension`, `ListDecisions`, `SetProductHidden` (HT-015).
- `StoreService` (HT-015): `GetMyStore`, `SaveStoreProfile`, `SaveProduct`, `DeleteProduct`, `AddProductPhoto`, `DeleteProductPhoto`, `SearchProducts`, `GetProduct`.
- `GET /photos/{key}` serves product photos (JPEG, cached a year: a key never changes what it holds).
- `GET /healthz` is a plain health check.

Services are at their Connect paths (`/haitukang.v1.AccountService/Register`, …). `internal/rpc` serves them over the domain packages (`internal/accounts`, `internal/requests`, `internal/catalog`, `internal/verify`).

**Visits (HT-005 to HT-007), `internal/visits`.** The customer proposes a window (in the future, within 30 days, 30 minutes to 8 hours) to a repairman who responded; the other side accepts or proposes another. A request has at most one proposed or confirmed visit (a unique index). Confirming makes a 4-digit arrival code that only the customer is sent; the repairman enters it on arrival (5 wrong tries lock it). 30 minutes after the window without an arrival, the customer can report a no-show; the request then reopens to the other repairmen who responded. A confirmed or completed visit takes the request off repairmen's open list.

**Penalty points (HT-008, HT-010), `internal/penalties`.** A no-show or a customer not at home gives 2 points, counting after the 24 hours the reported side has to dispute (`DisputeReport`); a disputed report's points never count until an admin reviews them. Cancelling a confirmed visit less than 12 hours before it, or arriving more than 30 minutes after the window, gives 1 point at once. Points expire after 90 days. At 4 an account is warned; at 6 it is suspended and those points are used up: 7 days, then 30, then closed until an admin reviews it. While suspended, an account arranges no new visits; a repairman is hidden from search and cannot respond, a customer cannot post requests. Visits record their points in the same transaction (`penalty_points`); the server applies points that begin to count once a minute.

**Browser push, `internal/push`.** The other side is told about a proposed window, the proposer about a confirmed one, and the customer (by a once-a-minute check) about a window that passed 30 minutes ago without an arrival. The server makes its VAPID keys on first start and keeps them in `push_keys`; changing them ends every subscription. Only browser push services are accepted as endpoints, since the server POSTs to them. Set `HAI_TUKANG_PUSH_CONTACT` (`mailto:` or `https:`) to your contact for push services. Messages are written in each device's language and time zone.

**Phone verification (HT-009).** `SendSignUpCode` sends a one-time code on WhatsApp to an Indonesian number (+62) with no account yet; `Register` needs it. A code is 6 digits and works once, for 5 minutes, and only while it is the newest for the number; 5 wrong tries end it. A new one can be sent after 60 seconds, at most 5 an hour per number. Only a hash of each code is kept (`phone_codes`). There is no WhatsApp provider yet: the server writes codes to its log (`verify.LogSender`) and says so at start. Never run it like that for real users: anyone who reads the log can sign up with any number. A provider is a `verify.Sender`.

**Sign-in.** `Register` and `Login` set an `ht_session` cookie: HttpOnly, SameSite=Lax, 30 days (admins: 12 hours). For an admin, `Login` without a code sends one to WhatsApp and answers `code_sent` without signing in; `Login` again with the code signs in. Set `HAI_TUKANG_SECURE_COOKIES=1` when the site is served over HTTPS. Sessions are stored as SHA-256 hashes of a random token.

**Admin reviews (HT-011, HT-012), `internal/admin`.** Every decision needs a reason of 5 to 500 characters and is written to `admin_decisions` in the same transaction as its effect. On a dispute, the points stand (and count at once) or are removed; the visit shows the outcome to both sides. A closed account is reopened (its counting points removed; the suspension count stays) or kept closed (marked reviewed). Suspensions by hand (`kind = 'admin'`) last 1 to 365 days and are not steps toward closing. Search judges suspensions at the penalties' clock (`accounts.Service.WithSearchClock`, for tests).

**Stores and materials (HT-015), `internal/stores`, `internal/photos`.** A store is a fourth role (`store`), signing up like a customer. Its profile (`store_profiles`) has a name, area, address, and pickup and/or its own delivery (fee in rupiah, areas as free text); its products (`products`) a name, a kind of work, a price per unit, a unit, available or sold out, and a description; each product up to 5 photos (`product_photos`; the limit holds under uploads at once: the product row is locked). A photo is decoded, checked (JPEG, PNG or WebP; 10 MB; 40 million pixels), made at most 1600 pixels on its longer side and saved again as JPEG, which drops its EXIF (a phone photo's GPS position); it is kept by a `photos.Storage` under a random key (`photos.Disk` now; S3-compatible storage can take over). Search (signed in) matches words in the name and description, the kind of work, and the area against the store's area and where it delivers; available first, at most 50. A product an admin hid (`SetProductHidden`, a logged decision) or of a suspended store is not found; the store still sees it.

**Errors.** A Connect code plus an `ErrorDetail` (`error.proto`) with a stable `reason`, which is what the frontend translates: `invalid_role`, `invalid_name`, `invalid_phone`, `weak_password`, `phone_taken`, `invalid_credentials`, `unauthorized`, `forbidden`, `invalid_service`, `invalid_area`, `invalid_about`, `invalid_description`, `invalid_message`, `not_found`, `already_responded`, `profile_required`, `invalid_code`, `code_expired`, `too_many_attempts`, `resend_too_soon`, `too_many_codes`, `code_not_sent`, `invalid_window`, `visit_exists`, `not_responded`, `not_your_turn`, `visit_changed`, `wrong_arrival_code`, `arrival_locked`, `no_show_too_early`, `invalid_subscription`, `suspended`, `repairman_unavailable`, `absent_not_now`, `dispute_closed`, `invalid_note`, `invalid_reason`, `invalid_days`, `not_disputed`, `not_closed`, `already_suspended`, `not_suspended`, `store_profile_required`, `too_many_photos`, `invalid_store_name`, `invalid_address`, `no_handover`, `invalid_delivery_fee`, `invalid_delivery_areas`, `invalid_product_name`, `invalid_price`, `invalid_unit`, `invalid_product_description`, `invalid_photo`, `photo_too_large`, `internal`. The error message is English, for developers; never show it to users. Unexpected errors are logged and answered as `internal`, without the cause. A new reason goes in `internal/rpc/errors.go` and needs a message in each frontend language (`typescripts/hai-tukang/src/i18n/`).

**Rules.**
- Accounts: name 2–80 characters, phone 8–15 digits, password at least 8 characters, stored as a bcrypt hash. Phones are stored in one form: `0812…`, `62812…` and `+62 812…` all become `+62812…`, so the same number cannot register twice.
- Repairman profile: one or more kinds of work (`listrik`, `pipa`, `atap`, `cat`, `ac`, `bangunan`, `kayu`, `lainnya`), an area of 2–80 characters, and an optional "about" of at most 500.
- Searching repairmen needs a sign-in; the area matches as a case-insensitive substring.
- Repair requests: only customers post them (kind of work, area, a 10–1000 character description). Only repairmen with a profile respond, once per request, with a 2–500 character message.
- Phone numbers: the customer sees each responding repairman's number; a repairman sees the customer's number only after responding.
- Not yet: a real WhatsApp sender, password reset, limits per IP address on sending codes.

## Check

```sh
buf lint
go test ./...
go vet ./...
go build ./...

# Include the PostgreSQL tests, and the acceptance test (internal/rpc/acceptance_test.go),
# which walks HT-001 to HT-004 through the generated clients (skipped without this variable):
HAI_TUKANG_TEST_DATABASE_URL='postgres://hai_tukang:hai_tukang_dev@127.0.0.1:5434/hai_tukang_test?sslmode=disable' go test ./...
```
