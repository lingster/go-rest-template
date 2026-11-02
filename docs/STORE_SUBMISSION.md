# Store submission checklist

Taking the app from internal testing to production on **Google Play** and the **App Store**. The
listing text and images are uploaded from the repo ([RELEASING.md](RELEASING.md#store-listing)); the
forms below have no API and are filled in by hand. Tick each box and keep the answers consistent
between the stores and with what the app really does. Credentials for reviewers never go in the repo.

## Facts to reuse (both stores)

Fill in once, then copy from here:

| | |
|---|---|
| Developer / data controller | |
| Contact email | |
| Website | |
| Privacy policy URL (https, must load) | |
| Terms of use URL | |
| Account deletion URL (web) | |
| Account deletion (in app) | e.g. Profile, Delete account |
| Minimum age | |
| Category | (the App Store category id is in `ios/fastlane/metadata/primary_category.txt`) |
| Ads / analytics / crash SDKs | none, unless you added some |

## Requirements both stores check

- [ ] **Privacy policy URL**: a public https page naming what you collect, why, who processes it
      (hosting, email, any SDK) and how to contact you. Used in Play *App content* and in
      `ios/fastlane/metadata/*/privacy_url.txt`. App Store precheck fetches it: it must not 404.
- [ ] **Account deletion**: both stores require that an app which lets users create an account also lets
      them delete it **from inside the app**, and Play additionally wants a **web URL** where a user can
      request deletion without the app. The template does not ship a delete-account endpoint: add one
      (and the screen) before you submit, and document the retention of anything you must keep.
- [ ] **Support and marketing URLs** (`support_url.txt`, `marketing_url.txt`): real pages.
- [ ] **Screenshots and text** reflect the final app, with no placeholder copy left
      (`grep -ri placeholder flutter_app/*/fastlane`).

## Google Play (*Play Console, your app, Policy and programs, App content*)

- [ ] **Privacy policy**: the URL above.
- [ ] **App access**: if anything is behind a login, choose *All or some functionality is restricted*
      and add instructions with a **demo account** (email and password, no 2FA, with whatever
      entitlement is needed to reach every feature). Keep the credentials out of the repo.
- [ ] **Ads**: declare whether the app contains ads.
- [ ] **Content rating** (IARC questionnaire): pick the category (usually *All other app types*),
      answer honestly about violence, sexuality, language, controlled substances, gambling and
      user-generated content (does a user see other users' content or chat?) and location sharing.
- [ ] **Target audience**: age groups (and *appeals to children*: no, unless it is a kids' app, which
      adds Families policy requirements).
- [ ] **Data safety**: for every data type the app collects (see the table below), say collected or
      not, required or optional, and the purpose; confirm encryption in transit (HTTPS) and give the
      deletion URL. Service providers that process data on your behalf are not "sharing".
- [ ] **News / COVID-19 / government / financial features / health**: answer *no* unless it applies.
- [ ] **Store settings**: category, contact email, website. **Store listing**: uploaded by
      `play-listing.yml`; check it renders, then save.
- [ ] **Release**: roll out the current draft internal release (ends draft-app status, then delete the
      `PLAY_RELEASE_STATUS` variable), choose countries, *Production, Create new release, Add from
      library*, review, send for review. A new personal account must run a closed test for 14 days with
      testers first; organisation accounts are exempt.

### Data collected by the template (starting point for the Data safety form)

| Play data type | Collected by the template | Why |
|---|---|---|
| Personal info: email address | yes, required | account and sign-in |
| Personal info: name | yes, optional display name | profile |
| Personal info: user IDs | yes (account id; Google subject when Sign in with Google is used) | account management |
| App info and performance | no, until you add crash reporting or analytics | |
| Device or other IDs | no, until you add push notifications | |
| Location, contacts, photos, files, health | no | |

Passkeys and biometrics: the biometric check stays on the device; the server stores a passkey's public
credential only. Add rows for everything you build on top.

## App Store (*App Store Connect, your app*)

- [ ] **App Privacy ("privacy nutrition labels")**: declare each data type, whether it is linked to the
      user, and whether it is used for tracking. The same inventory as the Data safety table above; keep
      the two forms identical in substance.
- [ ] **Privacy Policy URL** and **Support URL** (*App Information*, and `privacy_url.txt` / `support_url.txt`).
- [ ] **Age rating** questionnaire.
- [ ] **App Review Information**: contact details and a **demo account** (with a note on how to sign in;
      no 2FA on it), as App Review cannot create accounts or receive your codes.
- [ ] **Export compliance**: `ITSAppUsesNonExemptEncryption=false` is set in `Info.plist` (HTTPS only).
- [ ] **Account deletion** in the app (guideline 5.1.1(v)), see above.
- [ ] **Sign in with Apple**: required if you offer another third-party social login (e.g. Google) as
      the *only* way to sign in. The template also has email, password and passkey sign-in, which
      avoids the requirement; re-check if you change that. Do not mention "Google" or "Android" in
      the iOS listing text (precheck fails on it).
- [ ] **Pricing, availability and tax forms** (paid apps and in-app purchases need the Paid Apps agreement).
- [ ] **Version information**: the listing is uploaded by `ios-metadata.yml`; pick the TestFlight build,
      *Add for Review*, submit.

## After approval

- [ ] Keep the privacy policy, both privacy forms and the app's behaviour in step on every release.
- [ ] Re-run the listing workflows when you change text or screenshots.
