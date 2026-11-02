#!/usr/bin/env python3
"""Turn a fresh copy of go-rest-template into your own project.

Renames the Go module, app/API display name, Flutter package and bundle id,
Cloudflare worker name, author and contact email, moves the Android
MainActivity to the new package folder and regenerates the swagger docs.

Run it once, from the repository root, right after creating your project:

    python3 scripts/init_project.py                 # interactive prompts
    python3 scripts/init_project.py --name "6-Pack App" --owner lingster --yes

`copier copy` runs it automatically (see copier.yml). Standard library only.
"""

from __future__ import annotations

import argparse
import re
import shutil
import subprocess
import sys
from dataclasses import dataclass, fields
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent

# Values baked into the template. Order of TEMPLATE_* use in replacements() matters:
# longer, more specific strings are replaced before the shorter ones they contain.
TEMPLATE_OWNER = "lingster"
TEMPLATE_SLUG = "go-rest-template"
TEMPLATE_GO_MODULE = f"github.com/{TEMPLATE_OWNER}/{TEMPLATE_SLUG}/backend"
TEMPLATE_DART_PACKAGE = "saas_app"
TEMPLATE_ANDROID_ID = "com.example.saas_app"
TEMPLATE_IOS_ID = "com.example.saasApp"
TEMPLATE_WORKER = "saas-webapp"
TEMPLATE_NAMES = ("Go REST Template", "SaaS Starter")
TEMPLATE_AUTHOR = "Ling LI"
TEMPLATE_EMAIL = "ling@aidraw.ai"

TEMPLATE_ONLY = re.compile(r"<!-- template-only:start -->.*?<!-- template-only:end -->\n?", re.S)
KOTLIN_DIR = Path("flutter_app/android/app/src/main/kotlin")
SKIP_DIRS = {".git", "node_modules", ".dart_tool", "build", ".svelte-kit", ".wrangler", "bin", "data", "__pycache__"}
GENERATOR_FILES = (Path("copier.yml"), Path(".github/workflows/template.yml"), Path("scripts/init_project.py"), Path("scripts/test_init_project.py"))

DIGIT_WORDS = ["zero", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine"]


# ---------------------------------------------------------------- naming rules

def slugify(text: str) -> str:
    return re.sub(r"[^a-z0-9]+", "-", text.lower()).strip("-")


def _lead_digit_to_word(slug: str) -> str:
    """'6-pack-app' -> 'six-pack-app' (identifiers can't start with a digit)."""
    m = re.match(r"(\d)(.*)", slug)
    return f"{DIGIT_WORDS[int(m.group(1))]}{m.group(2)}" if m else slug


def default_dart_package(slug: str) -> str:
    return re.sub(r"[^a-z0-9]+", "_", _lead_digit_to_word(slug)).strip("_")


def default_bundle_id(owner: str, slug: str) -> str:
    org = re.sub(r"[^a-z0-9]", "", _lead_digit_to_word(owner.lower())) or "example"
    app = re.sub(r"[^a-z0-9]", "", _lead_digit_to_word(slug)) or "app"
    return f"com.{org}.{app}"


def default_worker(slug: str) -> str:
    return slugify(_lead_digit_to_word(slug))


GOOGLE_CLIENT_ID_RE = r"^[0-9A-Za-z._-]+\.apps\.googleusercontent\.com$"

VALIDATORS = {
    "slug": (r"^[a-z0-9][a-z0-9-]*$", "lowercase letters, digits and dashes"),
    "owner": (r"^[A-Za-z0-9][A-Za-z0-9-]*$", "a GitHub user or organisation name"),
    "go_module": (r"^[a-z0-9.\-]+(/[A-Za-z0-9._\-]+)+$", "a Go module path, e.g. github.com/you/app/backend"),
    "dart_package": (r"^[a-z][a-z0-9_]*$", "lowercase, start with a letter, letters/digits/underscores"),
    "bundle_id": (r"^[a-z][a-z0-9]*(\.[a-z][a-z0-9]*){2,}$", "reverse-DNS, each part starts with a letter, e.g. com.acme.app"),
    "worker": (r"^[a-z0-9][a-z0-9-]{0,62}$", "lowercase letters, digits and dashes"),
    "email": (r"^[^@\s]+@[^@\s]+\.[^@\s]+$", "an email address"),
    "google_web_client_id": (GOOGLE_CLIENT_ID_RE, "a Google OAuth client id ending in .apps.googleusercontent.com"),
    "google_ios_client_id": (GOOGLE_CLIENT_ID_RE, "a Google OAuth client id ending in .apps.googleusercontent.com"),
}
# --- store release (apple_team_id) ---
VALIDATORS["apple_team_id"] = (r"^[A-Z0-9]{10}$", "a 10-character Apple developer team id, e.g. ABCDE12345")
# --- end store release ---
OPTIONAL = {"google_web_client_id", "google_ios_client_id", "apple_team_id"}


@dataclass
class Settings:
    name: str
    slug: str
    owner: str
    go_module: str
    dart_package: str
    bundle_id: str
    worker: str
    author: str
    email: str
    # Optional "Sign in with Google" client IDs (public identifiers, not secrets). Empty = leave disabled.
    google_web_client_id: str = ""
    google_ios_client_id: str = ""
    # Optional Apple developer team id for the iOS store build (also settable later via the APPLE_TEAM_ID repo variable).
    apple_team_id: str = ""

    def validate(self) -> list[str]:
        errors = []
        for f in fields(self):
            value = getattr(self, f.name)
            if f.name in OPTIONAL and not value.strip():
                continue
            if not value.strip():
                errors.append(f"--{f.name.replace('_', '-')} must not be empty")
            elif f.name in VALIDATORS and not re.match(VALIDATORS[f.name][0], value):
                errors.append(f"--{f.name.replace('_', '-')} '{value}' is invalid: {VALIDATORS[f.name][1]}")
        return errors

    def replacements(self) -> list[tuple[str, str]]:
        pairs = [
            (TEMPLATE_GO_MODULE, self.go_module),
            (f"{TEMPLATE_OWNER}/{TEMPLATE_SLUG}", f"{self.owner}/{self.slug}"),
            (TEMPLATE_SLUG, self.slug),
            *[(n, self.name) for n in TEMPLATE_NAMES],
            (TEMPLATE_WORKER, self.worker),
            (TEMPLATE_ANDROID_ID, self.bundle_id),
            (TEMPLATE_IOS_ID, self.bundle_id),
            (TEMPLATE_DART_PACKAGE, self.dart_package),
            (TEMPLATE_AUTHOR, self.author),
            (TEMPLATE_EMAIL, self.email),
        ]
        return [(old, new) for old, new in pairs if old != new]


# ---------------------------------------------------------------- input

def parse_args(argv: list[str] | None) -> argparse.Namespace:
    p = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    p.add_argument("--name", help='display name, e.g. "6-Pack App"')
    p.add_argument("--slug", help="repo/project slug (default: from name)")
    p.add_argument("--owner", help="GitHub owner (user or org)")
    p.add_argument("--go-module", help="Go module path (default: github.com/<owner>/<slug>/backend)")
    p.add_argument("--dart-package", help="Flutter/Dart package name")
    p.add_argument("--bundle-id", help="Android application id and iOS bundle id")
    p.add_argument("--worker", help="Cloudflare worker name for the webapp")
    p.add_argument("--author", help="copyright holder for LICENSE")
    p.add_argument("--email", help="API contact email shown in swagger")
    p.add_argument("--google-web-client-id", default="",
                   help="Google web OAuth client id; enables Sign in with Google in the generated .env/config examples")
    p.add_argument("--google-ios-client-id", default="", help="Google iOS OAuth client id (optional)")
    p.add_argument("--apple-team-id", default="",
                   help="Apple developer team id (10 chars) baked into tool/fastlane/app_config.rb; optional, "
                        "else set the APPLE_TEAM_ID repo variable")
    p.add_argument("-y", "--yes", action="store_true", help="no prompts: use flags and defaults")
    p.add_argument("--dry-run", action="store_true", help="show what would change, write nothing")
    p.add_argument("--skip-docs", action="store_true", help="don't regenerate swagger (make docs)")
    p.add_argument("--keep-generator", action="store_true", help="keep copier.yml and this script afterwards")
    return p.parse_args(argv)


def resolve(args: argparse.Namespace, ask) -> Settings:
    """Fill each value from flags, else prompt (interactive) or derive a default."""

    def get(key: str, default: str) -> str:
        value = getattr(args, key)
        if value:
            return value
        return default if args.yes else (ask(key.replace("_", " "), default) or default)

    name = get("name", "My SaaS App")
    slug = get("slug", slugify(name))
    owner = get("owner", TEMPLATE_OWNER)
    return Settings(
        name=name,
        slug=slug,
        owner=owner,
        go_module=get("go_module", f"github.com/{owner}/{slug}/backend"),
        dart_package=get("dart_package", default_dart_package(slug)),
        bundle_id=get("bundle_id", default_bundle_id(owner, slug)),
        worker=get("worker", default_worker(slug)),
        author=get("author", TEMPLATE_AUTHOR),
        email=get("email", TEMPLATE_EMAIL),
        google_web_client_id=args.google_web_client_id.strip(),
        google_ios_client_id=args.google_ios_client_id.strip(),
        apple_team_id=args.apple_team_id.strip(),
    )


def prompt(label: str, default: str) -> str:
    return input(f"{label} [{default}]: ").strip()


# ---------------------------------------------------------------- actions

def text_files(root: Path):
    for path in sorted(root.rglob("*")):
        rel = path.relative_to(root)
        if path.is_dir() or any(part in SKIP_DIRS for part in rel.parts) or rel in GENERATOR_FILES:
            continue
        try:
            yield path, path.read_text(encoding="utf-8")
        except (UnicodeDecodeError, OSError):
            continue  # binary or unreadable


def rewrite_files(root: Path, pairs: list[tuple[str, str]], dry_run: bool) -> int:
    changed = 0
    for path, text in text_files(root):
        new = TEMPLATE_ONLY.sub("", text)  # drop "how to use this template" docs
        for old, repl in pairs:
            new = new.replace(old, repl)
        if new != text:
            changed += 1
            if not dry_run:
                path.write_text(new, encoding="utf-8")
    return changed


def reversed_client_id(client_id: str) -> str:
    """'1234-abc.apps.googleusercontent.com' -> 'com.googleusercontent.apps.1234-abc' (iOS URL scheme)."""
    return ".".join(reversed(client_id.split(".")))


def _set_assignment(text: str, key: str, value: str) -> str:
    """Set `KEY=...` / `KEY = ...` (env files and xcconfig) keeping the original separator."""
    def repl(m: re.Match) -> str:
        return m.group(1) + (" " if m.group(1).endswith(" =") else "") + value

    return re.sub(rf"^({re.escape(key)}[ \t]*=)[ \t]*.*$", repl, text, flags=re.M)


def _set_json_string(text: str, key: str, value: str) -> str:
    return re.sub(rf'("{re.escape(key)}": )"[^"]*"', lambda m: f'{m.group(1)}"{value}"', text)


def configure_google(root: Path, s: Settings, dry_run: bool) -> list[str]:
    """Pre-fill the Sign in with Google examples. No-op unless a web client id was given.

    The code ships in every project and stays disabled while GOOGLE_CLIENT_IDS is empty; client IDs are
    public identifiers, so writing them to the *.example files is fine (never put client secrets anywhere).
    """
    web, ios = s.google_web_client_id, s.google_ios_client_id
    if not web:
        return []

    def env_ids(t: str) -> str:
        return _set_assignment(t, "GOOGLE_CLIENT_IDS", ",".join(i for i in (web, ios) if i))

    def app_json(t: str) -> str:
        t = _set_json_string(t, "GOOGLE_SERVER_CLIENT_ID", web)
        return _set_json_string(t, "GOOGLE_IOS_CLIENT_ID", ios) if ios else t

    def xcconfig(t: str) -> str:
        t = _set_assignment(t, "GOOGLE_SERVER_CLIENT_ID", web)
        if ios:
            t = _set_assignment(t, "GOOGLE_IOS_CLIENT_ID", ios)
            t = _set_assignment(t, "GOOGLE_REVERSED_CLIENT_ID", reversed_client_id(ios))
        return t

    edits = {
        "backend/.env.example": env_ids,
        "webapp/.env.example": lambda t: _set_assignment(t, "PUBLIC_GOOGLE_SIGNIN", "true"),
        "flutter_app/config/app.example.json": app_json,
        "flutter_app/ios/Flutter/Google.xcconfig": xcconfig,
    }
    touched = []
    for rel, fn in edits.items():
        path = root / rel
        if not path.exists():
            continue
        text = path.read_text(encoding="utf-8")
        new = fn(text)
        if new != text:
            touched.append(rel)
            if not dry_run:
                path.write_text(new, encoding="utf-8")
    return touched


# --- store release ---------------------------------------------------------------
APP_CONFIG = Path("tool/fastlane/app_config.rb")
STORE_NAME_LIMIT = 30  # App Store / Play title limit (tool/fastlane/listing.rb)


def configure_apple(root: Path, s: Settings, dry_run: bool) -> bool:
    """Bake the optional Apple team id into the one app-config file of the release tooling."""
    path = root / APP_CONFIG
    if not s.apple_team_id or not path.exists():
        return False
    text = path.read_text(encoding="utf-8")
    new = re.sub(r'(DEFAULT_APPLE_TEAM_ID = )"[^"]*"', lambda m: f'{m.group(1)}"{s.apple_team_id}"', text)
    if new != text and not dry_run:
        path.write_text(new, encoding="utf-8")
    return new != text


def store_warnings(s: Settings) -> list[str]:
    """Listing files use the app name as the store title, which both stores cap at 30 characters."""
    if len(s.name) > STORE_NAME_LIMIT:
        return [f"app name is {len(s.name)} characters; store titles allow {STORE_NAME_LIMIT}. "
                "Shorten flutter_app/**/name.txt and title.txt (tool/fastlane/listing_test.rb checks it)."]
    return []
# --- end store release -----------------------------------------------------------


def move_main_activity(root: Path, bundle_id: str, dry_run: bool) -> Path | None:
    kotlin = root / KOTLIN_DIR
    sources = list(kotlin.rglob("MainActivity.kt")) if kotlin.exists() else []
    if not sources:
        return None
    target_dir = kotlin.joinpath(*bundle_id.split("."))
    if sources[0].parent == target_dir or dry_run:
        return target_dir
    target_dir.mkdir(parents=True, exist_ok=True)
    shutil.move(str(sources[0]), target_dir / "MainActivity.kt")
    # remove now-empty package folders left behind
    for d in sorted((p for p in kotlin.rglob("*") if p.is_dir()), key=lambda p: len(p.parts), reverse=True):
        if not any(d.iterdir()):
            d.rmdir()
    return target_dir


def regenerate_docs(root: Path) -> str:
    if not shutil.which("bee") or not shutil.which("make"):
        return "skipped (install bee: go install github.com/beego/bee/v2@latest, then run `make docs` in backend/)"
    result = subprocess.run(["make", "docs"], cwd=root / "backend", capture_output=True, text=True)
    return "regenerated" if result.returncode == 0 else f"FAILED, run `make docs` in backend/:\n{result.stderr[-800:]}"


def remove_generator(root: Path) -> None:
    for rel in GENERATOR_FILES:
        (root / rel).unlink(missing_ok=True)
    scripts = root / "scripts"
    if scripts.exists() and not any(p for p in scripts.iterdir() if p.name != "__pycache__"):
        shutil.rmtree(scripts)


def already_initialised(root: Path) -> bool:
    go_mod = root / "backend" / "go.mod"
    return go_mod.exists() and TEMPLATE_GO_MODULE not in go_mod.read_text()


def main(argv: list[str] | None = None, root: Path = ROOT, ask=prompt) -> int:
    args = parse_args(argv)
    if already_initialised(root):
        print("This project is already initialised (backend/go.mod no longer uses the template module).")
        return 1

    settings = resolve(args, ask)
    errors = settings.validate()
    if errors:
        print("Invalid settings:\n  " + "\n  ".join(errors), file=sys.stderr)
        return 2

    print("Initialising project with:")
    for f in fields(settings):
        print(f"  {f.name:<13} {getattr(settings, f.name)}")
    if not args.yes and not args.dry_run and ask("proceed? (y/n)", "y").lower() not in ("y", "yes"):
        print("Aborted.")
        return 1

    count = rewrite_files(root, settings.replacements(), args.dry_run)
    google_files = configure_google(root, settings, args.dry_run)
    apple = configure_apple(root, settings, args.dry_run)
    activity = move_main_activity(root, settings.bundle_id, args.dry_run)
    verb = "Would update" if args.dry_run else "Updated"
    print(f"{verb} {count} files.")
    if google_files:
        print("Sign in with Google pre-configured in: " + ", ".join(google_files))
    if apple:
        print(f"Apple team id set in {APP_CONFIG}")
    for warning in store_warnings(settings):
        print(f"Warning: {warning}")
    if activity:
        print(f"MainActivity.kt -> {activity.relative_to(root)}")
    if args.dry_run:
        return 0

    if not args.skip_docs:
        print(f"Swagger docs: {regenerate_docs(root)}")
    if not args.keep_generator:
        remove_generator(root)

    print(
        "\nDone. Next steps:\n"
        "  1. Set JWT_SECRET, WEBAUTHN_RP_ID and WEBAUTHN_RP_ORIGINS for each environment.\n"
        "  2. Add CLOUDFLARE_API_TOKEN / CLOUDFLARE_ACCOUNT_ID secrets and PUBLIC_API_URL / API_BASE_URL variables.\n"
        "  3. Optional: Sign in with Google (disabled until GOOGLE_CLIENT_IDS is set), see README.\n"
        "  4. Store release (optional): edit the listing placeholders and see docs/STORES.md (README: ASO).\n"
        "  5. git add -A && git commit -m 'chore: initialise project from template'"
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
