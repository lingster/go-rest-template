"""Tests for init_project.py: run it on a copy of the repo and check the result.

    python3 -m unittest discover -s scripts -v
"""

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

    @unittest.skipUnless(shutil.which("go"), "go not installed")
    def test_backend_still_builds(self):
        self.assertEqual(self.run_init(), 0)
        result = subprocess.run(["go", "build", "./..."], cwd=self.root / "backend", capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stderr)


if __name__ == "__main__":
    unittest.main()
