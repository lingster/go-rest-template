# New project checklist

The template ships working, but generic: indigo buttons, placeholder addresses and plain emails. Work through this
list when you start a project from it, before the first public release. Each item says why; most are an hour or less.
Tick them off in a PR so the reviewer can see what is left.

## 1. Brand the whole app, not just the landing page

The landing page (`backend/conf/landing.json`) is branded by `init_project.py`. These are not:

- [ ] **Colours.** Pick the primary colour and set it as the design tokens in `webapp/src/routes/layout.css` (light and
      dark). Then grep the webapp for the template's palette (`indigo`, `violet`, `purple`, `blue`) and replace every
      hard-coded colour with a token. The sign-in, sign-up, 2FA and password pages, `Button.svelte` and
      `AuthShell.svelte` use them. Check the auth pages in dark mode too: they must follow the site's theme toggle.
- [ ] **Auth pages look like the site.** Give `(auth)` pages the site header (logo, theme toggle) and footer, or at
      least the logo linking home, so sign-in does not look like a different website.
- [ ] **Logo and icons.** Replace the template's Svelte logo (`webapp/src/lib/assets/favicon.svg`) with your own, add
      favicon, apple-touch, 192/512 and maskable PNGs plus a web manifest in `webapp/static`, and regenerate the
      Flutter app icons from the same artwork.
- [ ] **Transactional emails** (`backend/mail/mail.go`: verify email, reset password, password changed, account
      exists). Give them one shared, email-safe layout: table-based, inline styles, max 600px, the logo as an
      absolute https PNG URL with the name as text beside it (images are often blocked), a bulletproof button in the
      brand colour, a "copy this link" fallback, when the link expires, and a footer with the site and support
      address. Keep the plain-text part in step. Use your product name in every subject line.

## 2. Make the sign-in forms work with password managers

Chrome, Safari, 1Password and Bitwarden guess wrong without these:

- [ ] Every input has a `name` (`email`, `password`, `current-password`, `new-password`).
- [ ] The account identifier uses `autocomplete="username"` (with `type="email"`) on register, login and forgot
      password, so the saved login pairs the address with the password. Login keeps `username webauthn` for passkeys.
- [ ] New-password fields carry the policy as `minlength`, `maxlength` and Safari's `passwordrules`, from the same
      constants the client-side check uses.
- [ ] Change- and reset-password forms include a hidden, read-only username field with the account's email, so the
      manager knows which saved login to update.
- [ ] Authenticator code fields use `autocomplete="one-time-code"`; a recovery-code field must not.
- [ ] Add `/.well-known/change-password`, redirecting to the change-password page.
- [ ] Flutter: fields inside an `AutofillGroup` with the right `autofillHints`, and
      `TextInput.finishAutofillContext()` after sign-up, sign-in and password change.

## 3. Email that actually sends

`MAIL_DRIVER=console` only writes reset and verification links to the logs, so with `REQUIRE_EMAIL_VERIFICATION=true`
nobody can finish signing up.

- [ ] Pick a transactional provider. Postmark works over plain SMTP: `MAIL_DRIVER=smtp`,
      `SMTP_HOST=smtp.postmarkapp.com`, `SMTP_PORT=587`, and the server token as both `SMTP_USERNAME` and
      `SMTP_PASSWORD`.
- [ ] Add the provider's DKIM TXT and Return-Path CNAME records (the CNAME DNS-only if you use Cloudflare), and check
      you have a DMARC record. Verify the domain in the provider before going live.
- [ ] Set `MAIL_FROM` to `Your App <no-reply@your-domain>` and `ADMIN_EMAIL` to a real address: the template's
      `example.com` values are placeholders.
- [ ] Send yourself a real verification and reset email from production, and read them on a phone.

## 4. Secrets and settings

- [ ] Keep API secrets in the GitHub environment (`Settings > Environments > production`) rather than in a file on the
      server, and have the deploy write them out, so changing one is a redeploy rather than a manual edit with sudo.
      Use environment *variables* for settings that are not secret, so they stay readable.
- [ ] Never commit `.env`; keep local copies mode 600.

## 5. Production readiness

- [ ] **Monitoring and alerts from day one**: an external probe of the public URL (and of the origin, to tell your
      outage from the CDN's), container up/restart/memory alerts, and app metrics on a `/metrics` that is not publicly
      routed. An outage nobody is told about lasts until someone happens to look.
- [ ] **Memory limits with headroom, and a health check that restarts.** Docker marks a container unhealthy but never
      restarts it; if a process can hang, run the container with `init: true` and let the health check stop it after a
      few failures so the restart policy replaces it. A web server running `wrangler dev` in Docker needs about 700 MB
      at rest: 512 MB gets OOM-killed. Prefer a production runtime over a dev server.
- [ ] **Backups off the host**, with a restore actually tested.
- [ ] **Legal pages**: privacy policy, terms and a support page with a real contact address, linked from the footer
      and the app stores. Self-service account deletion (the stores require it).

## 6. SEO

The landing page has its own SEO checklist in the README. Also:

- [ ] Redirect `http://` to `https://` (Cloudflare: *Always Use HTTPS*) and `www`/other domains to one canonical host.
- [ ] Check `robots.txt` and `sitemap.xml` return 200 through the CDN.
- [ ] Add the site to Google Search Console and Bing Webmaster Tools (Bing can import from Search Console), submit the
      sitemap, and turn on Cloudflare *Crawler Hints* so changed pages are announced through IndexNow.
- [ ] Pages people search for (guides, destination or category pages) with real content, linked from the home page and
      footer, and listed in the sitemap.
