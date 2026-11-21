# API Contract (source of truth for backend, Flutter and webapp)

All three projects MUST follow this contract exactly. Base path: `/api/v1`. JSON only.

## Conventions

- Auth header: `Authorization: Bearer <access_token>`.
- Session transport: native clients (Flutter) get and send the refresh token in JSON bodies. Browser clients (the
  webapp) send `X-Session-Transport: cookie` on every request with credentials included; the refresh token then lives
  in an HttpOnly cookie (see "Cookie session transport" below) and is never in a body.
- IDs are UUID strings. Timestamps are RFC3339 UTC strings.
- Success responses return the resource JSON directly (no envelope).
- Error responses: HTTP status + body
  ```json
  { "error": { "code": "invalid_credentials", "message": "Human readable", "details": {"field": "reason"} } }
  ```
  `details` is optional (used for `validation_error`).
- Error codes: `validation_error`(422), `invalid_credentials`(401), `unauthorized`(401), `invalid_token`(400/401),
  `token_expired`(401), `email_not_verified`(403), `account_disabled`(403), `forbidden`(403),
  `invalid_mfa_code`(401), `not_found`(404), `conflict`(409), `rate_limited`(429), `internal_error`(500),
  `reauthentication_required`(403: the operation needs a recent sign-in; sign in again and retry),
  `account_exists_sign_in_required`(409, `POST /auth/google` only: see below).
- `rate_limited` is also returned per **account** (from any IP) after 10 second-factor or current-password attempts
  in 15 minutes (`/auth/login/2fa`, `/me/2fa/*`, `/me/change-password`).
- `POST /auth/login` returns `rate_limited` (429) once **failed** password checks reach 10 per 15 minutes for the same
  email from the same client (IPv4 address / IPv6 /64), or 100 per hour for that email from any client. Only failures
  count, a correct password from another client still works, and unknown emails are throttled and answered identically
  (no account enumeration). While blocked, even the correct password gets 429. Passkey and Google sign-in are not
  throttled this way.
- An MFA challenge (`mfa_token`) and a refresh token stop working (401 `invalid_token`) once the account's sessions
  are revoked (password change/reset, admin revoke-sessions, deactivation).
- Password policy: min 8 chars, max 128.

## Objects

```jsonc
// User
{ "id": "uuid", "email": "a@b.com", "name": "Ada", "role": "user|admin|reception|cover_manager",
  "email_verified": true, "is_active": true, "totp_enabled": false, "passkey_count": 0,
  "has_password": true, "identities": ["google"],   // has_password=false for Google-created accounts
  "created_at": "...", "updated_at": "..." }

// Tokens (refresh_token is omitted with X-Session-Transport: cookie)
{ "access_token": "jwt", "refresh_token": "opaque", "token_type": "Bearer", "expires_in": 900 }

// AuthResponse (successful login)
{ "mfa_required": false, "tokens": Tokens, "user": User }

// MfaChallenge (password ok, 2FA needed)
{ "mfa_required": true, "mfa_token": "short-lived jwt (5 min)", "methods": ["totp", "recovery_code", "passkey"] }

// Passkey
{ "id": "uuid", "name": "MacBook", "created_at": "...", "last_used_at": "... | null" }

// Paginated<T>
{ "items": [T], "total": 123, "page": 1, "per_page": 20 }
```

Access token: HS256 JWT, claims `sub` (user id), `role`, `typ: "access"`, `exp`, `iat`, `jti`. TTL default 15m.
Refresh token: random opaque string, stored hashed (SHA-256) in DB, rotated on every refresh
(old one revoked; reuse of a revoked token revokes the whole family). TTL default 30 days.

## Public endpoints

| Method | Path | Body | Response |
|---|---|---|---|
| GET | `/healthz` (no /api/v1 prefix) | – | `{"status":"ok"}` |
| POST | `/auth/register` | `{email, password, name}` | 202 `{message}`: identical for new and already-registered addresses (see below) |
| POST | `/auth/login` | `{email, password}` | 200 AuthResponse **or** MfaChallenge |
| POST | `/auth/login/2fa` | `{mfa_token, code}` (TOTP 6 digits or recovery code) | 200 AuthResponse |
| GET | `/auth/providers` | – | 200 `{"google": {"enabled": bool, "web_client_id": "..."}}` (public) |
| POST | `/auth/google` | `{id_token}` (Google ID token) | 200 AuthResponse **or** MfaChallenge; 401 `invalid_token`; 404 `google_signin_disabled`; 409 `account_exists_sign_in_required` |
| POST | `/auth/refresh` | `{refresh_token}`, or none with the cookie | 200 Tokens |
| POST | `/auth/logout` | `{refresh_token}`, or none with the cookie | 204 (idempotent) |
| POST | `/auth/verify-email` | `{token}` | 200 `{message}` |
| POST | `/auth/resend-verification` | `{email}` | 202 `{message}` (always 202) |
| POST | `/auth/forgot-password` | `{email}` | 202 `{message}` (always 202, no user enumeration) |
| POST | `/auth/reset-password` | `{token, password}` | 200 `{message}` (signs out everywhere: refresh **and** access tokens) |
| POST | `/auth/passkey/login/begin` | `{email?}` (accepted, ignored: always a discoverable ceremony, user verification required) | 200 `{session_id, options}` |
| POST | `/auth/passkey/login/finish` | `{session_id, credential}` | 200 AuthResponse (passkey satisfies MFA) |

### Cookie session transport (browsers)

With the request header `X-Session-Transport: cookie`:
- `/auth/login`, `/auth/login/2fa`, `/auth/google`, `/auth/passkey/login/finish` (when they return an AuthResponse) and
  `/auth/refresh` set the refresh token as a cookie and omit `refresh_token` from the JSON. An MfaChallenge sets nothing.
- Cookie: `__Secure-rt` (`rt` when the server runs with `COOKIE_SECURE=false`), `HttpOnly; Secure; SameSite=Strict;
  Path=/api/v1/auth; Max-Age=<refresh TTL>`, host-only unless `COOKIE_DOMAIN` is set.
- `/auth/refresh` and `/auth/logout` read the token from the cookie when the body has none. A body token wins (a client
  can move a stored token into the cookie by sending it once with the header). Refresh rotates the cookie; a failed
  refresh (4xx) clears it; logout always clears it. With the header but no cookie and no body: refresh → 401
  `invalid_token`, logout → 204.
- CSRF: a request authenticated by the cookie without the header, or with an `Origin` not explicitly listed in
  `CORS_ALLOWED_ORIGINS`, → 403 `forbidden`. Without the header and without a body token or cookie → 422 as before.
- CORS allows the `X-Session-Transport` header and sends `Access-Control-Allow-Credentials: true` to explicitly listed
  origins only. The webapp and API must be same-site (e.g. `app.example.com` + `api.example.com`); otherwise route the
  API through the webapp's origin, because browsers block third-party cookies.

Login when `REQUIRE_EMAIL_VERIFICATION=true` and email unverified → 403 `email_not_verified`.
Disabled user → 403 `account_disabled`.

Registration (`POST /auth/register`) never reveals whether an address is taken: it answers `202 {message}` with the same
body either way and returns no user. A new address gets the verification email; an existing one is left untouched and its
owner is emailed an "you already have an account" notice with sign-in and reset-password links (throttled per address,
shared with forgot-password). Clients should show the message ("check your email") and send the user to sign in. With
`REQUIRE_EMAIL_VERIFICATION=false` a fresh account can sign in immediately, which the registrant (only) can tell apart.

Google sign-in (`POST /auth/google`): matched by Google `sub`, else by case-insensitive email **only when Google is
authoritative for that address** (`gmail.com`/`googlemail.com`, or the ID token's `hd` claim equals the email's domain):
the identity is linked and the email marked verified (if the local account was unverified, its password, TOTP, recovery
codes, passkeys and sessions are wiped first). For any other domain with an existing account the call fails with 409
`account_exists_sign_in_required`: sign in with the password instead. If no account has that email, a new account is
created with `has_password=false` (for every domain). TOTP users get the same MfaChallenge and
finish via `/auth/login/2fa`. Password login and `/me/change-password` fail for accounts without a password (generic
`invalid_credentials` / 409 `password_not_set`); such users set one through forgot/reset-password. `/me/2fa/totp/disable`
takes `password` only when `has_password=true`.

## Authenticated user endpoints (Bearer)

| Method | Path | Body | Response |
|---|---|---|---|
| GET | `/me` | – | User |
| PATCH | `/me` | `{name}` | User |
| POST | `/me/change-password` | `{current_password, new_password}` | 200 `{message}` (signs out everywhere, including this session) |
| POST | `/me/2fa/totp/setup` | – | `{secret, otpauth_url}` (pending until enabled; needs a recent sign-in) |
| POST | `/me/2fa/totp/enable` | `{code}` | `{recovery_codes: [10 strings]}` |
| POST | `/me/2fa/totp/disable` | `{password, code}` | 200 `{message}` |
| POST | `/me/2fa/recovery-codes` | `{code}` | `{recovery_codes: [...]}` (regenerate) |
| GET | `/me/passkeys` | – | `[Passkey]` |
| POST | `/me/passkeys/register/begin` | – | `{session_id, options}` (needs a recent sign-in) |
| POST | `/me/passkeys/register/finish` | `{session_id, name, credential}` | 201 Passkey |
| DELETE | `/me/passkeys/:id` | – | 204 |

## Admin endpoints (Bearer, role=admin)

| Method | Path | Body / Query | Response |
|---|---|---|---|
| GET | `/admin/stats` | – | `{users_total, users_active, users_verified, users_2fa}` |
| GET | `/admin/users` | `?page=1&per_page=20&q=` | Paginated<User> |
| GET | `/admin/users/:id` | – | User |
| PATCH | `/admin/users/:id` | `{name?, role?, is_active?, email_verified?}` (role: `user`, `admin`, `reception`, `cover_manager`) | User |
| DELETE | `/admin/users/:id` | – | 204 (cannot delete self) |
| POST | `/admin/users/:id/revoke-sessions` | – | 204 (refresh and access tokens stop working at once) |

Sessions: access tokens carry the user's session version; password change/reset, revoke-sessions and deactivation
bump it, so outstanding access tokens get 401 `unauthorized` immediately (clients then try to refresh, which also fails).
"Recent sign-in" means a password/Google/passkey login within `REAUTH_MAX_AGE` (10 min); refreshing does not renew it.
| GET | `/admin/audit-logs` | `?page&per_page&user_id=` | Paginated<`{id, user_id, actor_id, action, ip, user_agent, metadata, created_at}`> |

## Cover desk (supply teachers)

Supply-teacher bookings, agency ETA updates and the reception sign-in log. Roles: `reception` (front desk),
`cover_manager` (arranges cover); `admin` has cover-manager rights. Dates are `YYYY-MM-DD` in the school's zone
(`SCHOOL_TIMEZONE`, default `Europe/London`); `HH:MM` inputs are wall-clock times in that zone on the booking date;
every timestamp in a response is RFC3339 UTC. School day: P1 08:55–09:55, P2 09:55–10:55, P3 11:15–12:15,
P4 13:00–14:00, P5 14:00–15:00; booking kinds `full` = P1–P5, `am` = P1–P3, `pm` = P4–P5.

```jsonc
// Agency (never includes the key or its hash)
{ "id": "uuid", "name": "Kestrel Teaching", "phone": "01632 960 214", "is_active": true, "created_at": "..." }
// AgencyKeyResponse = Agency + { "api_key": "agk_..." }   (shown once; only the SHA-256 hash is stored)
// SupplyTeacher
{ "id": "uuid", "agency_id": "uuid", "name": "Priya Natarajan", "subject": "Science", "dbs_confirmed": true, "created_at": "..." }
// Lesson
{ "period": "P1", "start": "08:55", "end": "09:55", "class_name": "9X2", "room": "Lab 2", "free": false }
// Booking (cover manager view)
{ "id": "uuid", "agency_id": "uuid", "supply_teacher_id": "uuid|null", "date": "2026-11-23", "kind": "full|am|pm",
  "covering": "Mr Okafor", "subject": "Science", "status": "requested|confirmed|cancelled",
  "expected_arrival": "2026-11-23T08:15:00Z", "lanyard": "S-01", "lessons": [Lesson], "created_at": "...", "updated_at": "..." }
// SignInEvent (append-only log entry)
{ "id": "uuid", "booking_id": "uuid", "teacher_name": "...", "type": "signin|correction", "ref_id": "uuid (correction)",
  "at": "...", "actor_user_id": "uuid", "lanyard": "S-01", "photo_id_checked": true,
  "dbs_evidence": "agency_confirmation|certificate_seen", "reason": "wrong_person|not_on_site|checks_incomplete (correction)" }
```

### Cover managers (Bearer, role `cover_manager` or `admin`; reception gets 403)

| Method | Path | Body / Query | Response |
|---|---|---|---|
| POST | `/cover/agencies` | `{name, phone}` | 201 AgencyKeyResponse; 409 name taken |
| GET | `/cover/agencies` | – | `[Agency]` |
| POST | `/cover/agencies/:id/rotate-key` | – | 200 AgencyKeyResponse (the old key stops working at once) |
| POST | `/cover/teachers` | `{agency_id, name, subject?, dbs_confirmed}` | 201 SupplyTeacher |
| GET | `/cover/teachers` | `?agency_id=` | `[SupplyTeacher]` |
| POST | `/cover/bookings` | `{agency_id, date, kind, covering, subject, expected_arrival: "HH:MM", lanyard?, lessons: [{period, class_name, room, free}]}` | 201 Booking (`requested`) |
| PATCH | `/cover/bookings/:id` | `{status?: "confirmed"\|"cancelled", supply_teacher_id?, covering?, lanyard?, lessons?}` | 200 Booking |

- `lessons` must list each period of the kind exactly once (`free: true` for booked time without a class; otherwise
  `class_name` and `room` are required). Agency must exist and be active.
- Confirm: `status: "confirmed"` with `supply_teacher_id`, which must belong to the booking's agency (422 otherwise);
  a confirmed booking can be re-assigned the same way. `supply_teacher_id` is rejected without `status: "confirmed"`.
- Cancelled bookings are final (409 on any change). Status changes on a signed-in booking → 409 (correct the sign-in
  first). A concurrent status change → 409. Date, kind, agency, subject and expected arrival are fixed at creation.

### Reception and cover managers (Bearer, role `reception`, `cover_manager` or `admin`)

| Method | Path | Body / Query | Response |
|---|---|---|---|
| GET | `/cover/today` | `?date=YYYY-MM-DD` (default: today) | Today view (role-projected, below) |
| GET | `/cover/week` | `?start=YYYY-MM-DD` (a Monday; default: this week; 422 otherwise) | Week view (role-projected) |
| POST | `/cover/bookings/:id/sign-in` | `{photo_id_checked: true, dbs_certificate_seen: bool}` | 201 SignInEvent |
| POST | `/cover/sign-ins/:id/corrections` | `{reason}` | 201 SignInEvent (type `correction`) |
| GET | `/cover/sign-ins` | `?date=` (booking date, default today) | `{date, items: [SignInEvent]}` newest first |

**Today view.** Non-cancelled bookings of the date, sorted by expected arrival, each with a derived `status`:
`signed_in` (a sign-in not withdrawn by a correction) > `unconfirmed` (booking not confirmed) > `no_eta` (no agency
update yet) > `running_late` (latest ETA ≥ 5 min after the expected arrival) or `on_the_way`. `eta_stale` is true when
the latest ETA (while travelling) was received more than 20 minutes ago. `counts` has every status.

```jsonc
// Reception: CoverTodayReception
{ "date": "2026-11-23", "timezone": "Europe/London", "counts": {"signed_in": 1, "unconfirmed": 0, "no_eta": 0, "on_the_way": 2, "running_late": 1},
  "arrivals": [{ "booking_id": "uuid", "teacher_name": "...", "agency_name": "...", "agency_phone": "...",
    "expected_arrival": "...", "status": "on_the_way", "eta": "...|null", "eta_updated_at": "...|null", "eta_stale": false,
    "lanyard": "S-01", "dbs_certificate_required": false }] }
// Cover manager / admin: CoverTodayManager = the same plus, per arrival,
//   "covering", "subject", "kind", "first_lesson": {period, start, class_name, room} | null, "dbs_confirmed", "note"
// and a top-level "attention": [{ "severity": "bad|warn", "type": "unconfirmed|cover_gap|eta_stale", "booking_id", "message", "detail" }]
```

Attention items: an unconfirmed booking whose first lesson has not ended (`bad`); a cover gap when the ETA is later
than 5 minutes before the first taught lesson starts, while that lesson has not ended ("P1 Science needs cover until
<ETA+5>"); a stale ETA. Problems are listed before warnings.

**Week view.** `{start, rows: [{teacher_name, agency_name, days: [{date, bookings: [cell]}] x5}]}`, one row per supply
teacher (each unassigned booking has its own row, `teacher_name` empty). Reception cells: `{booking_id, kind, status}`
(booking status); cover managers additionally get `covering` and `lessons`.

**Sign-in.** Only confirmed bookings dated today (school time) that are not currently signed in (409 otherwise).
`photo_id_checked` must be true, and when the teacher's agency has not confirmed a DBS check `dbs_certificate_seen` must
be true (422 with `details.photo_id_checked` / `details.dbs_certificate_seen`). The server records the time, the signed-in
user, the booking's lanyard and the DBS evidence (`certificate_seen` if seen, else `agency_confirmation`). Two
simultaneous sign-ins of one booking cannot both succeed: the loser gets 409.

**Corrections.** `reason` is one of `wrong_person`, `not_on_site`, `checks_incomplete`. Only the booking's current
sign-in can be corrected (409 if already withdrawn or superseded; 404 for unknown ids or a correction's id). The
original entry stays in the log; the booking can then be signed in again. Nothing in the log is ever edited or deleted.

### Agencies (header `X-Agency-Key: <api_key>`)

| Method | Path | Body / Query | Response |
|---|---|---|---|
| GET | `/agency/bookings` | `?date=` (default today) | `{date, items: [{id, date, kind, status, teacher_name, expected_arrival}]}` (own bookings only) |
| POST | `/agency/bookings/:id/eta` | `{eta: "HH:MM" \| RFC3339, note?}` | 201 `{id, booking_id, eta, reported_at, note}` |

- A missing, unknown, rotated-out or deactivated key → 401 `unauthorized` with one identical message. Too many invalid
  keys from one client (20 per 15 min) → 429; each key is limited to `AGENCY_RATE_LIMIT_PER_MINUTE` (120) requests/min.
- ETA: another agency's booking → 404 exactly like an unknown id. The booking must be confirmed, dated today and not
  signed in (409). The ETA must fall within 06:00–16:00 school time on the booking date and be no more than 10 minutes
  before the server's clock (422). `reported_at` is the server's receive time. `note` is at most 120 characters after
  control/format characters are removed. At most one update per booking per 30 s and 100 per booking → 429
  `rate_limited`. Agencies send an ETA only, never a location.

## WebAuthn / passkeys

- `options` is exactly what github.com/go-webauthn/webauthn returns (`{"publicKey": {...}}`), binary fields base64url.
- `credential` is the standard WebAuthn JSON serialization (`PublicKeyCredential.toJSON()` shape,
  base64url fields: `id`, `rawId`, `type`, `response{clientDataJSON, attestationObject|authenticatorData, signature, userHandle}`).
- `session_id` is an opaque server-side key for the stored ceremony session (5 min TTL).

## Email links

- Verify: `{FRONTEND_URL}/verify-email?token=<token>`
- Reset:  `{FRONTEND_URL}/reset-password?token=<token>`
Tokens are random, stored hashed, single use; verify TTL 24h, reset TTL 1h.
Both the SvelteKit webapp and Flutter (go_router, web + deep links) implement these two routes.

## Backend configuration (env vars)

`APP_ENV` (dev|prod), `HTTP_PORT` (8080), `DB_DRIVER` (sqlite|postgres), `DATABASE_URL`
(sqlite: `file:data/app.db`, postgres DSN), `JWT_SECRET`, `ACCESS_TOKEN_TTL` (15m), `REFRESH_TOKEN_TTL` (720h),
`REQUIRE_EMAIL_VERIFICATION` (true), `MAIL_DRIVER` (console|smtp), `SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME`,
`SMTP_PASSWORD`, `MAIL_FROM`, `FRONTEND_URL` (http://localhost:5173), `CORS_ALLOWED_ORIGINS` (comma list; `*` refused in
prod), `COOKIE_SECURE` (true in prod), `COOKIE_DOMAIN` (empty = host-only),
`WEBAUTHN_RP_ID` (localhost), `WEBAUTHN_RP_NAME`, `WEBAUTHN_RP_ORIGINS` (comma list),
`ADMIN_EMAIL` / `ADMIN_PASSWORD` (seed an admin on first start if set),
`GOOGLE_CLIENT_IDS` (comma list of web/iOS/Android OAuth client IDs, first = web; empty disables Google sign-in),
`REAUTH_MAX_AGE` (10m), `SMTP_REQUIRE_TLS` (true in prod), `SWAGGER_ENABLED` (true; false in prod),
`SCHOOL_TIMEZONE` (Europe/London; must be a valid IANA zone), `AGENCY_RATE_LIMIT_PER_MINUTE` (120 per agency key, 0 disables).

## Repository layout

```
backend/        Go + Beego v2 API (module github.com/lingster/go-rest-template/backend), main at backend/main.go
backend/swagger swagger.json generated by `bee generate docs` (committed), UI served at /swagger/ (when `SWAGGER_ENABLED`)
flutter_app/    Flutter client (Riverpod, go_router, dio, flutter_secure_storage)
webapp/         SvelteKit (sv create, TS, Tailwind, adapter-cloudflare) auth + admin UI
.github/        CI (push to main / PR) + release workflow (on release published)
docs/           this contract + other docs
```
Dev ports: API 8080, webapp 5173, Mailpit UI 8025 / SMTP 1025.
