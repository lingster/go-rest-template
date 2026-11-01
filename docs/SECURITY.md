# Security notes

## Token storage trade-offs

- **Access tokens** are short-lived (15 min) HS256 JWTs; **refresh tokens** are opaque, stored hashed (SHA-256) server-side and rotated on every use. Reuse of a revoked refresh token revokes the whole family.
- **Flutter (mobile)**: keep tokens in the platform keystore (`flutter_secure_storage`), not in plain preferences.
- **Webapp**: tokens in `localStorage` are readable by any XSS payload. Prefer keeping the access token in memory and the refresh token in an `HttpOnly; Secure; SameSite` cookie if you move auth to the same site, and keep the CSP strict so XSS is unlikely. Whatever you pick, document the trade-off for your users.
- **Flutter web** shares the browser trade-offs above.

## Content Security Policy

- The webapp should ship a strict CSP (no `unsafe-inline`; use nonces or hashes for any inline script/style; `connect-src` limited to your API origin). SvelteKit supports this via `kit.csp` in `svelte.config.js`.
- Avoid inline event handlers and inline styles; serve third-party libraries locally.
- Serve the API with `Strict-Transport-Security`, `X-Content-Type-Options: nosniff` and a restrictive CORS allow-list (never `*` with credentials).

## Rate limiting

- Auth endpoints (login, register, forgot/reset password, resend verification, 2FA) must be rate limited per IP and per account; the API returns `429 rate_limited`.
- The built-in limiter is in-memory and per-instance. Behind multiple instances, enforce limits at the edge (Cloudflare rules) or move the counters to a shared store.
- Password reset and resend-verification always respond 202 to avoid user enumeration.

## Secrets

- Never commit secrets. `JWT_SECRET` must be a long random value (32+ bytes) unique per environment; rotating it invalidates all access tokens.
- Store deploy credentials (`CLOUDFLARE_API_TOKEN`, `CLOUDFLARE_ACCOUNT_ID`) as GitHub Actions **secrets**, and public values (`PUBLIC_API_URL`, `API_BASE_URL`) as **variables**.
- Use a Cloudflare API token scoped to Workers deploys only.
- WebAuthn: set the relying party ID and allowed origins to your real domains; passkeys are bound to them.
- Android release signing: keep the keystore and passwords in secrets, never in the repo.

## Reporting a vulnerability

Please report security issues privately via GitHub Security Advisories on this repository rather than public issues.
