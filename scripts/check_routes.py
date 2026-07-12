#!/usr/bin/env python3
# S121-E1 (O2): FE↔BE route-reconciliation gate.
#
# The recurring "handler-without-route / path-drift / wrong-method" defect class
# (v0.42.25/26/27 and the whole S121 audit) was only ever caught by manual
# Playwright sweeps. This static gate closes it: it diffs every frontend
# `apiFetch(...)` call against every backend `routes.go` registration and fails
# when the frontend calls a (method, path) the backend never registers.
#
# It is a fast, dependency-free set-diff — no running stack, no DB. Because
# backend route literals are group-relative (e.g. `/controls/bulk` mounted under
# the `/vaktcomply` group) while frontend paths are absolute below `/api/v1`
# (e.g. `/vaktcomply/controls/bulk`), a frontend path is considered covered when
# some backend literal of the same method is a segment-aligned SUFFIX of it. This
# keeps false positives near zero (the gate's whole value is a quiet, trustworthy
# signal) while still catching missing routes and method mismatches.
#
# Curated ALLOWLIST entries are frontend calls that legitimately have no
# routes.go registration (dynamic dispatch, non-apiFetch endpoints, etc.).

import re
import sys
import pathlib

ROOT = pathlib.Path(__file__).resolve().parent.parent
BACKEND = ROOT / "backend"
FRONTEND = ROOT / "frontend" / "src"

METHODS = ("GET", "POST", "PUT", "PATCH", "DELETE")

# (method, normalized-path) pairs the frontend may call without a matching
# routes.go literal. Each entry is a matcher blind spot, NOT a real gap.
ALLOWLIST = {
    # Cross-file group roots: registered as `.METHOD("")` on a group whose prefix
    # is set in cmd/api/routes.go (e.g. protected.Group("/webhooks")). The static
    # resolver only follows group prefixes within one file, so the root path is
    # invisible here even though the route exists.
    ("GET", "/webhooks"),
    ("POST", "/webhooks"),
    ("GET", "/audit-log"),
    ("POST", "/setup"),
    # Dynamically-constructed backend routes: vaktcomply registers collab-tasks
    # per entity via `g.GET("/"+entity+"/:id/collab-tasks", ...)` in a loop, so the
    # full literal never appears as a single string the parser can read.
    ("GET", "/vaktcomply/:p/:p/collab-tasks"),
    ("POST", "/vaktcomply/:p/:p/collab-tasks"),
    # Managed-hosting multi-org admin (AdminTenantsPage): deliberately NOT built —
    # gated on the EULA/AVV legal review (Sprints 104/111/118). The frontend stubs
    # call endpoints that intentionally do not exist yet.
    ("GET", "/admin/organizations"),
    ("POST", "/admin/organizations"),
    ("POST", "/admin/organizations/:p/impersonate"),
}


def norm(path: str) -> str:
    """Normalize a path: strip query, collapse params to :p, strip trailing slash."""
    path = path.split("?", 1)[0]
    path = path.split("#", 1)[0]
    # A ${...} template glued to the end of a word (not its own /segment/) is a
    # query-string interpolation (e.g. `/milestones${qs}`), not a path param —
    # truncate the path there.
    path = re.sub(r"(?<=[^/])\$\{.*$", "", path)
    # ${...} template expressions and :param segments -> :p
    path = re.sub(r"\$\{[^}]*\}", ":p", path)
    path = re.sub(r":[A-Za-z_][A-Za-z0-9_]*", ":p", path)
    # collapse any segment that is now purely a param placeholder
    if path != "/" and path.endswith("/"):
        path = path[:-1]
    if not path.startswith("/"):
        path = "/" + path
    return path


def collect_backend_routes():
    """Return dict method -> set of normalized, locally-qualified path literals.

    Backend routes are registered on Echo groups whose prefixes are assigned
    locally, e.g. `keys := g.Group("/api-keys")` then `keys.POST("", ...)`. We
    resolve the chain of `.Group("...")` prefixes *within the same file* so a
    root registration (`.POST("")`) resolves to `/api-keys`, not the empty
    string. Cross-file mount prefixes (the module group in cmd/api/routes.go)
    are intentionally left off — the frontend matcher uses suffix matching.
    """
    routes = {m: set() for m in METHODS}
    grp_pat = re.compile(r'(\w+)\s*:=\s*(\w+)\.Group\(\s*"([^"]*)"')
    route_pat = re.compile(r'(\w+)\.(GET|POST|PUT|PATCH|DELETE)\(\s*"([^"]*)"')
    for f in BACKEND.rglob("*.go"):
        if f.name.endswith("_test.go"):
            continue
        text = f.read_text(encoding="utf-8", errors="ignore")
        # var -> (parent_var, prefix) within this file
        groups = {}
        for var, parent, prefix in grp_pat.findall(text):
            groups[var] = (parent, prefix)

        def resolve(var):
            prefix = ""
            seen = set()
            while var in groups and var not in seen:
                seen.add(var)
                parent, p = groups[var]
                prefix = p + prefix
                var = parent
            return prefix

        for var, m, lit in route_pat.findall(text):
            full = resolve(var) + lit
            # Routes registered directly on the `api`/`protected` group in
            # cmd/api/routes.go resolve to an /api/v1-prefixed literal; strip it so
            # both sides are /api/v1-relative and the suffix match is symmetric.
            if full.startswith("/api/v1"):
                full = full[len("/api/v1"):] or "/"
            routes[m].add(norm(full))
    return routes


def _all_methods_paths(be):
    """Union of every backend path across all methods (for path-only checks)."""
    out = set()
    for s in be.values():
        out |= s
    return out


# Skip counter — S123-G2. A gate that silently skips inputs it cannot parse
# reports success for work it did not do (the "OK over a subset" trap, D12). We
# count every recognised call whose path we could NOT resolve to a concrete
# /api/v1 path and print the total, so "OK" never overstates coverage.
SKIPPED = []


def _file_base_consts(text):
    """Map local `const NAME = '/path'` (string-literal path) declarations so a
    call built as `${NAME}/rest` can be resolved. Only literal string values that
    look like an API path (start with '/') are captured."""
    consts = {}
    for name, val in re.findall(r"""const\s+([A-Za-z_$][\w$]*)\s*=\s*['"]([^'"]+)['"]""", text):
        if val.startswith("/"):
            consts[name] = val
    return consts


def _resolve_path(raw, consts):
    """Resolve a raw call path literal to an absolute /api/v1-relative path, or
    return None if it cannot be resolved statically (caller records a skip)."""
    # Leading `${BASE}` (or `${BASE}` mid-word) — substitute a known const.
    m = re.match(r"\$\{([A-Za-z_$][\w$]*)\}(.*)$", raw)
    if m:
        name, rest = m.group(1), m.group(2)
        if name in consts:
            raw = consts[name] + rest
        else:
            return None  # unknown base constant — unresolvable
    if not raw.startswith("/"):
        return None  # still relative / dynamic
    if raw.startswith("/api/v1/") or raw == "/api/v1":
        raw = raw[len("/api/v1"):] or "/"
    return raw


def _method_in(window, default="GET"):
    m = re.search(r"method:\s*['\"]([A-Z]+)['\"]", window)
    return m.group(1) if m else default


def collect_frontend_calls():
    """Return (method_calls, path_only_calls):
      - method_calls: set of (method, path) from apiFetch(...) and raw
        fetch('/api/v1/...') — method is reliably readable from the options.
      - path_only_calls: set of paths from `endpoint=` props, whose HTTP method
        is component-specific and not statically knowable (an ExportButton GETs,
        an AIAdvisor POST-streams). These are checked for existence under ANY
        method — enough to catch a route that does not exist at all (CB-01).
    Resolves `${BASE}` local constants; records unresolvable paths in SKIPPED (G2)."""
    method_calls = set()
    path_only = set()
    path_pat = re.compile(r"""^\s*(['"`])([^'"`]*)\1""")
    raw_fetch_pat = re.compile(r"""\bfetch\s*\(\s*(['"`])(/api/v1/[^'"`]*)\1""")
    raw_fetch_tmpl_pat = re.compile(r"""\bfetch\s*\(\s*`(\$\{[^`]*?)`""")
    endpoint_pat = re.compile(r"""endpoint\s*[=:]\s*\{?\s*(['"`])([^'"`]*)\1""")
    call_pat = re.compile(r"apiFetch\s*(?:<[^>]*>)?\s*\(")

    for f in list(FRONTEND.rglob("*.ts")) + list(FRONTEND.rglob("*.tsx")):
        if f.name.endswith((".test.ts", ".test.tsx", ".spec.ts", ".spec.tsx")):
            continue
        # api/client.ts is the apiFetch WRAPPER itself: its lone `fetch(\`${API_BASE}${path}\`)`
        # is the transport, not a call to a concrete endpoint. Skip it.
        if f.name == "client.ts" and f.parent.name == "api":
            continue
        text = f.read_text(encoding="utf-8", errors="ignore")
        consts = _file_base_consts(text)
        rel = f.relative_to(ROOT)

        def method_window(end_pos):
            # Cap at the next fetch/apiFetch call so we never read a sibling
            # call's `method:` (the dsr-portal /info-vs-/submit bleed).
            tail = text[end_pos: end_pos + 400]
            stops = [tail.find("fetch("), tail.find("apiFetch")]
            stops = [s for s in stops if s >= 0]
            return tail if not stops else tail[: min(stops)]

        # 1) apiFetch(...) — resolve ${BASE}; method read from THIS call's options.
        for mobj in call_pat.finditer(text):
            after = text[mobj.end(): mobj.end() + 400]
            pm = path_pat.match(after)
            if not pm:
                SKIPPED.append(f"{rel}: apiFetch with a non-literal path")
                continue
            resolved = _resolve_path(pm.group(2), consts)
            if resolved is None:
                SKIPPED.append(f"{rel}: apiFetch(\"{pm.group(2)[:40]}\") unresolved")
                continue
            nxt = after.find("apiFetch", pm.end())
            window = after if nxt < 0 else after[:nxt]
            method_calls.add((_method_in(window), norm(resolved)))

        # 2) raw fetch('/api/v1/...') — window taken from the MATCH position, not
        # text.find (which would land on a path-mentioning doc comment above).
        for mobj in raw_fetch_pat.finditer(text):
            resolved = _resolve_path(mobj.group(2), consts)
            if resolved is None:
                continue
            method_calls.add((_method_in(method_window(mobj.end())), norm(resolved)))

        # 2b) raw fetch(`${BASE}/...`) — resolve the base constant.
        for mobj in raw_fetch_tmpl_pat.finditer(text):
            resolved = _resolve_path(mobj.group(1), consts)
            if resolved is None:
                SKIPPED.append(f"{rel}: fetch(`{mobj.group(1)[:40]}`) unresolved base")
                continue
            method_calls.add((_method_in(method_window(mobj.end())), norm(resolved)))

        # 3) endpoint= props — path-only (method not statically knowable).
        for q, path in endpoint_pat.findall(text):
            resolved = _resolve_path(path, consts)
            if resolved is None:
                SKIPPED.append(f"{rel}: endpoint=\"{path[:40]}\" unresolved")
                continue
            path_only.add(norm(resolved))

    return method_calls, path_only


def _segs_match(be_segs, fe_segs):
    """Segment-wise match where a backend :p wildcard matches any fe segment.

    This handles the common case where the frontend hardcodes a param value in
    the path (e.g. `/frameworks/NIS2/enable`) while the backend registers a param
    route (`/frameworks/:name/enable`) — the value IS the param."""
    for be_s, fe_s in zip(be_segs, fe_segs):
        if be_s == ":p":
            continue
        if be_s != fe_s:
            return False
    return True


def suffix_covered(fe_path: str, be_paths: set, strict: bool = False) -> bool:
    """True if some backend literal is a segment-aligned suffix of fe_path.

    strict=True rejects a backend literal made ENTIRELY of `:p` wildcard segments
    (e.g. `/:p` from a bare `g.GET("/:id")`): it matches the trailing segment of
    ANY path (`.../export/xlsx` "matches" `/:p` on `xlsx`), masking genuinely
    missing routes like CB-01. Use strict for `endpoint=` paths, whose terminal
    segments are concrete resource names, not param values. The permissive mode
    (default) keeps legitimate bare `/:id` routes (github/webhooks DELETE) green."""
    fe_segs = fe_path.strip("/").split("/")
    for be in be_paths:
        if be in ("", "/"):
            continue
        be_segs = be.strip("/").split("/")
        if len(be_segs) > len(fe_segs):
            continue
        if strict and all(s == ":p" for s in be_segs):
            continue
        if _segs_match(be_segs, fe_segs[len(fe_segs) - len(be_segs):]):
            return True
    return False


def main():
    be = collect_backend_routes()
    fe, fe_path_only = collect_frontend_calls()
    any_method_paths = _all_methods_paths(be)

    missing = []
    for method, path in sorted(fe):
        if (method, path) in ALLOWLIST:
            continue
        if not suffix_covered(path, be.get(method, set())):
            missing.append((method, path))

    # endpoint= props: existence under ANY method (method is component-specific),
    # strict so a bare `/:p` route cannot spuriously satisfy a concrete export path.
    for path in sorted(fe_path_only):
        if ("*", path) in ALLOWLIST or ("GET", path) in ALLOWLIST:
            continue
        if not suffix_covered(path, any_method_paths, strict=True):
            missing.append(("(any)", path))

    if missing:
        print("FE→BE route reconciliation FAILED — frontend calls with no matching backend route:")
        for method, path in missing:
            print(f"  {method:6} {path}")
        print(
            "\nFix: register the route in the module's routes.go (or correct the "
            "HTTP method), or — if intentional — add it to ALLOWLIST in "
            "scripts/check_routes.py with a comment explaining why."
        )
        sys.exit(1)

    # G2: report what the parser could NOT resolve, so "OK" is honest about its
    # coverage instead of silently claiming success over a subset (D12).
    skipped = len(SKIPPED)
    print(f"OK — {len(fe)} method-checked calls (apiFetch + raw fetch) and "
          f"{len(fe_path_only)} endpoint= paths all matched a backend route.")
    if skipped:
        print(f"note: {skipped} call(s) had a path the static parser could not "
              f"resolve (dynamic/computed) and were not checked:")
        for s in sorted(set(SKIPPED)):
            print(f"  - {s}")


if __name__ == "__main__":
    main()
