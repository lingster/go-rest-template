# API Contract (source of truth for backend, Flutter and webapp)

All three projects MUST follow this contract exactly. Base path: `/api/v1`. JSON only.

## Conventions

- Auth header: `Authorization: Bearer <access_token>`.
- IDs are UUID strings. Timestamps are RFC3339 UTC strings.
- Success responses return the resource JSON directly (no envelope).
- Error responses: HTTP status + body
  ```json
  { "error": { "code": "invalid_credentials", "message": "Human readable", "details": {"field": "reason"} } }
  ```
  `details` is optional (used for `validation_error`).
- Error codes: `validation_error`(422), `invalid_credentials`(401), `unauthorized`(401), `invalid_token`(400/401),
  `token_expired`(401), `email_not_verified`(403), `account_disabled`(403), `forbidden`(403),
  `invalid_mfa_code`(401), `not_found`(404), `conflict`(409), `rate_limited`(429), `internal_error`(500).
- Password policy: min 8 chars, max 128.

## Objects

```jsonc
// User
{ "id": "uuid", "email": "a@b.com", "name": "Ada", "role": "user|admin",
  "email_verified": true, "is_active": true, "totp_enabled": false, "passkey_count": 0,
  "created_at": "...", "updated_at": "..." }

// Tokens
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
| POST | `/auth/register` | `{email, password, name}` | 201 `{user: User, message}` |
| POST | `/auth/login` | `{email, password}` | 200 AuthResponse **or** MfaChallenge |
| POST | `/auth/login/2fa` | `{mfa_token, code}` (TOTP 6 digits or recovery code) | 200 AuthResponse |
| POST | `/auth/refresh` | `{refresh_token}` | 200 Tokens |
| POST | `/auth/logout` | `{refresh_token}` | 204 |
| POST | `/auth/verify-email` | `{token}` | 200 `{message}` |
| POST | `/auth/resend-verification` | `{email}` | 202 `{message}` (always 202) |
| POST | `/auth/forgot-password` | `{email}` | 202 `{message}` (always 202, no user enumeration) |
| POST | `/auth/reset-password` | `{token, password}` | 200 `{message}` (revokes all refresh tokens) |
| POST | `/auth/passkey/login/begin` | `{email?}` (omit for discoverable credential) | 200 `{session_id, options}` |
| POST | `/auth/passkey/login/finish` | `{session_id, credential}` | 200 AuthResponse (passkey satisfies MFA) |

Login when `REQUIRE_EMAIL_VERIFICATION=true` and email unverified → 403 `email_not_verified`.
Disabled user → 403 `account_disabled`.

## Authenticated user endpoints (Bearer)

| Method | Path | Body | Response |
|---|---|---|---|
| GET | `/me` | – | User |
| PATCH | `/me` | `{name}` | User |
| POST | `/me/change-password` | `{current_password, new_password}` | 200 `{message}` |
| POST | `/me/2fa/totp/setup` | – | `{secret, otpauth_url}` (pending until enabled) |
| POST | `/me/2fa/totp/enable` | `{code}` | `{recovery_codes: [10 strings]}` |
| POST | `/me/2fa/totp/disable` | `{password, code}` | 200 `{message}` |
| POST | `/me/2fa/recovery-codes` | `{code}` | `{recovery_codes: [...]}` (regenerate) |
| GET | `/me/passkeys` | – | `[Passkey]` |
| POST | `/me/passkeys/register/begin` | – | `{session_id, options}` |
| POST | `/me/passkeys/register/finish` | `{session_id, name, credential}` | 201 Passkey |
| DELETE | `/me/passkeys/:id` | – | 204 |

## Admin endpoints (Bearer, role=admin)

| Method | Path | Body / Query | Response |
|---|---|---|---|
| GET | `/admin/stats` | – | `{users_total, users_active, users_verified, users_2fa}` |
| GET | `/admin/users` | `?page=1&per_page=20&q=` | Paginated<User> |
| GET | `/admin/users/:id` | – | User |
| PATCH | `/admin/users/:id` | `{name?, role?, is_active?, email_verified?}` | User |
| DELETE | `/admin/users/:id` | – | 204 (cannot delete self) |
| POST | `/admin/users/:id/revoke-sessions` | – | 204 |
| GET | `/admin/audit-logs` | `?page&per_page&user_id=` | Paginated<`{id, user_id, actor_id, action, ip, user_agent, metadata, created_at}`> |

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
`SMTP_PASSWORD`, `MAIL_FROM`, `FRONTEND_URL` (http://localhost:5173), `CORS_ALLOWED_ORIGINS` (comma list),
`WEBAUTHN_RP_ID` (localhost), `WEBAUTHN_RP_NAME`, `WEBAUTHN_RP_ORIGINS` (comma list),
`ADMIN_EMAIL` / `ADMIN_PASSWORD` (seed an admin on first start if set).

## Repository layout

```
backend/        Go + Beego v2 API (module github.com/lingster/go-rest-template/backend), main at backend/main.go
backend/swagger swagger.json generated by `bee generate docs` (committed), UI served at /swagger/
flutter_app/    Flutter client (Riverpod, go_router, dio, flutter_secure_storage)
webapp/         SvelteKit (sv create, TS, Tailwind, adapter-cloudflare) auth + admin UI
.github/        CI (push to main / PR) + release workflow (on release published)
docs/           this contract + other docs
```
Dev ports: API 8080, webapp 5173, Mailpit UI 8025 / SMTP 1025.
