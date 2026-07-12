#!/usr/bin/env python3
# Sprint 123 — two outbound-security gates that freeze known-bad classes so a
# regression is red in CI instead of found by a live audit six weeks later.
#
# G6 — RedisClientOpt without Password (hard rule):
#   The NOAUTH class (S122-B2): an Asynq/Redis client built from a parsed Redis
#   option variable but missing `Password` silently fails against the shipped
#   --requirepass Redis. Any `RedisClientOpt{ Addr: <expr> }` whose Addr is a
#   non-string-literal (a variable/field — i.e. the production path) and which
#   lacks `Password:` fails, unless it carries an inline `// redisauth-ok:` note.
#   String-literal Addr ("localhost:6379") is an allowed dev fallback; an empty
#   `RedisClientOpt{}` is the documented "no redis" zero value.
#
# G5 — http.Client literals in services/** and platform/** (ratchet):
#   Every outbound request should ideally go through httputil.GuardedClient/
#   GuardedDialContext (DNS-rebinding / SSRF re-validation). The existing
#   population targets mostly fixed vendor hosts; migrating them is SA15-01/02
#   (S124/S125). Until then this gate freezes the count of *unguarded* http.Client
#   literals and only lets it go DOWN — a NEW unguarded client fails CI.
#
# Exit non-zero on any violation.

import re
import sys
import pathlib

ROOT = pathlib.Path(__file__).resolve().parent.parent
BACKEND = ROOT / "backend"

# ── G5 ratchet baseline ──────────────────────────────────────────────────────
# Count of unguarded `http.Client{` literals under services/** + platform/**.
# Only ever lower this — never raise to admit a new unguarded client; use
# httputil.GuardedClient instead. 14 at S123-G5; lowered to 12 in S124-2 after
# alerting + siem forwarder moved to GuardedClient (SA15-01).
HTTPCLIENT_BASELINE = 12

REDIS_RE = re.compile(r"RedisClientOpt\{")
HTTPCLIENT_RE = re.compile(r"\bhttp\.Client\{")
STRING_ADDR_RE = re.compile(r'Addr:\s*"')


def go_prod_files(*subdirs):
    for sub in subdirs:
        base = BACKEND / sub
        if not base.exists():
            continue
        for p in base.rglob("*.go"):
            if p.name.endswith("_test.go"):
                continue
            yield p


def balanced_block(text, open_idx):
    """Return the substring of the {...} block starting at the '{' at open_idx."""
    depth = 0
    for i in range(open_idx, len(text)):
        if text[i] == "{":
            depth += 1
        elif text[i] == "}":
            depth -= 1
            if depth == 0:
                return text[open_idx : i + 1]
    return text[open_idx:]


def check_redis():
    violations = []
    # Scan all production Go (Redis clients live in cmd/ and internal/).
    for p in list(go_prod_files("internal")) + [
        f for f in (BACKEND / "cmd").rglob("*.go") if not f.name.endswith("_test.go")
    ]:
        text = p.read_text(encoding="utf-8", errors="ignore")
        lines = text.splitlines()
        for m in REDIS_RE.finditer(text):
            block = balanced_block(text, m.end() - 1)
            if "Addr:" not in block:
                continue  # empty zero-value {} — the documented "no redis" case
            if "Password:" in block:
                continue  # correctly authenticated
            if STRING_ADDR_RE.search(block):
                continue  # string-literal Addr → dev localhost fallback
            # Locate the line for an escape-hatch comment / message. Look a few
            # lines up (a multi-line justification comment sits above the literal).
            line_no = text[: m.start()].count("\n") + 1
            window = "\n".join(lines[max(0, line_no - 6) : line_no + 1])
            if "redisauth-ok" in window:
                continue
            violations.append(f"{p.relative_to(ROOT)}:{line_no}: RedisClientOpt with a "
                              f"variable Addr but no Password (NOAUTH class). Add "
                              f"`Password: ...` or an inline `// redisauth-ok: <reason>`.")
    return violations


def check_httpclient():
    count = 0
    hits = []
    for p in go_prod_files("internal/services", "internal/shared/platform"):
        text = p.read_text(encoding="utf-8", errors="ignore")
        for m in HTTPCLIENT_RE.finditer(text):
            block = balanced_block(text, m.end() - 1)
            # A client is "guarded" when its transport/dialer comes from httputil.
            if "Guarded" in block or "httputil." in block:
                continue
            count += 1
            line_no = text[: m.start()].count("\n") + 1
            hits.append(f"{p.relative_to(ROOT)}:{line_no}")
    return count, hits


def main():
    problems = []

    redis_violations = check_redis()
    problems.extend(redis_violations)

    count, hits = check_httpclient()
    if count > HTTPCLIENT_BASELINE:
        problems.append(
            f"G5: {count} unguarded http.Client literals in services/**+platform/** "
            f"(baseline {HTTPCLIENT_BASELINE}). A new one was added — route it through "
            f"httputil.GuardedClient/GuardedDialContext.\n  " + "\n  ".join(hits))
    elif count < HTTPCLIENT_BASELINE:
        print(f"note: unguarded http.Client count dropped to {count} "
              f"(baseline {HTTPCLIENT_BASELINE}). Lower HTTPCLIENT_BASELINE in "
              f"check_outbound_security.py to lock in the improvement.")

    if problems:
        print("❌ Outbound-security gate failed:\n")
        for p in problems:
            print(" - " + p)
        sys.exit(1)

    print(f"✓ Outbound-security OK — Redis clients authenticated; "
          f"{count}/{HTTPCLIENT_BASELINE} unguarded http.Client literals (frozen).")


if __name__ == "__main__":
    main()
