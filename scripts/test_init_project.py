"""Tests for init_project.py: run it on a copy of the repo and check the result.

    python3 -m unittest discover -s scripts -v
"""

import json
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import init_project as ip  # noqa: E402

REPO = Path(__file__).resolve().parent.parent
OLD_NAMES = ("go-rest-template", "Go REST Template", "SaaS Starter", "saas_app", "saasApp", "saas-webapp", "com.example")


def copy_repo(dst: Path) -> Path:
    files = subprocess.run(["git", "ls-files", "-z"], cwd=REPO, capture_output=True, check=True).stdout
    for rel in filter(None, files.decode().split("\0")):
        src = REPO / rel
        if src.is_file():
            (dst / rel).parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(src, dst / rel)
    return dst


class NamingTests(unittest.TestCase):
    def test_defaults_for_leading_digit(self):
        self.assertEqual(ip.slugify("6-Pack App"), "6-pack-app")
        self.assertEqual(ip.default_dart_package("6-pack-app"), "six_pack_app")
        self.assertEqual(ip.default_bundle_id("lingster", "6-pack-app"), "com.lingster.sixpackapp")
        self.assertEqual(ip.default_worker("6-pack-app"), "six-pack-app")

    def test_validation_rejects_bad_values(self):
        s = ip.Settings("X", "Bad Slug", "o", "github.com/o/x/backend", "9pkg", "com.9x.app", "w", "a", "not-an-email")
        msgs = " ".join(s.validate())
        for flag in ("--slug", "--dart-package", "--bundle-id", "--email"):
            self.assertIn(flag, msgs)


class GoogleHelperTests(unittest.TestCase):
    def test_reversed_client_id(self):
        self.assertEqual(ip.reversed_client_id("1234-abc.apps.googleusercontent.com"), "com.googleusercontent.apps.1234-abc")


class LandingHelperTests(unittest.TestCase):
    def test_origins(self):
        self.assertEqual(ip.site_origin("https://www.acme.test/path"), "https://www.acme.test")
        self.assertEqual(ip.webapp_origin("https://www.acme.test/"), "https://app.acme.test")
        self.assertEqual(ip.webapp_origin("https://acme.test"), "https://app.acme.test")

    def test_title_fits_search_results(self):
        self.assertEqual(ip.landing_title("Acme", "Fast"), "Acme \u2013 Fast")
        self.assertEqual(ip.landing_title("Acme", "x" * 80), "Acme")

    def test_ts_string_quoting(self):
        self.assertEqual(ip.ts_string("plain"), "'plain'")
        self.assertEqual(ip.ts_string("it's"), '"it\'s"')

    def test_validation_rejects_bad_landing_values(self):
        s = ip.Settings("X", "x", "o", "github.com/o/x/backend", "x", "com.o.x", "x", "a", "a@b.co",
                        tagline="two\nlines", site_url="http://insecure.test")
        msgs = " ".join(s.validate())
        self.assertIn("--tagline", msgs)
        self.assertIn("--site-url", msgs)


class InitProjectTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.root = copy_repo(Path(self.tmp.name))

    def tearDown(self):
        self.tmp.cleanup()

    def run_init(self, *extra):
        argv = ["--yes", "--skip-docs", "--name", "6-Pack App", "--owner", "acme", "--author", "Acme Ltd",
                "--email", "dev@acme.test", *extra]
        return ip.main(argv, root=self.root, ask=lambda *_: "")

    def test_renames_everything(self):
        self.assertEqual(self.run_init(), 0)
        leftovers = [
            f"{p.relative_to(self.root)}: {name}"
            for p, text in ip.text_files(self.root)
            for name in OLD_NAMES
            if name in text
        ]
        self.assertEqual(leftovers, [])
        self.assertIn("module github.com/acme/6-pack-app/backend", (self.root / "backend/go.mod").read_text())
        self.assertIn("name: six_pack_app", (self.root / "flutter_app/pubspec.yaml").read_text())
        self.assertIn('"six-pack-app"', (self.root / "webapp/wrangler.jsonc").read_text())
        self.assertIn("Acme Ltd", (self.root / "LICENSE").read_text())
        activity = self.root / ip.KOTLIN_DIR / "com/acme/sixpackapp/MainActivity.kt"
        self.assertIn("package com.acme.sixpackapp", activity.read_text())
        self.assertFalse((self.root / ip.KOTLIN_DIR / "com/example").exists())
        self.assertFalse((self.root / "scripts").exists())
        self.assertFalse((self.root / "copier.yml").exists())
        self.assertFalse((self.root / ".github/workflows/template.yml").exists())
        readme = (self.root / "README.md").read_text()
        self.assertNotIn("template-only", readme)
        self.assertNotIn("## Using this template", readme)
        self.assertIn("## Before deploying", readme)

    def test_refuses_to_run_twice(self):
        self.assertEqual(self.run_init("--keep-generator"), 0)
        self.assertEqual(self.run_init(), 1)

    def test_dry_run_writes_nothing(self):
        before = (self.root / "backend/go.mod").read_text()
        self.assertEqual(self.run_init("--dry-run"), 0)
        self.assertEqual((self.root / "backend/go.mod").read_text(), before)

    def test_google_stays_disabled_by_default(self):
        self.assertEqual(self.run_init(), 0)
        self.assertRegex((self.root / "backend/.env.example").read_text(), r"(?m)^GOOGLE_CLIENT_IDS=$")
        self.assertIn("PUBLIC_GOOGLE_SIGNIN=false", (self.root / "webapp/.env.example").read_text())

    def test_google_client_ids_prefill_examples(self):
        web, ios = "111-web.apps.googleusercontent.com", "222-ios.apps.googleusercontent.com"
        self.assertEqual(self.run_init("--google-web-client-id", web, "--google-ios-client-id", ios), 0)
        self.assertRegex((self.root / "backend/.env.example").read_text(), rf"(?m)^GOOGLE_CLIENT_IDS={web},{ios}$")
        self.assertIn("PUBLIC_GOOGLE_SIGNIN=true", (self.root / "webapp/.env.example").read_text())
        cfg = (self.root / "flutter_app/config/app.example.json").read_text()
        self.assertIn(f'"GOOGLE_SERVER_CLIENT_ID": "{web}"', cfg)
        self.assertIn(f'"GOOGLE_IOS_CLIENT_ID": "{ios}"', cfg)
        xc = (self.root / "flutter_app/ios/Flutter/Google.xcconfig").read_text()
        self.assertRegex(xc, rf"(?m)^GOOGLE_IOS_CLIENT_ID = {ios}$")
        self.assertRegex(xc, r"(?m)^GOOGLE_REVERSED_CLIENT_ID = com\.googleusercontent\.apps\.222-ios$")
        self.assertRegex(xc, rf"(?m)^GOOGLE_SERVER_CLIENT_ID = {web}$")

    def test_invalid_google_client_id_is_rejected(self):
        self.assertEqual(self.run_init("--google-web-client-id", "not-a-client-id"), 2)

    # --- landing page ---
    def landing(self):
        return json.loads((self.root / "backend/conf/landing.json").read_text())

    def test_landing_defaults_from_name(self):
        self.assertEqual(self.run_init(), 0)
        data = self.landing()
        self.assertEqual(data["siteName"], "6-Pack App")
        self.assertEqual(data["tagline"], ip.DEFAULT_TAGLINE)
        self.assertEqual(data["url"], "https://example.com/")
        self.assertEqual(data["publisher"]["name"], "Acme Ltd")
        self.assertEqual(data["publisher"]["email"], "dev@acme.test")
        self.assertRegex(data["lastModified"], r"^\d{4}-\d{2}-\d{2}$")
        self.assertIn("com.acme.sixpackapp", data["stores"]["playStoreUrl"])

    def test_landing_and_webapp_use_tagline_and_site_url(self):
        self.assertEqual(self.run_init("--tagline", "Lift smarter, not harder", "--site-url", "https://www.acme.test/"), 0)
        data = self.landing()
        self.assertEqual(data["url"], "https://www.acme.test/")
        self.assertEqual(data["title"], "6-Pack App \u2013 Lift smarter, not harder")
        self.assertEqual(data["hero"]["headline"], "Lift smarter, not harder")
        self.assertEqual(data["publisher"]["url"], "https://www.acme.test")
        self.assertEqual(data["footer"]["links"][0]["href"], "https://app.acme.test")
        seo = (self.root / "webapp/src/lib/seo.ts").read_text()
        self.assertIn("export const SITE_NAME = '6-Pack App';", seo)
        self.assertIn("export const TAGLINE = 'Lift smarter, not harder';", seo)
        self.assertIn("export const WEB_ORIGIN = 'https://app.acme.test';", seo)
        self.assertIn("export const MAIN_SITE_URL = 'https://www.acme.test/';", seo)
        robots = (self.root / "webapp/static/robots.txt").read_text()
        self.assertIn("Sitemap: https://app.acme.test/sitemap.xml", robots)

    def test_tagline_with_apostrophe_is_a_valid_ts_string(self):
        self.assertEqual(self.run_init(), 0)
        self.assertIn('export const TAGLINE = "Your app\'s one-line pitch";', (self.root / "webapp/src/lib/seo.ts").read_text())

    def test_invalid_site_url_is_rejected(self):
        self.assertEqual(self.run_init("--site-url", "http://insecure.test"), 2)

    @unittest.skipUnless(shutil.which("go"), "go not installed")
    def test_backend_landing_tests_pass_after_rename(self):
        self.assertEqual(self.run_init("--tagline", "Lift smarter", "--site-url", "https://acme.test"), 0)
        result = subprocess.run(["go", "test", ".", "./landing"], cwd=self.root / "backend", capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
    # --- /landing page ---

    @unittest.skipUnless(shutil.which("go"), "go not installed")
    def test_backend_still_builds(self):
        self.assertEqual(self.run_init(), 0)
        result = subprocess.run(["go", "build", "./..."], cwd=self.root / "backend", capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stderr)


if __name__ == "__main__":
    unittest.main()
