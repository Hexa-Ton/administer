#!/usr/bin/env python3
"""
administer wordlist tools — validation, deduplication, and category
statistics for administer wordlists. Standard library only.

Invoked by the Go CLI's `administer wordlist` subcommands. It is
never on the live scan's request path, so scanning works with zero
Python dependency; `administer doctor` reports whether python3 is
available, and only these subcommands are affected if it is not.
"""
import argparse
import hashlib
import json
import re
import sys

MANDATORY_SIX = ["/admin", "/login", "/admin/login", "/login/admin", "/owner", "/master"]

CATEGORY_PATTERNS = [
    ("admin_panel", re.compile(r"(admin|dashboard|control[-_]?panel|manage)", re.I)),
    ("login_auth", re.compile(r"(login|signin|sign-in|auth|session)", re.I)),
    ("api", re.compile(r"(/api/|graphql|/v[0-9]+/)", re.I)),
    ("config_backup", re.compile(r"(\.env|\.git|\.bak$|\.old$|\.sql$|\.zip$|backup)", re.I)),
    ("cms", re.compile(r"(wp-|wordpress|joomla|drupal|magento)", re.I)),
    ("devops_cloud", re.compile(r"(actuator|health|metrics|\.well-known)", re.I)),
]


def load_lines(path):
    with open(path, "r", encoding="utf-8", errors="replace") as f:
        return [line.rstrip("\n") for line in f]


def clean(raw):
    seen = set()
    cleaned = []
    skipped_blank = 0
    skipped_comment = 0
    skipped_dupe = 0
    for line in raw:
        s = line.strip()
        if not s:
            skipped_blank += 1
            continue
        if s.startswith("#"):
            skipped_comment += 1
            continue
        if not s.startswith("/"):
            s = "/" + s
        if s in seen:
            skipped_dupe += 1
            continue
        seen.add(s)
        cleaned.append(s)
    return cleaned, {
        "skipped_blank": skipped_blank,
        "skipped_comment": skipped_comment,
        "skipped_duplicate": skipped_dupe,
    }


def categorize(paths):
    counts = {name: 0 for name, _ in CATEGORY_PATTERNS}
    counts["uncategorized"] = 0
    for p in paths:
        matched = False
        for name, pattern in CATEGORY_PATTERNS:
            if pattern.search(p):
                counts[name] += 1
                matched = True
                break
        if not matched:
            counts["uncategorized"] += 1
    return counts


def check_mandatory_six(paths):
    if len(paths) < 6:
        return False, "fewer than 6 entries"
    for i, want in enumerate(MANDATORY_SIX):
        if paths[i] != want:
            return False, f"position {i + 1} is {paths[i]!r}, want {want!r}"
    return True, "ok"


def cmd_validate(args):
    raw = load_lines(args.path)
    cleaned, stats = clean(raw)
    ok, detail = check_mandatory_six(cleaned)
    result = {
        "path": args.path,
        "raw_lines": len(raw),
        "unique_paths": len(cleaned),
        "mandatory_six_ok": ok,
        "mandatory_six_detail": detail,
        **stats,
    }
    print(json.dumps(result, indent=2))
    sys.exit(0 if ok else 2)


def cmd_stats(args):
    raw = load_lines(args.path)
    cleaned, stats = clean(raw)
    categories = categorize(cleaned)
    digest = hashlib.sha256("\n".join(cleaned).encode("utf-8")).hexdigest()
    result = {
        "path": args.path,
        "unique_paths": len(cleaned),
        "sha256": digest,
        "categories": categories,
        **stats,
    }
    print(json.dumps(result, indent=2))


def main():
    parser = argparse.ArgumentParser(description="administer wordlist tools")
    sub = parser.add_subparsers(dest="command", required=True)

    p_validate = sub.add_parser("validate", help="validate a wordlist and check mandatory-six order")
    p_validate.add_argument("path")
    p_validate.set_defaults(func=cmd_validate)

    p_stats = sub.add_parser("stats", help="dedupe, categorize, and hash a wordlist")
    p_stats.add_argument("path")
    p_stats.set_defaults(func=cmd_stats)

    args = parser.parse_args()
    args.func(args)


if __name__ == "__main__":
    main()
