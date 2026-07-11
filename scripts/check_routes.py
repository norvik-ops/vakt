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
            routes[m].add(norm(full))
    return routes


def collect_frontend_calls():
    """Return set of (method, normalized-path) tuples from apiFetch calls."""
    calls = set()
    # apiFetch<...>('path' | `path`, { ... method: 'X' ... })
    call_pat = re.compile(r"apiFetch\s*(?:<[^>]*>)?\s*\(")
    path_pat = re.compile(r"""^\s*(['"`])([^'"`]*)\1""")
    method_pat = re.compile(r"method:\s*['\"]([A-Z]+)['\"]")
    for f in list(FRONTEND.rglob("*.ts")) + list(FRONTEND.rglob("*.tsx")):
        if f.name.endswith((".test.ts", ".test.tsx", ".spec.ts", ".spec.tsx")):
            continue
        text = f.read_text(encoding="utf-8", errors="ignore")
        for mobj in call_pat.finditer(text):
            after = text[mobj.end(): mobj.end() + 400]
            pm = path_pat.match(after)
            if not pm:
                continue  # dynamically-built path — cannot analyze statically
            raw = pm.group(2)
            if not raw.startswith("/"):
                continue  # relative/base-var path — skip
            if raw.startswith("/api/v1/"):
                raw = raw[len("/api/v1"):]
            # Only look for `method:` within THIS call — stop at the next
            # apiFetch(...) so we never pick up a neighbouring call's method.
            nxt = after.find("apiFetch", pm.end())
            window = after if nxt < 0 else after[:nxt]
            method = "GET"
            meth = method_pat.search(window)
            if meth:
                method = meth.group(1)
            calls.add((method, norm(raw)))
    return calls


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


def suffix_covered(fe_path: str, be_paths: set) -> bool:
    """True if some backend literal is a segment-aligned suffix of fe_path."""
    fe_segs = fe_path.strip("/").split("/")
    for be in be_paths:
        if be in ("", "/"):
            continue
        be_segs = be.strip("/").split("/")
        if len(be_segs) > len(fe_segs):
            continue
        if _segs_match(be_segs, fe_segs[len(fe_segs) - len(be_segs):]):
            return True
    return False


def main():
    be = collect_backend_routes()
    fe = collect_frontend_calls()

    missing = []
    for method, path in sorted(fe):
        if (method, path) in ALLOWLIST:
            continue
        if not suffix_covered(path, be.get(method, set())):
            missing.append((method, path))

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

    print(f"OK — {len(fe)} frontend apiFetch calls all matched a backend route.")


if __name__ == "__main__":
    main()
