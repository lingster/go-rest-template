# SaaS webapp (SvelteKit)

Auth + admin UI for the Go/Beego REST API described in [`docs/API_CONTRACT.md`](../docs/API_CONTRACT.md).
SvelteKit 3 (Svelte 5 runes), TypeScript, Tailwind CSS 4, deployed to **Cloudflare Workers** with
`@sveltejs/adapter-cloudflare`.

## Development

Requires Node 22+.

```sh
cd webapp
cp .env.example .env     # optional, defaults to http://localhost:8080
npm install
npm run dev              # http://localhost:5173
```

The API must allow the webapp origin: set `CORS_ALLOWED_ORIGINS=http://localhost:5173` and
`FRONTEND_URL=http://localhost:5173` on the backend (these are the backend defaults).

> `.npmrc` sets `legacy-peer-deps=true` because npm 10's resolver crashes on the vitest 4 peer graph.

### Scripts

| Script               | What it does                                                          |
| -------------------- | --------------------------------------------------------------------- |
| `npm run dev`        | Vite dev server                                                       |
| `npm run build`      | `wrangler types --check` + production build (Worker)                  |
| `npm run check`      | `svelte-check` (also verifies `worker-configuration.d.ts`)            |
| `npm run lint`       | Prettier check + ESLint                                               |
| `npm run format`     | Prettier write                                                        |
| `npm test`           | Vitest, single run (CI friendly)                                      |
| `npm run test:unit`  | Vitest watch mode                                                     |
| `npm run preview:cf` | Build, then run the Worker locally with `wrangler dev`                |
| `npm run deploy`     | Build, then `wrangler deploy`                                         |
| `npm run gen`        | Regenerate `worker-configuration.d.ts` after editing `wrangler.jsonc` |

## Configuration

| Variable         | Default                 | Notes                                                             |
| ---------------- | ----------------------- | ----------------------------------------------------------------- |
| `PUBLIC_API_URL` | `http://localhost:8080` | API origin, without `/api/v1`. **Read at build time** (see below) |

| `PUBLIC_GOOGLE_SIGNIN` | `false` | `true` adds Google Identity Services to the CSP (build time) |

`PUBLIC_API_URL` is inlined into the client bundle by Vite and is also used to generate the
`connect-src` directive of the Content Security Policy (`vite.config.ts`). It therefore has to be set
when building, not only when running:

```sh
PUBLIC_API_URL=https://api.example.com npm run build
```

## Deploying to Cloudflare Workers

1. Authenticate once with `npx wrangler login`, or in CI export `CLOUDFLARE_API_TOKEN` (token with the
   "Edit Cloudflare Workers" template) and `CLOUDFLARE_ACCOUNT_ID`.
2. Optionally rename the Worker in `wrangler.jsonc` (`name`, default `saas-webapp`) and attach a custom
   domain / route.
3. Deploy with the production API URL:

   ```sh
   PUBLIC_API_URL=https://api.example.com npm run deploy
   ```

4. On the backend set `FRONTEND_URL` and `CORS_ALLOWED_ORIGINS` to the deployed webapp origin
   (e.g. `https://app.example.com`) so email links, CORS and the refresh cookie's origin check work.
5. Keep the webapp and API **same-site**: the session's refresh token is an HttpOnly cookie set by the API, and
   browsers block it as a third-party cookie when the two are on different sites. `app.example.com` +
   `api.example.com` works; `saas-webapp.<account>.workers.dev` + `api.example.com` does not. In that case attach a
   custom domain to the Worker, or route the API through the webapp's origin (e.g. a Cloudflare route
   `app.example.com/api/*` to the API) and build with `PUBLIC_API_URL=https://app.example.com`.

`npm run preview:cf` runs the built Worker locally (the CSP header is applied exactly as in production).

## Passkeys (WebAuthn) and the RP ID

Passkeys are bound to a Relying Party ID, which must be the webapp's registrable domain (or a parent of
its host), configured on the backend:

- local dev: `WEBAUTHN_RP_ID=localhost`, `WEBAUTHN_RP_ORIGINS=http://localhost:5173`
- production: `WEBAUTHN_RP_ID=example.com`, `WEBAUTHN_RP_ORIGINS=https://app.example.com`

The origin shown in the browser must be listed in `WEBAUTHN_RP_ORIGINS` and the browser needs a secure
context (HTTPS or `localhost`). Changing the RP ID invalidates every registered passkey. The client uses
`@simplewebauthn/browser` and passes `options.publicKey` from the contract as `optionsJSON`; the returned
credential JSON is sent as `credential`.

## Architecture

```
src/
  app.html, app.d.ts
  lib/
    api/
      client.ts        typed fetch wrapper: JSON, bearer, ApiError, single-flight refresh + retry
      http.ts          the app-wide client instance (base URL)
      types.ts         contract types
      auth.ts me.ts admin.ts   one function per endpoint
    stores/
      session.svelte.ts  rune-based auth state (access token in memory, refresh token in an HttpOnly cookie)
      mfa.svelte.ts      pending MFA challenge between /login and /login/2fa
    passkeys.ts        WebAuthn ceremonies (login + registration)
    forms.ts           error/validation helpers, safe redirect
    navigation.ts      goto() for validated dynamic paths
    components/        Alert, Button, Card, FormField, PasswordField, Pagination, AppShell, AuthShell
      security/        TotpSection, PasskeySection, RecoveryCodes
      admin/           UserActions
  routes/
    +page.svelte       public landing page (server-rendered)
    (auth)/            login, login/2fa, register, forgot-password, reset-password, verify-email
    (app)/             dashboard, settings/profile, settings/security   (auth guard)
    (admin)/admin/     stats, users, users/[id], audit-logs             (admin role guard)
```

The `(auth)`, `(app)` and `(admin)` groups set `ssr = false`: tokens live in the browser, so these pages
render client-side only. The landing page is server-rendered. Guards live in `AppShell.svelte`: it
restores the session (refresh token -> `/me`), redirects anonymous users to `/login?redirect=...`, and
sends non-admins away from `(admin)`.

### Token storage

The access token (short lived) is kept in memory only. The refresh token never reaches JavaScript: the
client sends `X-Session-Transport: cookie` with `credentials: 'include'` on every request, so the API
sets it as an `HttpOnly; Secure; SameSite=Strict` cookie scoped to `/api/v1/auth` and leaves it out of
the JSON. A reload restores the session by calling `/auth/refresh` (the cookie is sent automatically);
a failure means signed out. Sessions stored in `localStorage` by older builds are migrated once on load
(the stored token is sent in the body, the API answers with the cookie, the key is deleted).

An XSS bug can no longer steal the refresh token, but it can still act as the user while the page is
open, so the strict CSP (no inline scripts, no third-party origins) still matters. See
[`docs/SECURITY.md`](../docs/SECURITY.md) for the CSRF defence and the same-site requirement.

### Refresh flow

On a 401 with code `unauthorized`/`token_expired` the client calls `/auth/refresh` once (concurrent
requests share the same in-flight refresh, because refresh tokens rotate), keeps the new access token and
retries the original request. If refreshing fails the session is cleared and the user is sent to
`/login` by the guard.

### Content Security Policy

Configured in `vite.config.ts` (`csp`, mode `auto`): `default-src 'self'`, `script-src 'self'`,
`style-src 'self'`, `connect-src 'self' <API origin>`, `img-src 'self' data:` (the TOTP QR code is a
locally generated data URL), `frame-ancestors 'none'`, `object-src 'none'`. No CDNs, no inline event
handlers, no inline `style` attributes.
