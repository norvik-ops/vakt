#!/usr/bin/env python3
# S128-2 (G7): OpenAPI under-coverage ratchet.
#
# openapi.yaml is not documentation. The frontend's types are GENERATED from it
# (`npm run api-types`), external SDK consumers read it, and the contract test
# validates real responses against it. A route that is missing from the spec is a
# route whose shape nothing checks — which is how /vaktcomply/controls/:id/changelog
# came to return an object where the frontend expected an array, and how
# dsr-portal-settings and policy-templates shipped, got called by the frontend, and
# appeared in the spec exactly nowhere (S128-1, D14/D15).
#
# So this gate freezes the number of undocumented routes and only lets it go DOWN.
# A new route must either be documented or must explicitly raise nothing: the
# baseline can be lowered, never raised.
#
# WHY A FLOOR AND NOT A TARGET: full coverage is ~371 route-operations of work.
# A gate that demands it on day one gets switched off on day two. A gate that says
# "you may not make it worse" survives, and every sprint that documents a batch
# lowers the number for good.
#
# HOW IT RESOLVES A ROUTE'S REAL PATH — this is the whole difficulty, and the
# reason check_routes.py could not be reused as-is. A module registers
# group-relative literals:
#
#     // internal/modules/vaktscan/routes.go
#     func Register(g *echo.Group, h *Handler) { g.GET("/findings", ...) }
#
# and cmd/api/routes.go decides where that group hangs:
#
#     vaktscan.Register(protected.Group("/vaktscan", ...), ...)
#     protected := api.Group("", ...)      // api := e.Group("/api/v1")
#
# Only both halves together say `/api/v1/vaktscan/findings`. check_routes.py sidesteps
# this with suffix matching, which is fine for "does a backend route exist at all"
# but useless here: to diff against the spec we need the actual, absolute path.
#
# WHAT IT CANNOT SEE, it counts and prints (`skipped`). A gate that silently drops
# the inputs it cannot parse reports success for work it did not do — the trap
# check_routes.py fell into, quietly skipping a quarter of the frontend while
# printing OK.

import re
import sys
import pathlib

ROOT = pathlib.Path(__file__).resolve().parent.parent
BACKEND = ROOT / "backend"
ROUTES_GO = BACKEND / "cmd" / "api" / "routes.go"
SPEC = BACKEND / "internal" / "shared" / "apidocs" / "openapi.yaml"

METHODS = ("GET", "POST", "PUT", "PATCH", "DELETE")

# The known-undocumented routes, one per line, checked in beside this script.
#
# A bare COUNT would be enough to stop the number growing, but it would tell a
# developer "one of these 361 routes is your fault" and leave them to find it. The
# list names the offender. Regenerate with:
#
#     python3 scripts/check_openapi_coverage.py --update-baseline
#
# and commit the diff — shrinking it is the point, and a shrink that nobody records
# is one the next commit can quietly undo.
BASELINE_FILE = pathlib.Path(__file__).resolve().parent / "openapi_coverage_baseline.txt"

SKIPPED = []

# Routes that are deliberately NOT part of the JSON API contract, so documenting
# them in openapi.yaml would be wrong rather than merely tedious:
#   - the API-docs UI serves itself and its own assets (it IS the rendered spec);
#   - /metrics speaks the Prometheus text format, not JSON;
#   - /trust/:slug returns the public Trust Center HTML page, not a resource.
# They are excluded from both the numerator and the denominator, so the coverage
# figure is a statement about the API and not diluted by things that are not it.
NOT_API = {
    ("GET", "/api/docs"),
    ("GET", "/api/docs/swagger-ui-bundle.js"),
    ("GET", "/api/docs/swagger-ui.css"),
    ("GET", "/api/v1/openapi.yaml"),
    ("GET", "/metrics"),
    ("GET", "/trust/:p"),
}


def norm(path: str) -> str:
    """Absolute path with every parameter collapsed to `:p`.

    Both sides of the diff go through this, so `/x/{id}` (spec) and `/x/:id`
    (Echo) compare equal, and `/x/{cid}` vs `/x/:id` does not spuriously differ.
    """
    path = re.sub(r"\{[^}]*\}", ":p", path)
    path = re.sub(r":[A-Za-z_][A-Za-z0-9_]*", ":p", path)
    path = re.sub(r"/{2,}", "/", path)
    if len(path) > 1 and path.endswith("/"):
        path = path[:-1]
    return path or "/"


# ── Backend side ─────────────────────────────────────────────────────────────

GROUP_ASSIGN = re.compile(r'(\w+)\s*:=\s*(\w+)\.Group\(\s*"([^"]*)"')
# The trailing group catches a concatenation: vaktcomply builds four routes per
# entity as `g.GET("/"+entity+"/:id/comments", ...)`. Reading only the leading "/"
# literal would invent a route at the group root that does not exist — a gate that
# hallucinates a route is worse than one that admits it cannot see it.
ROUTE_CALL = re.compile(r'(\w+)\.(GET|POST|PUT|PATCH|DELETE)\(\s*"([^"]*)"(\s*\+)?')
# A mount: `pkg.RegisterX(<args>)` in cmd/api/routes.go. The argument list is
# captured whole (balanced by split_args below) — a registration can mount more
# than one group at once, and taking only the first would silently drop the rest.
MOUNT = re.compile(r'\b(\w+)\.(Register\w*)\(')
# A group expression used inline as a mount target: `protected.Group("/vaktscan", …)`
INLINE_GROUP = re.compile(r'(\w+)\.Group\(\s*"([^"]*)"')


def split_args(text: str, open_idx: int):
    """Top-level arguments of the call whose '(' is at open_idx."""
    depth, arg, args = 0, "", []
    i = open_idx
    while i < len(text):
        ch = text[i]
        if ch in "([{":
            depth += 1
            if depth == 1:
                i += 1
                continue
        elif ch in ")]}":
            depth -= 1
            if depth == 0:
                args.append(arg.strip())
                return [a for a in args if a], i
        elif ch == "," and depth == 1:
            args.append(arg.strip())
            arg = ""
            i += 1
            continue
        if depth >= 1:
            arg += ch
        i += 1
    return [], len(text)


def echo_params(head: str):
    """[(name, 'Group'|'Echo')] for the echo parameters of a function signature,
    in order — so they can be matched positionally against the call's arguments."""
    out = []
    for names, kind in re.findall(r"(\w+(?:\s*,\s*\w+)*)\s+\*echo\.(Group|Echo)\b", head):
        for n in names.split(","):
            out.append((n.strip(), kind))
    return out


def resolve_group_vars(text: str) -> dict:
    """var -> absolute prefix, for the group variables declared in cmd/api/routes.go."""
    raw = {}
    for var, parent, prefix in GROUP_ASSIGN.findall(text):
        raw[var] = (parent, prefix)
    # `api := e.Group("/api/v1")` — the root's parent is the echo instance itself.
    resolved = {}

    def resolve(var, seen=()):
        if var in resolved:
            return resolved[var]
        if var not in raw or var in seen:
            return ""  # the echo instance, or a cycle
        parent, prefix = raw[var]
        out = resolve(parent, seen + (var,)) + prefix
        resolved[var] = out
        return out

    for var in raw:
        resolve(var)
    return resolved


def go_package_dirs() -> dict:
    """Import path -> directory, for every package under backend/internal and backend/cmd."""
    out = {}
    for f in BACKEND.rglob("*.go"):
        if f.name.endswith("_test.go"):
            continue
        rel = f.parent.relative_to(BACKEND)
        out["github.com/matharnica/vakt/" + rel.as_posix()] = f.parent
    return out


def import_aliases(text: str, pkg_dirs: dict) -> dict:
    """Local package identifier -> directory, from the import block."""
    aliases = {}
    for alias, path in re.findall(r'^\s*(?:(\w+)\s+)?"(github\.com/matharnica/vakt/[^"]+)"', text, re.M):
        d = pkg_dirs.get(path)
        if d is None:
            continue
        aliases[alias or path.rsplit("/", 1)[-1]] = d
    return aliases


def func_body(dir_: pathlib.Path, name: str):
    """Source text of `func name(...)` in the given package dir, brace-balanced."""
    for f in sorted(dir_.glob("*.go")):
        if f.name.endswith("_test.go"):
            continue
        text = f.read_text(encoding="utf-8", errors="ignore")
        m = re.search(r"^func\s+" + re.escape(name) + r"\s*\(", text, re.M)
        if not m:
            continue
        i = text.index("{", m.end() - 1)
        depth, j = 0, i
        while j < len(text):
            if text[j] == "{":
                depth += 1
            elif text[j] == "}":
                depth -= 1
                if depth == 0:
                    return text[i: j + 1], f
            j += 1
        return text[i:], f
    return None, None


# A call to another function in the same package, passing a group: the module's
# public Register() is often only a doorway (vaktcomply.Register -> registerRoutes
# -> registerAccessReviewRoutes). Not following those loses most of the module's
# routes — and loses them QUIETLY, which is the failure mode this whole file exists
# to prevent.
DELEGATE = re.compile(r"(?<![\w.])(\w+)\(\s*(\w+)\s*[,)]")


def routes_in_body(body: str, mounts: dict, origin: str = "?", pkg_dir=None, seen=None) -> set:
    """(method, absolute path) for every literal route registered in a function body.

    `mounts` maps EVERY echo parameter of the function to its absolute mount prefix
    — all of them at once, not one per call. A function can take two groups
    (usermgmt.RegisterRoutes takes the admin group and the public invite group), and
    resolving one at a time would report the other's routes as unresolvable on every
    pass: a gate lying about its own blind spots.

    Local sub-groups (`sub := g.Group("/x")`) resolve against their parent, so a
    route on `sub` lands at mount + "/x" + literal.
    """
    prefixes = dict(mounts)
    # Resolve local groups; iterate to a fixed point so `a := g.Group(...)` and
    # `b := a.Group(...)` both land regardless of declaration order.
    pending = GROUP_ASSIGN.findall(body)
    for _ in range(len(pending) + 1):
        for var, parent, prefix in pending:
            if parent in prefixes and var not in prefixes:
                prefixes[var] = prefixes[parent] + prefix

    out = set()

    # Follow same-package delegation: `registerRoutes(g, h)` continues the wiring
    # under the same mount.
    if pkg_dir is not None:
        seen = seen or set()
        for callee, arg in DELEGATE.findall(body):
            if callee in seen or arg not in prefixes:
                continue
            if callee in ("Group", "Use", "Add") or callee[0].isupper() and callee in METHODS:
                continue
            sub_head, _ = func_head(pkg_dir, callee)
            sub_body, _ = func_body(pkg_dir, callee)
            if sub_head is None or sub_body is None:
                continue
            sub_params = echo_params(sub_head)
            if not sub_params:
                continue  # not a route-wiring function
            seen.add(callee)
            out |= routes_in_body(
                sub_body,
                {sub_params[0][0]: prefixes[arg]},
                f"{origin}->{callee}",
                pkg_dir,
                seen,
            )

    for var, method, lit, concat in ROUTE_CALL.findall(body):
        if concat:
            SKIPPED.append(f"{origin}: {method} \"{lit}\"+… — path built at runtime, cannot be resolved statically")
            continue
        if var not in prefixes:
            # A route hung on a group this resolver could not trace back to the
            # mount. Counting it is the whole point: a route the gate cannot see is
            # a route the gate is not guarding, and saying so out loud is the
            # difference between a signal and a comfortable number.
            SKIPPED.append(f"{origin}: {method} {lit} on unresolved group `{var}`")
            continue
        out.add((method, norm(prefixes[var] + lit)))
    return out


def collect_backend_routes() -> set:
    text = ROUTES_GO.read_text(encoding="utf-8")
    groups = resolve_group_vars(text)
    pkg_dirs = go_package_dirs()
    aliases = import_aliases(text, pkg_dirs)

    routes = set()

    # 1) Routes registered directly on a group in cmd/api/routes.go.
    for var, method, lit, concat in ROUTE_CALL.findall(text):
        if concat:
            SKIPPED.append(f"cmd/api/routes.go: {method} \"{lit}\"+… — path built at runtime")
            continue
        if var in groups:
            routes.add((method, norm(groups[var] + lit)))

    # 2) Routes registered by a module's Register* function, mounted here.
    for mm in MOUNT.finditer(text):
        pkg, func = mm.group(1), mm.group(2)
        d = aliases.get(pkg)
        if d is None:
            continue  # a method on a local value, not one of our packages
        args, _ = split_args(text, mm.end() - 1)

        head, _ = func_head(d, func)
        body, _ = func_body(d, func)
        if head is None or body is None:
            SKIPPED.append(f"{pkg}.{func} — function not found in {d.relative_to(ROOT)}")
            continue

        params = echo_params(head)
        if not params:
            SKIPPED.append(f"{pkg}.{func} — takes no echo group/instance")
            continue

        # Every echo parameter is mounted, not just the first: usermgmt.RegisterRoutes
        # takes the admin group AND the public invite group, and reading only the
        # first would drop /invite/info and /invite/accept without a word.
        mounts = {}
        for idx, (pname, kind) in enumerate(params):
            if kind == "Echo":
                mounts[pname] = ""  # mounted on the root; builds absolute paths itself
                continue
            if idx >= len(args):
                SKIPPED.append(f"{pkg}.{func} — parameter {pname} has no matching argument")
                continue
            a = args[idx]
            if a in groups:
                mounts[pname] = groups[a]
                continue
            m = INLINE_GROUP.search(a)
            if m and m.group(1) in groups:
                mounts[pname] = groups[m.group(1)] + m.group(2)
            else:
                SKIPPED.append(f"{pkg}.{func}({a[:36]}…) — mount prefix not resolvable")

        routes |= routes_in_body(body, mounts, f"{pkg}.{func}", d)

    return routes


def func_head(dir_: pathlib.Path, name: str):
    """The parameter list of `func name(...)`, so we know what the group is called."""
    for f in sorted(dir_.glob("*.go")):
        if f.name.endswith("_test.go"):
            continue
        text = f.read_text(encoding="utf-8", errors="ignore")
        m = re.search(r"^func\s+" + re.escape(name) + r"\s*(\([^{]*)", text, re.M)
        if m:
            return m.group(1), f
    return None, None


# ── Spec side ────────────────────────────────────────────────────────────────

def collect_spec_ops() -> set:
    """(METHOD, /api/v1-absolute path) for every operation in openapi.yaml.

    Parsed line-wise rather than with a YAML library so the gate stays
    dependency-free (it runs on a bare CI runner). The structure it relies on is
    the file's own: a path item at two-space indent, its methods at four.
    """
    ops = set()
    path = None
    in_paths = False
    for line in SPEC.read_text(encoding="utf-8").splitlines():
        if re.match(r"^paths:\s*$", line):
            in_paths = True
            continue
        if not in_paths:
            continue
        if re.match(r"^\S", line):  # a new top-level key ends the paths block
            break
        m = re.match(r"^  (/\S*):\s*$", line)
        if m:
            path = m.group(1)
            continue
        m = re.match(r"^    (get|post|put|patch|delete):\s*$", line)
        if m and path:
            ops.add((m.group(1).upper(), norm("/api/v1" + path)))
    return ops


def load_baseline() -> set:
    if not BASELINE_FILE.exists():
        return set()
    out = set()
    for line in BASELINE_FILE.read_text(encoding="utf-8").splitlines():
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        method, path = line.split(None, 1)
        out.add((method, path))
    return out


def write_baseline(undocumented) -> None:
    body = "\n".join(f"{m} {p}" for m, p in sorted(undocumented))
    BASELINE_FILE.write_text(
        "# Routes that exist in the backend and are NOT in openapi.yaml (S128-2 / G7).\n"
        "# Generated: python3 scripts/check_openapi_coverage.py --update-baseline\n"
        "# This list may only ever get SHORTER. See the script header for why.\n"
        + body + "\n",
        encoding="utf-8",
    )


def main() -> int:
    update = "--update-baseline" in sys.argv

    be = collect_backend_routes() - NOT_API
    spec = collect_spec_ops()

    undocumented = set(be - spec)
    n = len(undocumented)
    covered = len(be) - n
    pct = 100.0 * covered / len(be) if be else 0.0

    if update:
        write_baseline(undocumented)
        print(f"baseline written: {n} undocumented route(s) of {len(be)}")
        return 0

    baseline = load_baseline()
    print(f"OpenAPI coverage: {covered}/{len(be)} backend operations documented ({pct:.1f}%)")
    print(f"undocumented: {n}   baseline: {len(baseline)}")

    if SKIPPED:
        print(f"\nskipped: {len(SKIPPED)} route(s)/mount(s) the parser could not resolve — "
              f"they are NOT part of the numbers above:")
        for item in sorted(set(SKIPPED)):
            print(f"  - {item}")

    new = sorted(undocumented - baseline)
    if new:
        print(f"\nUNDER-COVERAGE GATE FAILED: {len(new)} route(s) with no openapi.yaml entry:\n")
        for method, path in new:
            print(f"  {method:6} {path}")
        print("\nFix: document each route in backend/internal/shared/apidocs/openapi.yaml, "
              "then run `npm run api-types` in frontend/ — same commit, or the type-drift "
              "check fails on the next push.")
        return 1

    fixed = sorted(baseline - undocumented)
    if fixed:
        print(f"\n{len(fixed)} route(s) newly documented — lock the gain in:\n")
        for method, path in fixed[:10]:
            print(f"  {method:6} {path}")
        if len(fixed) > 10:
            print(f"  … and {len(fixed) - 10} more")
        print("\n  python3 scripts/check_openapi_coverage.py --update-baseline\n")
        return 1

    return 0


if __name__ == "__main__":
    sys.exit(main())
