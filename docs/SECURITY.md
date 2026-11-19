# Security notes

## Token storage trade-offs

- **Access tokens** are short-lived (15 min) HS256 JWTs; **refresh tokens** are opaque, stored hashed (SHA-256) server-side and rotated on every use. Reuse of a revoked refresh token revokes the whole family.
- **Flutter (mobile)**: keep tokens in the platform keystore (`flutter_secure_storage`), not in plain preferences.
- **Webapp**: tokens in `localStorage` are readable by any XSS payload. Prefer keeping the access token in memory and the refresh token in an `HttpOnly; Secure; SameSite` cookie if you move auth to the same site, and keep the CSP strict so XSS is unlikely. Whatever you pick, document the trade-off for your users.
- **Flutter web** shares the browser trade-offs above.

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

## Reporting a vulnerability

Please report security issues privately via GitHub Security Advisories on this repository rather than public issues.
