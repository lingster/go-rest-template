// EDIT ME: what the store screenshots and the feature graphic say. Everything
// the listing images need to know about your app is in this one file; the
// rendering code lives in support/ and rarely needs to change.
//
// Regenerate after editing (docs: README, "Customise your store listing"):
//   flutter test test_store
import 'dart:ui' show Color;

/// Placeholder branding: replace with yours. [storeAppName] is renamed by
/// scripts/init_project.py together with the rest of the template.
const storeAppName = 'SaaS Starter';
const storeTagline = 'Your one-line pitch';

/// Brand gradient of the caption band and the feature graphic (the app's
/// colour scheme is seeded from 0xFF4F46E5 in lib/core/theme.dart).
const brandDark = Color(0xFF312E81);
const brandLight = Color(0xFF4F46E5);
const brandAccent = Color(0xFFC7D2FE);

/// One screenshot: file stem (the number orders them in the store), the app
/// route to open, the ASO caption in the band above the screen, and whether
/// the user is signed in.
typedef Shot = ({String file, String route, String caption, bool signedIn});

/// Play phone, App Store iPhone and iPad all use this list (Play takes 2 to
/// 8 shots, the App Store 1 to 10 per device). Keep captions short: a benefit
/// in a few words, written for search as well as for people.
const shots = <Shot>[
  (
    file: '1_sign_in',
    route: '/login',
    caption: 'Sign in with email, Google or a passkey',
    signedIn: false,
  ),
  (
    file: '2_home',
    route: '/home',
    caption: 'Your dashboard at a glance',
    signedIn: true,
  ),
  (
    file: '3_profile',
    route: '/profile',
    caption: 'Manage your profile',
    signedIn: true,
  ),
  (
    file: '4_security',
    route: '/security',
    caption: 'Two-factor and biometric security',
    signedIn: true,
  ),
];

/// App Store locales `deliver` uploads: the same scenes, one folder each.
/// Add a locale here (and its metadata/ folder) to ship another language.
const iosLocales = ['en-GB', 'en-US'];
