# Security notes

## Token storage trade-offs

- **Access tokens** are short-lived (15 min) HS256 JWTs; **refresh tokens** are opaque, stored hashed (SHA-256) server-side and rotated on every use. Reuse of a revoked refresh token revokes the whole family.
- **Flutter (mobile)**: tokens travel in JSON bodies and are kept in the platform keystore (`flutter_secure_storage`), not in plain preferences.
- **Webapp**: the access token is kept in memory only. The refresh token is an `HttpOnly` cookie that page scripts cannot
  read, so an XSS payload cannot exfiltrate it. The webapp opts in by sending `X-Session-Transport: cookie` on every
  request (with `credentials: 'include'`); the API then sets the cookie on login / 2FA / Google / passkey login and refresh
  and leaves `refresh_token` out of the body. On load the webapp restores the session with a cookie `POST /auth/refresh`.
  A refresh token left in `localStorage` by an older build is sent once in the body to move it into the cookie, then deleted.
  Requests that set or clear the cookie (login, 2FA, Google, passkey finish, refresh, logout) are sent one at a time, so a
  slow restore or refresh can never overwrite a newer sign-in's cookie or re-create it after logout.
  - Cookie: `__Secure-rt` (`rt` when `COOKIE_SECURE=false`, plain-HTTP dev only), `HttpOnly; Secure; SameSite=Strict;
    Path=/api/v1/auth; Max-Age=<REFRESH_TOKEN_TTL>`, host-only unless `COOKIE_DOMAIN` is set. `__Host-` is not used because
    it forbids a path. Logout and failed refreshes clear it.
  - CSRF: a refresh/logout authenticated by the cookie must carry `X-Session-Transport: cookie`, which a cross-site form or
    simple request cannot send without passing a CORS preflight, and its `Origin`, when present, must be explicitly listed in
    `CORS_ALLOWED_ORIGINS`; otherwise 403 `forbidden`. `SameSite=Strict` is a second layer. CORS sends
    `Access-Control-Allow-Credentials: true` only to explicitly listed origins, never with `*`, and `APP_ENV=prod` refuses to
    start with `CORS_ALLOWED_ORIGINS=*` (the allow-list is the CSRF origin check; native apps do not use CORS).
  - **Same-site deployment required**: the webapp and API must share a registrable domain (e.g. `app.example.com` +
    `api.example.com`, or one host). Browsers treat cookies on a different site (e.g. `*.workers.dev` -> `api.example.com`)
    as third-party and block them, and `SameSite=Strict` would withhold them anyway. For such deployments, serve the API on
    the webapp's own origin (e.g. a Cloudflare route `app.example.com/api/*` to the API) and set `PUBLIC_API_URL` to it.
  - XSS is still serious: injected script cannot steal the refresh token, but while the page is open it can call the API
    as the user (including `/auth/refresh` to mint access tokens). Keep the CSP strict.
- **Flutter web** sends no `X-Session-Transport` header, so it gets body tokens and shares the old browser trade-off
  (anything it stores is readable by XSS). Switch it to the cookie transport if it ships to production.

## Sessions and step-up

- Access tokens carry the user's session version (`ver`). Password change/reset, admin "revoke sessions", deactivation
  and the Google pre-hijack wipe bump it, so outstanding access tokens stop working at once, not after their 15 min TTL.
- The version is bound to the other bearer credentials too. Each refresh-token row stores the version it was issued
  under, and `/auth/refresh` rejects (and revokes the family of) a token whose version is not the user's current one,
  read after the token is consumed. This closes a race where a rotation that loaded the user before a revoke-all
  could mint a refresh token after the old rows were revoked and later restore the session. The MFA challenge JWT
  also carries `ver`; completing `/auth/login/2fa` fails with 401 if the version moved, so a pending challenge does not
  survive a password reset or revoke-sessions. Refresh tokens issued before this column existed count as version 0,
  so users whose version is already above 0 sign in once more after the upgrade.
- Access tokens also carry `auth_time`, the last time a credential was proven. Refresh rotation keeps it, so a stolen
  refresh token cannot be used to look "freshly signed in". Adding a credential (passkey registration, TOTP setup)
  requires `auth_time` within `REAUTH_MAX_AGE` (10 min) or returns 403 `reauthentication_required`; otherwise a stolen
  session could plant a passkey that survives a password reset.
- Passkeys are registered as discoverable and **user-verifying** (PIN/biometric): one passkey both signs in and satisfies
  MFA, so a bare touch of a stolen security key must not be enough. Assertions with a clone warning (sign counter went
  backwards) are rejected. Login ceremonies never include `allowCredentials`, so they do not reveal accounts or credential IDs.

## Content Security Policy

- The webapp should ship a strict CSP (no `unsafe-inline`; use nonces or hashes for any inline script/style; `connect-src` limited to your API origin). SvelteKit supports this via `kit.csp` in `svelte.config.js`.
- Avoid inline event handlers and inline styles; serve third-party libraries locally.
- Serve the API with `Strict-Transport-Security`, `X-Content-Type-Options: nosniff` and a restrictive CORS allow-list (never `*` with credentials).

## Sign in with Google

- The server verifies the Google ID token itself (RS256 signature against Google's JWKS, `iss`, `aud` in `GOOGLE_CLIENT_IDS`, `exp`/`iat` with 30s leeway, `email_verified`). Never trust a client-supplied email.
- Google is a first factor only: accounts with TOTP receive the usual MFA challenge.
- Linking an existing local account by email requires that Google is **authoritative** for the address: a `gmail.com`/`googlemail.com` address, or an ID token whose `hd` (hosted domain) claim equals the address's domain. For other domains `email_verified` only says Google once verified the address (domains lapse and get re-registered), so the request fails with 409 `account_exists_sign_in_required` and is audited (`auth.google_link_denied`); the user signs in with their password instead. Matching by Google `sub`, and creating a brand-new account for an unused email, work for every domain.
- When an authoritative link hits an account whose email was never verified locally (someone may have pre-registered the victim's address), Google counts as proof of ownership: the email is marked verified and the password, TOTP secret, recovery codes, passkeys and all sessions are wiped (audited as `auth.unverified_account_reset`). The owner sets a password through the reset flow if they want one. This prevents pre-registration account takeover.
- Google-created accounts have no password (empty hash, never verifies). Password-requiring flows: login fails generically, change-password returns 409 `password_not_set` (use the e-mail reset flow, which proves mailbox control), TOTP disable needs only the TOTP/recovery code.
- Webapp CSP must allow `accounts.google.com` (script/frame/connect/style) only when Google sign-in is enabled.

## Rate limiting

- Per client: `/api/v1/auth/*` is limited per IPv4 address or per IPv6 **/64** (one subscriber's block), so rotating
  IPv6 addresses does not reset the budget. With `TRUST_PROXY=true` only the **last** `X-Forwarded-For` entry is used;
  earlier entries are client-supplied and would let anyone choose their own rate-limit key and audit-log IP.
- Per account (any IP): 10 second-factor attempts and 10 current-password checks per 15 min, then `429 rate_limited`.
  Without this, a botnet that knows the password can brute-force a 6-digit TOTP in minutes.
- Password login counts **failed password checks only**, in two buckets: 10 per 15 min per (email, client IP / IPv6 /64),
  and 100 per hour per email from anywhere. Past either, `/auth/login` answers `429 rate_limited` before verifying
  the password (so no hash is computed, and even the correct password is refused). Because the tight bucket is scoped
  to the client, an attacker cannot lock the real user out: their address and sign-ins from other addresses keep
  working, and a correct password clears that client's bucket. The per-email ceiling bounds distributed guessing
  and is deliberately high; at that point the account owner can still sign in with a passkey or Google. Unknown
  emails are counted and answered identically, so throttling reveals nothing about which accounts exist.
  **Passkey and Google sign-in are not throttled by this**: they are the user's escape hatch during an attack, and
  neither accepts guessable secrets. Checking and recording are separate steps, so a burst of concurrent requests
  can overshoot a bucket by the number of requests in flight; the per-IP limit above bounds that.
- At most 5 forgot-password / resend-verification emails per address per hour (anti inbox-flooding).
- The limiters are in-memory and per-instance. Behind multiple instances, enforce limits at the edge (Cloudflare rules) or move the counters to a shared store (`ratelimit.Limiter` interface).
- Password reset and resend-verification always respond 202 to avoid user enumeration. Mail is delivered by a background
  queue, so the response time does not reveal whether an email was sent.

## API documentation

- `SWAGGER_ENABLED` controls the spec and Swagger UI at `/swagger/`. It defaults to **on** in dev and **off** when
  `APP_ENV=prod` (the path then returns 404, like any unknown route). Enabling it in prod publishes the API surface to
  everyone and logs a startup warning; if you need it there, put it behind your edge's authentication.

## Account enumeration

- `POST /auth/register` answers `202 {message}` with an identical body whether or not the address is registered, and
  returns no user object. A new address gets the verification email; an existing one is left unchanged and its owner is
  emailed an "account already exists" notice (sign-in and reset links), throttled with the same per-address mail limit as
  forgot-password and silently skipped when throttled. The password is hashed in both paths (the hash cost dominates)
  and mail goes through the background queue, so neither status, body nor timing differs.
- Resistance is complete only with `REQUIRE_EMAIL_VERIFICATION=true`. With it off, a fresh account can sign in at once,
  so the registrant can tell "new" from "already exists" by trying to log in (their password only works on a new
  account). That is visible to the person who submitted the form, not in the register response; keep verification on
  if enumeration matters.
- Login, forgot-password and passkey login already give the same answer for unknown and known accounts.

## Email

- `SMTP_REQUIRE_TLS` (default on in prod) makes delivery fail instead of falling back to plaintext when STARTTLS is not
  offered, which also defeats STARTTLS-stripping. SMTP conversations time out after 30s.
- `MAIL_DRIVER=console` prints reset links to the logs; prod logs a warning if it is used.

## Containers

- `backend/Dockerfile`: digest-pinned build and runtime images, `go mod verify`, static binary on distroless
  `nonroot` (no shell), built-in `HEALTHCHECK` (`/server healthcheck`).
- `docker-compose.yml` is a **development** stack: ports bound to 127.0.0.1, read-only root filesystems, `cap_drop: ALL`,
  `no-new-privileges`, non-root users (Postgres runs as `postgres` without its root entrypoint), memory/PID limits,
  rotated logs and a Postgres network Mailpit cannot reach. Do not expose Mailpit: it shows every reset link.
- For production run the same image with `read_only`, `cap_drop: ALL`, `no-new-privileges`, secrets from your platform's
  secret store (not a committed file), TLS in front, and `APP_ENV=prod`.

## Secrets

- Never commit secrets. `JWT_SECRET` must be a long random value (32+ bytes) unique per environment; rotating it invalidates all access tokens **and** makes stored TOTP secrets undecryptable (the at-rest key is derived from it), so users with 2FA would need recovery codes.
- `ADMIN_PASSWORD` is only for the first start: prod refuses the example value or anything under 12 characters. Remove it from the environment once the admin exists (a warning is logged while it is set).
- Store deploy credentials (`CLOUDFLARE_API_TOKEN`, `CLOUDFLARE_ACCOUNT_ID`) as GitHub Actions **secrets**, and public values (`PUBLIC_API_URL`, `API_BASE_URL`) as **variables**.
- Use a Cloudflare API token scoped to Workers deploys only.
- WebAuthn: set the relying party ID and allowed origins to your real domains; passkeys are bound to them.
- Android release signing: keep the keystore and passwords in secrets, never in the repo.

## Cover desk

- **Agency API keys.** `agk_` + 256 random bits, returned exactly once (on create and on rotate). Only the SHA-256 hash
  is stored (a high-entropy key needs no slow hash, and the hash is the lookup index); listings never return a key or
  hash. Rotation replaces the hash, so the old key fails immediately. A missing, wrong, oversized, rotated-out or
  deactivated key gets the same 401 message, so responses never reveal whether a key exists. Agency calls are not
  under the per-IP `/auth` limit (one integration usually calls from one address); instead each client gets 20 invalid
  keys per 15 min before 429 (only failures count, so a valid key is never locked out by others), and each valid key is
  limited to `AGENCY_RATE_LIMIT_PER_MINUTE` (120). `X-Agency-Key` is deliberately absent from the CORS allow-list:
  agencies integrate server-to-server, so browsers cannot send the key cross-origin. A bearer token is not an agency
  credential and vice versa. Like the other limiters these are in-memory and per-instance.
- **Agency isolation.** An agency only lists its own bookings, and reporting an ETA for another agency's booking is a
  404 identical to an unknown id, so booking ids cannot be probed. Agencies never receive class, room or "covering"
  (absent member of staff) data.
- **Server-side role projection.** Reception and cover managers share endpoints, but the service builds the full view
  once and projects it down for reception into separate response types (`CoverTodayReception`, `CoverWeekReception`)
  that have no fields for covering, subject, lessons, rooms, agency notes or attention items. Filtering is never left
  to the client. Manager-only actions are guarded by `RequireCoverManager` in the controller and re-checked in the
  services; `admin` implies cover-manager rights.
- **Append-only safeguarding log.** Sign-ins and corrections are `sign_in_events` rows that are never updated or deleted:
  the repository interface has no update or delete methods (likewise for ETA updates). A mistaken sign-in is withdrawn
  by a correction (fixed reasons) that references it; the original stays visible. Each entry records the server time,
  the signed-in user who acted, the lanyard and the DBS evidence; photo ID must be confirmed, and a DBS certificate must
  be seen when the agency has not attested an enhanced DBS check. Sign-ins and corrections are also audited.
- **Sign-in concurrency.** `bookings.version` is an optimistic-concurrency token. Every booking mutation (status,
  teacher assignment, covering, lanyard, lessons) and every sign-in log append (sign-in or correction) runs, in ONE
  database transaction, `UPDATE bookings SET version = version + 1 ... WHERE id = ? AND version = ?` (a sign-in also
  requires `status = confirmed` and the teacher it validated against) followed by its inserts; lessons are replaced in
  that same transaction. If the compare-and-set matches no row the operation is rolled back and gets 409, so a
  sign-in and a cancel or reassignment that both read "confirmed, not signed in" can never both be applied. A booking's
  events also strictly alternate (sign-in, correction, sign-in, ...), so each event's `seq` is the number of earlier
  events and `(booking_id, seq)` has a unique index: a second guard (and deterministic log order) that makes two
  simultaneous sign-ins or two corrections of one sign-in lose with 409. The mechanism needs no row locks or mutable
  "current sign-in" column and works the same on SQLite and Postgres.
- **Teacher snapshot on log events.** Each sign-in and correction stores the `supply_teacher_id` and `teacher_name`
  the booking had when it was appended (a correction copies them from the sign-in it withdraws), and the log is shown
  from those stored values, never the booking's current teacher. Reassigning a booking later therefore cannot rewrite
  who was recorded as signed in.
- **ETA only, no location.** Agencies report an expected arrival time and an optional note, never a location. The
  receive time is the server's clock (a client timestamp is never trusted), ETAs outside 06:00–16:00 school time on the
  booking date or more than 10 minutes in the past are rejected, notes are capped at 120 characters with control and
  format characters (newlines, bidi overrides, zero-width) removed, and each booking accepts at most one update per 30 s
  and 100 in total. The limits are atomic: every update carries a `seq` (the number of earlier updates) with a unique
  `(booking_id, seq)` index, so simultaneous reports computed from the same earlier updates collide, the loser gets
  429, and neither the cap (`seq < 100`) nor the 30 s interval can be beaten by racing requests. Rate-limited reports
  are not audited (noise); accepted ones are their own append-only table.
- **Data minimisation for reception.** Reception sees the teacher's name, the agency's name and phone, times, status,
  lanyard and whether a DBS certificate must be checked, which is what the front desk needs; not who is absent, what
  is taught or where.
- **Time zone.** `SCHOOL_TIMEZONE` is validated at start-up and the tz database is compiled into the binary; dates and
  `HH:MM` inputs are converted with the zone's rules for that date, so DST changes do not shift bookings or ETA windows.

## Reporting a vulnerability

Please report security issues privately via GitHub Security Advisories on this repository rather than public issues.
