#!/usr/bin/env python3
"""G-CSRF — every authenticated frontend WRITE must go through apiFetch.

Why this gate exists (R1-18-D4, Codeaudit v5b)
----------------------------------------------
Seven authenticated writes used a raw ``fetch()`` instead of ``apiFetch``,
almost all of them multipart uploads, because ``apiFetch`` used to force
``Content-Type: application/json`` and that destroys the multipart boundary.
A raw ``fetch()`` carries no ``X-CSRF-Token``, so the double-submit check
rejected every one of them with 403: evidence upload, evidence files, supplier
CSV import, AI report, findings import, asset import and asset CSV import were
unusable from the day they were written, not merely insecure.

A fix at those seven sites is not a fix of the class. This gate is.

What it checks
--------------
Every raw ``fetch(...)`` call in ``frontend/src`` whose HTTP method is a write
(POST/PUT/PATCH/DELETE) must either

  * set ``X-CSRF-Token`` in its ``headers`` position — correct, but a
    hand-rolled copy of what ``apiFetch`` does, so it is counted and reported
    rather than failed. The check is scoped to ``headers`` on purpose: a
    search over the whole options text would let a request that sends no
    headers at all pass by merely naming the token in a body or error string,
    and would then count it as something it had verified, or
  * appear in ALLOWLIST below with a reason (public / token-in-URL endpoints
    that have no session and therefore no CSRF cookie).

Anything else fails, **by name**, with file, line and URL.

What it deliberately does NOT check — read this before trusting a green run
--------------------------------------------------------------------------
* ``apiFetch`` calls. They are the correct path; their CSRF handling is
  covered by ``frontend/src/api/client.test.ts`` and
  ``client.formdata.test.ts``, not by a static scan.
* GET/HEAD/OPTIONS requests. The backend's CSRF middleware ignores safe
  methods, so a raw read is not this defect class. Reads are counted, not
  judged. A GET route later upgraded to mutate state is outside this gate.
* Backend-side CSRF enforcement. This gate reads the frontend only.
* Anything outside ``frontend/src`` (e2e specs, marketing sites, scripts).
* Test files (``*.test.*`` / ``*.spec.*``): they mock ``fetch`` on purpose.
  Their count is printed as ``excluded`` so the omission is visible.
* Writes issued through an application-level *wrapper* that takes a
  ``RequestInit`` and forwards it. ``shared/utils/downloadBlob.ts`` is the only
  one; it now attaches the token itself for non-safe methods, so its own
  ``fetch`` clears the gate on the CSRF branch rather than an exemption. A
  future wrapper of the same shape would be invisible here — the scanner sees
  the wrapper's single ``fetch``, not its call sites.
* Whether an endpoint is genuinely public. The ALLOWLIST reasons were each
  checked against the backend mount by hand; nothing re-verifies them, so a
  route later moved from ``api`` to ``protected`` would keep a stale pass.
* Evasion by indirection the scanner cannot follow.

  DETECTED (each verified by writing the construct and running the gate —
  reading the regex is not evidence): direct ``fetch``;
  ``window``/``globalThis``/``self``-qualified ``fetch``; aliases via
  ``const f = fetch``, a bare ``f = fetch`` assignment, and
  ``const {fetch: ff} = window``; any of those called with optional chaining
  (``f?.(...)``, ``fetch?.(...)``); ``fetch.call``/``.apply``/``.bind``;
  ``navigator.sendBeacon``; a hoisted ``const r = new Request(...); fetch(r)``;
  a top-level computed key ``{['method']: 'POST'}``. ``XMLHttpRequest`` and
  ``axios`` are reported as unreadable rather than ignored.

  NOT DETECTED: a fetch reference stored on an object property
  (``const api = {go: window.fetch}; api.go(...)``) or passed as a function
  argument; an alias assigned across module boundaries; ``new Proxy(fetch,
  {})``; a URL built so the endpoint cannot be read. None of these exist in
  the codebase today.

  The list above is the gate's actual reach, and it is written out precisely
  so it can be falsified. That has now paid for itself twice: an earlier
  version promised in prose that four constructs "must NOT resolve to GET"
  while one line of hoisting defeated one of them, and this list itself
  claimed ``const f = fetch`` was detected while a single ``?.`` walked past
  it. A claim someone can disprove is worth more than a prose assurance
  nobody can check — but only if it is corrected when the check lands a hit.

Every call the parser could not resolve is counted and listed as ``skipped``.
A gate that silently drops input it cannot parse reports success for work it
did not do — ``check_routes.py`` did exactly that for a quarter of the
frontend's calls, and that is where the drift lived. A skipped write is not
tolerated: if the method cannot be resolved, the call is reported as a failure
rather than waved through, because "I cannot tell" on a write is the blind spot
this gate exists to close.

Exit codes: 0 = clean, 1 = violations found.
"""

from __future__ import annotations

import re
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parent.parent
SCAN_ROOT = "frontend/src"
SOURCE_SUFFIXES = (".ts", ".tsx")
TEST_MARKERS = (".test.", ".spec.")

WRITE_METHODS = {"POST", "PUT", "PATCH", "DELETE"}

# The one file that is allowed to call fetch() with a caller-controlled method:
# it *is* apiFetch. Judging the implementation of the correct path by the rule
# it enforces is circular. Named here rather than skipped silently, and its
# own CSRF behaviour is covered by client.test.ts / client.formdata.test.ts.
APIFETCH_IMPL = "frontend/src/api/client.ts"

# ── Allowlist ────────────────────────────────────────────────────────────────
# Raw-fetch writes that legitimately carry no CSRF header. Keyed by
# (file path relative to repo root, URL expression exactly as written in the
# source). Every entry needs a reason; an entry without one is a hole.
#
# The common case: endpoints reached by someone who has no Vakt session at all
# — an external supplier, an auditor, a data subject following a mailed link.
# They authenticate with a token in the path, there is no csrf_token cookie to
# double-submit, and routing them through apiFetch would add a header the
# backend does not check and trigger a redirect to /login on 401.
ALLOWLIST: dict[tuple[str, str], str] = {
    # Public NIS2 self-assessment wizard — no login, mounted on a public group.
    (
        "frontend/src/pages/NIS2WizardPage.tsx",
        "'/api/v1/public/nis2-assessment/start'",
    ): "public wizard, no session exists",
    (
        "frontend/src/pages/NIS2WizardPage.tsx",
        "'/api/v1/public/nis2-assessment/answer'",
    ): "public wizard, no session exists",
    # Demo feedback widget. POST /feedback is mounted on the `api` group with
    # no auth middleware (cmd/api/routes.go:753, demo builds only); CSRF lives
    # on `protected`, so there is nothing to double-submit.
    (
        "frontend/src/shared/components/FeedbackWidget.tsx",
        "'/api/v1/feedback'",
    ): "public demo-only endpoint, mounted outside the CSRF group",
    # Supplier portal — external supplier authenticated by the token in the URL.
    (
        "frontend/src/pages/SupplierPortalPage.tsx",
        "`/api/v1/vaktcomply/supplier/${token}/save`",
    ): "token-in-URL portal, no session cookie",
    (
        "frontend/src/pages/SupplierPortalPage.tsx",
        "`/api/v1/vaktcomply/supplier/${token}/submit`",
    ): "token-in-URL portal, no session cookie",
    (
        "frontend/src/pages/SupplierPortalPage.tsx",
        "`/api/v1/vaktcomply/supplier/${token}/upload`",
    ): "token-in-URL portal, no session cookie",
    # DSR portal — data subject following a public link, no session.
    (
        "frontend/src/pages/DSRPortalPage.tsx",
        "`/api/v1/vaktprivacy/dsr-portal/${slug}/submit`",
    ): "public DSR intake, no session cookie",
    # Auditor invite acceptance — token in URL, runs before any session exists.
    (
        "frontend/src/pages/AuditorAcceptPage.tsx",
        "`/api/v1/auditor/accept/${token}`",
    ): "token-in-URL invite acceptance, runs pre-session",
    # Policy acceptance link mailed to employees — token in URL, no session.
    (
        "frontend/src/modules/vaktcomply/hooks/usePolicyAcceptance.ts",
        "`/api/v1/policy-accept/${token}`",
    ): "token-in-URL policy acceptance, no session cookie",
    # Crash reporter: must still work when the session is already gone, and
    # must never redirect on 401 the way apiFetch does.
    (
        "frontend/src/shared/components/ErrorBoundary.tsx",
        "'/api/v1/errors'",
    ): "crash reporter, must survive a dead session and never redirect",
}

# ── Source scanning ──────────────────────────────────────────────────────────

# Ways to reach the network. Bare `fetch(`, `window.fetch(`/`globalThis.fetch(`/
# `self.fetch(`, and any local alias (`const f = fetch`). A bare
# `(?<![\w$.])fetch` alone excludes member access — which correctly rejects
# `res.fetch` but also let `window.fetch` through unseen.
# `(?:\?\.)?` on every call form: an optional-chained call `fetch?.(url, ...)`
# is a call like any other, and without it a single `?.` walked past the
# pattern silently.
FETCH_RE = re.compile(
    r"(?:(?<![\w$.])|(?<=\bwindow\.)|(?<=\bglobalThis\.)|(?<=\bself\.))fetch\s*(?:\?\.)?\("
)
# Any binding of fetch to another name: `const f = fetch`, a bare `f = fetch`
# assignment with the declaration elsewhere, and `const {fetch: ff} = window`
# destructuring. Requiring a declarator keyword missed the latter two.
ALIAS_RE = re.compile(
    r"\b(?:const|let|var\s+)?([A-Za-z_$][\w$]*)\s*=\s*(?:window\.|globalThis\.|self\.)?fetch\b(?!\s*[.(])"
)
ALIAS_DESTRUCTURE_RE = re.compile(r"\{[^{}]*\bfetch\s*:\s*([A-Za-z_$][\w$]*)[^{}]*\}\s*=")
# fetch.call/.apply/.bind reach the network with the arguments shifted or
# wrapped in an array, so the positional parse below cannot read them. They
# have no legitimate use here and are reported rather than parsed.
FETCH_REFLECT_RE = re.compile(r"\bfetch\s*\.\s*(call|apply|bind)\s*\(")
# sendBeacon is ALWAYS a POST and the API accepts no headers at all, so it can
# never carry a CSRF token. XMLHttpRequest and axios are the other two ways to
# write without touching `fetch`. None of the three exist in the codebase today;
# they are detected so that stays true.
BEACON_RE = re.compile(r"\bnavigator\s*\.\s*sendBeacon\s*\(")
OTHER_CLIENT_RE = re.compile(r"\bnew\s+XMLHttpRequest\s*\(|\baxios\s*\.")

# The method is read from the TOP-LEVEL properties of the options literal
# (see resolve_method), not by searching the whole argument text: a nested
# `method:` in a body object, or a nested computed key, says nothing about
# the request's method and used to decide it.
REQUEST_CTOR_RE = re.compile(r"\bnew\s+Request\s*\(")
# A first argument that is neither a string nor a template literal may be a
# Request object carrying its own method — `const r = new Request(u, {...});
# fetch(r)` defeated the new-Request check by one line of hoisting.
LITERAL_URL_RE = re.compile(r"""^\s*['"`]""")
CSRF_RE = re.compile(r"X-CSRF-Token", re.IGNORECASE)

# `headers: someVar` or the `headers,` shorthand. The header object is often
# built a few lines above the call (useAIStream, useAgentRun do exactly this),
# so scanning only the text between fetch's parentheses reports a correct call
# as a violation. A gate that cries wolf on healthy code gets switched off, so
# the identifier is resolved against the file it lives in.
HEADERS_IDENT_RE = re.compile(r"\bheaders\s*:\s*([A-Za-z_$][\w$]*)")
HEADERS_SHORTHAND_RE = re.compile(r"(?:^|[{,\s])headers\s*(?=[,}])")


def strip_comments(src: str) -> str:
    """Blank out comments, preserving offsets (and therefore line numbers).

    String and template-literal contents are kept — the URL lives there — so
    the scanner must know when it is inside one, otherwise a "//" in a URL
    would start a comment and swallow the rest of the line.
    """
    out = list(src)
    i, n = 0, len(src)
    state = "code"
    template_depth = 0  # nesting of ${ ... } inside template literals
    while i < n:
        ch = src[i]
        nxt = src[i + 1] if i + 1 < n else ""
        if state == "code":
            if ch == "/" and nxt == "/":
                state = "line_comment"
                out[i] = out[i + 1] = " "
                i += 2
                continue
            if ch == "/" and nxt == "*":
                state = "block_comment"
                out[i] = out[i + 1] = " "
                i += 2
                continue
            if ch == "'":
                state = "single"
            elif ch == '"':
                state = "double"
            elif ch == "`":
                state = "template"
        elif state == "line_comment":
            if ch == "\n":
                state = "code"
            else:
                out[i] = " "
        elif state == "block_comment":
            if ch == "*" and nxt == "/":
                out[i] = out[i + 1] = " "
                i += 2
                state = "code"
                continue
            if ch != "\n":
                out[i] = " "
        elif state in ("single", "double"):
            if ch == "\\":
                i += 2
                continue
            if (state == "single" and ch == "'") or (state == "double" and ch == '"'):
                state = "code"
        elif state == "template":
            if ch == "\\":
                i += 2
                continue
            if ch == "`":
                state = "code"
            elif ch == "$" and nxt == "{":
                template_depth += 1
                i += 2
                state = "template_expr"
                continue
        elif state == "template_expr":
            # Inside ${ ... }. Comments are legal here but vanishingly rare;
            # what matters is finding the closing brace without being fooled by
            # nested braces.
            if ch == "{":
                template_depth += 1
            elif ch == "}":
                template_depth -= 1
                if template_depth == 0:
                    state = "template"
        i += 1
    return "".join(out)


def match_paren(src: str, open_idx: int) -> int | None:
    """Index of the ')' closing the '(' at open_idx, or None if unbalanced.

    String- and template-aware so a ')' inside a URL does not close the call.
    """
    depth = 0
    i, n = open_idx, len(src)
    state = "code"
    tpl_depth = 0
    while i < n:
        ch = src[i]
        nxt = src[i + 1] if i + 1 < n else ""
        if state == "code":
            if ch in "([{":
                depth += 1
            elif ch in ")]}":
                depth -= 1
                if depth == 0 and ch == ")":
                    return i
            elif ch == "'":
                state = "single"
            elif ch == '"':
                state = "double"
            elif ch == "`":
                state = "template"
        elif state in ("single", "double"):
            if ch == "\\":
                i += 2
                continue
            if (state == "single" and ch == "'") or (state == "double" and ch == '"'):
                state = "code"
        elif state == "template":
            if ch == "\\":
                i += 2
                continue
            if ch == "`":
                state = "code"
            elif ch == "$" and nxt == "{":
                tpl_depth = 1
                i += 2
                state = "template_expr"
                continue
        elif state == "template_expr":
            if ch == "{":
                tpl_depth += 1
            elif ch == "}":
                tpl_depth -= 1
                if tpl_depth == 0:
                    state = "template"
        i += 1
    return None


def split_top_level_args(arglist: str) -> list[str]:
    """Split a call's argument text on top-level commas."""
    args: list[str] = []
    depth = 0
    state = "code"
    tpl_depth = 0
    current: list[str] = []
    i, n = 0, len(arglist)
    while i < n:
        ch = arglist[i]
        nxt = arglist[i + 1] if i + 1 < n else ""
        if state == "code":
            if ch in "([{":
                depth += 1
            elif ch in ")]}":
                depth -= 1
            elif ch == "," and depth == 0:
                args.append("".join(current).strip())
                current = []
                i += 1
                continue
            elif ch == "'":
                state = "single"
            elif ch == '"':
                state = "double"
            elif ch == "`":
                state = "template"
        elif state in ("single", "double"):
            if ch == "\\":
                current.append(ch)
                current.append(nxt)
                i += 2
                continue
            if (state == "single" and ch == "'") or (state == "double" and ch == '"'):
                state = "code"
        elif state == "template":
            if ch == "\\":
                current.append(ch)
                current.append(nxt)
                i += 2
                continue
            if ch == "`":
                state = "code"
            elif ch == "$" and nxt == "{":
                tpl_depth = 1
                current.append(ch)
                current.append(nxt)
                i += 2
                state = "template_expr"
                continue
        elif state == "template_expr":
            if ch == "{":
                tpl_depth += 1
            elif ch == "}":
                tpl_depth -= 1
                if tpl_depth == 0:
                    state = "template"
        current.append(ch)
        i += 1
    tail = "".join(current).strip()
    if tail:
        args.append(tail)
    return args


def object_properties(literal: str) -> list[str] | None:
    """Top-level properties of an object literal, or None if it is not one."""
    text = literal.strip()
    if not (text.startswith("{") and text.endswith("}")):
        return None
    return split_top_level_args(text[1:-1])


def headers_expr(options: str) -> str | None:
    """The text of the options' ``headers`` value, or None when there is none.

    The CSRF check MUST be scoped to this expression. Searching the whole
    options text for the header name lets a request with no headers at all
    pass merely by naming it in a body literal —

        body: JSON.stringify({err: 'X-CSRF-Token missing'})

    — which an error-handling or i18n file could contain quite innocently.
    That is worse than a skip: the gate makes a positive false statement in
    the safe direction and increments its "manual CSRF" counter as though it
    had seen something correct.
    """
    props = object_properties(options)
    if props is None:
        return None
    for prop in props:
        text = prop.strip()
        if re.match(r"^headers\s*:", text):
            return text.split(":", 1)[1].strip()
        if re.match(r"^headers\s*$", text):
            return "headers"  # { headers } shorthand
    return None


def has_csrf_header(options: str, file_src: str) -> bool:
    """True only when the token is set in the request's headers position."""
    expr = headers_expr(options)
    if expr is None:
        return False
    if CSRF_RE.search(expr):
        return True
    return csrf_via_variable(expr, file_src)


def csrf_via_variable(options: str, file_src: str) -> bool:
    """True when the headers expression is a variable given a CSRF token.

    Resolution is per identifier, not per file: a file may build one headers
    object with the token and another without, and only the first should count.
    """
    idents = set(HEADERS_IDENT_RE.findall(options))
    idents.update(re.findall(r"^\s*([A-Za-z_$][\w$]*)\s*$", options))
    if HEADERS_SHORTHAND_RE.search(options):
        idents.add("headers")
    for ident in idents:
        esc = re.escape(ident)
        # `h['X-CSRF-Token'] = ...` / `h["X-CSRF-Token"] = ...`
        if re.search(rf"\b{esc}\s*\[\s*['\"]X-CSRF-Token['\"]\s*\]\s*=", file_src, re.IGNORECASE):
            return True
        # `const h: Record<string,string> = { ..., 'X-CSRF-Token': t }`
        decl = re.search(rf"\b{esc}\b[^=;\n]*=\s*\{{", file_src)
        if decl:
            end = match_paren_like(file_src, file_src.index("{", decl.start()))
            if end and CSRF_RE.search(file_src[decl.start() : end]):
                return True
    return False


def match_paren_like(src: str, open_idx: int) -> int | None:
    """Index just past the '}' closing the '{' at open_idx."""
    depth = 0
    for i in range(open_idx, len(src)):
        if src[i] == "{":
            depth += 1
        elif src[i] == "}":
            depth -= 1
            if depth == 0:
                return i + 1
    return None


def resolve_method(url_arg: str, options: str) -> str | None:
    """The request's HTTP method, or None when it cannot be proven.

    None means "cannot prove this is not a write" and is treated as a
    violation, never as a GET. Defaulting an unreadable call to GET is the
    failure mode this whole gate exists to prevent: it produces a silent
    misclassification that ``skipped: 0`` then certifies as full coverage.

    Six constructs must NOT resolve to GET:
      * ``fetch(new Request(url, {method: 'POST'}))`` — method is in arg 1.
      * ``const r = new Request(...); fetch(r)`` — same, hoisted one line out
        of sight. Any non-literal single argument may be a Request.
      * ``fetch(url, opts)`` — options are an identifier, contents unknown.
      * ``fetch(url, {...base})`` — a spread can carry method from elsewhere.
      * ``fetch(url, {method, ...})`` — shorthand, no colon to match on.
      * ``fetch(url, {['method']: 'POST'})`` — computed key hides the name.
    """
    if REQUEST_CTOR_RE.search(url_arg):
        return None

    if not options:
        # No second argument: a literal URL really is a GET, but a bare
        # identifier could be a Request object built anywhere.
        return "GET" if LITERAL_URL_RE.match(url_arg) else None

    props = object_properties(options)
    if props is None:
        # An identifier, call or ternary — we cannot see inside it.
        return None

    # Everything below is scoped to TOP-LEVEL properties. Only a top-level
    # `method` can set the method, and only a top-level spread or computed key
    # can hide one — searching the whole options text turned a nested computed
    # key (`headers: {[k]: v}`) on a perfectly ordinary GET into a violation.
    # A gate that reddens healthy code gets switched off rather than fixed.
    method_value: str | None = None
    unreadable = False
    for prop in props:
        text = prop.strip()
        if text.startswith("...") or text.startswith("["):
            unreadable = True
            continue
        keyed = re.match(r"^method\s*:\s*(.*)$", text, re.S)
        if keyed:
            method_value = keyed.group(1).strip()
            continue
        if re.match(r"^method\s*$", text):
            return None  # { method } shorthand — value lives elsewhere

    if method_value is not None:
        lit = re.match(r"""^['"`]([A-Za-z]+)['"`]$""", method_value)
        return lit.group(1).upper() if lit else None
    if unreadable:
        return None
    return "GET"


@dataclass
class Call:
    path: str
    line: int
    url: str
    method: str | None  # None = present but not statically resolvable
    has_csrf: bool
    kind: str = "fetch"  # "fetch" | "sendBeacon"


def collect_files() -> list[Path]:
    """Tracked AND untracked-but-not-ignored sources.

    ``git ls-files`` alone misses files that exist locally but are not staged
    yet — the gate is then green where it is read and red where it counts.
    """
    tracked = subprocess.run(
        ["git", "ls-files", "--", SCAN_ROOT],
        cwd=REPO_ROOT, capture_output=True, text=True, check=True,
    ).stdout.split()
    untracked = subprocess.run(
        ["git", "ls-files", "--others", "--exclude-standard", "--", SCAN_ROOT],
        cwd=REPO_ROOT, capture_output=True, text=True, check=True,
    ).stdout.split()
    names = sorted(set(tracked) | set(untracked))
    return [REPO_ROOT / n for n in names if n.endswith(SOURCE_SUFFIXES)]


def scan_file(path: Path) -> tuple[list[Call], list[tuple[int, str]]]:
    raw = path.read_text(encoding="utf-8", errors="replace")
    src = strip_comments(raw)
    rel = path.relative_to(REPO_ROOT).as_posix()
    calls: list[Call] = []
    unparsed: list[tuple[int, str]] = []

    # `navigator.sendBeacon(url, data)` is an unconditional POST that cannot
    # carry any header, so it can never satisfy the double-submit check.
    for m in BEACON_RE.finditer(src):
        line = src.count("\n", 0, m.start()) + 1
        close = match_paren(src, src.index("(", m.start()))
        args = split_top_level_args(src[src.index("(", m.start()) + 1 : close]) if close else []
        url = " ".join(args[0].split()) if args else "<unparsed>"
        calls.append(Call(rel, line, url, "POST", has_csrf=False, kind="sendBeacon"))

    for m in OTHER_CLIENT_RE.finditer(src):
        line = src.count("\n", 0, m.start()) + 1
        unparsed.append((line, f"{m.group(0).strip()} — HTTP client this gate cannot read"))

    # fetch.call / .apply / .bind — argument positions shift or move into an
    # array, so the positional parse below would read the wrong operand.
    # Reported as an unresolvable write rather than parsed.
    for m in FETCH_REFLECT_RE.finditer(src):
        line = src.count("\n", 0, m.start()) + 1
        calls.append(
            Call(rel, line, f"fetch.{m.group(1)}(...)", method=None, has_csrf=False)
        )

    # Any binding of fetch to another name reaches the network under a name
    # the main pattern does not know.
    aliases = set(ALIAS_RE.findall(src)) | set(ALIAS_DESTRUCTURE_RE.findall(src))
    aliases.discard("fetch")
    call_patterns = [FETCH_RE]
    for alias in aliases:
        call_patterns.append(re.compile(rf"(?<![\w$.])({re.escape(alias)})\s*(?:\?\.)?\("))

    for m in (mm for pat in call_patterns for mm in pat.finditer(src)):
        open_idx = src.index("(", m.start())
        line = src.count("\n", 0, m.start()) + 1
        close_idx = match_paren(src, open_idx)
        if close_idx is None:
            unparsed.append((line, "unbalanced fetch( ... ) — parser gave up"))
            continue
        args = split_top_level_args(src[open_idx + 1 : close_idx])
        if not args:
            unparsed.append((line, "fetch() with no arguments"))
            continue

        url = " ".join(args[0].split())
        options = args[1] if len(args) > 1 else ""

        method = resolve_method(url, options)
        calls.append(Call(rel, line, url, method, has_csrf_header(options, src)))
    return calls, unparsed


def main() -> int:
    files = collect_files()
    sources = [f for f in files if not any(t in f.name for t in TEST_MARKERS)]
    excluded = len(files) - len(sources)

    # The exemption is a file path, so a rename silently turns it into a
    # no-op: the gate would keep printing the old name and `files scanned`
    # would be off by one. The failure direction is safe (client.ts gets
    # scanned and apiFetch's own spread turns it red), but a wrong name in the
    # output is a lie either way.
    if not (REPO_ROOT / APIFETCH_IMPL).exists():
        print(f"FAIL — the apiFetch exemption points at a file that no longer exists: {APIFETCH_IMPL}")
        print("Update APIFETCH_IMPL to the new path.")
        return 1

    reads = 0
    manual_csrf: list[Call] = []
    allowed: list[Call] = []
    violations: list[tuple[Call, str]] = []
    skipped: list[tuple[str, int, str]] = []
    used_allowlist: set[tuple[str, str]] = set()

    for path in sources:
        if path.relative_to(REPO_ROOT).as_posix() == APIFETCH_IMPL:
            continue
        calls, unparsed = scan_file(path)
        for line, reason in unparsed:
            skipped.append((path.relative_to(REPO_ROOT).as_posix(), line, reason))
        for call in calls:
            key = (call.path, call.url)

            # sendBeacon is a POST that structurally cannot carry a header, so
            # no amount of CSRF plumbing saves it; only the allowlist can.
            if call.kind == "sendBeacon":
                if key in ALLOWLIST:
                    allowed.append(call)
                    used_allowlist.add(key)
                else:
                    violations.append(
                        (call, "navigator.sendBeacon cannot send headers — it can never pass CSRF")
                    )
                continue

            # A resolvable safe method is out of scope, token or not. Checked
            # before the allowlist so an allowlisted URL that is also fetched
            # with GET does not inflate the allowlist count.
            if call.method is not None and call.method not in WRITE_METHODS:
                reads += 1
                continue

            if key in ALLOWLIST:
                allowed.append(call)
                used_allowlist.add(key)
                continue

            # Carrying the token makes the call safe whichever method it turns
            # out to be. This is what lets a legitimate wrapper clear the gate
            # without an exemption: downloadBlob spreads the caller's init, so
            # its method is unresolvable by construction, but it attaches the
            # header itself for every non-safe method.
            if call.has_csrf:
                manual_csrf.append(call)
                continue

            if call.method is None:
                # Never waved through as a GET: an unreadable call may well be
                # a write, and guessing GET is the silent misclassification
                # this gate exists to prevent.
                violations.append(
                    (call, "HTTP method is not a literal — cannot prove this is not a write")
                )
                continue

            violations.append((call, "authenticated write without X-CSRF-Token — use apiFetch"))

    print(f"G-CSRF — raw fetch() writes in {SCAN_ROOT}")
    print(f"  files scanned : {len(sources) - 1} (excluded {excluded} test files, plus {APIFETCH_IMPL})")
    print(f"  raw reads     : {reads} (not judged — CSRF middleware ignores safe methods)")
    print(f"  allowlisted   : {len(allowed)} public / token-in-URL writes")
    print(f"  manual CSRF   : {len(manual_csrf)} raw writes that set the header themselves")
    print(f"  skipped       : {len(skipped)}")
    for path, line, reason in skipped:
        print(f"      {path}:{line} — {reason}")

    stale = sorted(set(ALLOWLIST) - used_allowlist)
    if stale:
        print(f"  stale allowlist entries: {len(stale)} (call moved or was fixed — remove them)")
        for path, url in stale:
            print(f"      {path} — {url}")

    if manual_csrf:
        print("\n  Raw writes with a hand-rolled X-CSRF-Token (correct, but each copy")
        print("  is the next drift source — prefer apiFetch):")
        for call in manual_csrf:
            print(f"      {call.path}:{call.line} {call.method or '<unresolved>'} {call.url}")

    if violations:
        print(f"\nFAIL — {len(violations)} authenticated write(s) bypass apiFetch:\n")
        for call, reason in violations:
            print(f"  {call.path}:{call.line}")
            print(f"      {call.method or '<unresolved>'} {call.url}")
            print(f"      {reason}")
        print("\nFix: route the call through apiFetch (it attaches X-CSRF-Token and")
        print("X-Vakt-Session-Id, and omits Content-Type for FormData so the browser")
        print("sets the multipart boundary). If the endpoint really is public or")
        print("token-authenticated, add it to ALLOWLIST with a reason.")
        return 1

    print("\nOK — every authenticated frontend write goes through apiFetch.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
