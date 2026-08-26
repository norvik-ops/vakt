#!/usr/bin/env node
// verify-sri.cjs — fails the build when the Vite production output is missing
// Subresource Integrity hashes on the subresources it loads.
//
// Audit response F[1]: strict CSP closes the inline-injection path; SRI closes
// the supply-chain path (asset swap at the CDN/proxy layer). If either layer
// regresses, deployments to self-hosters silently lose the guarantee — this
// check makes the regression a CI failure.
//
// Why this file was rewritten (K2-02, 2026-07-30)
// -----------------------------------------------
// The previous version had two defects that between them made it decorative:
//
//   (a) It looked at `<script src=>` and `<link rel=stylesheet>` only — 3 of the
//       13 integrity-carrying tags in the real dist/index.html. The four
//       `<link rel="modulepreload">` entries that load vendor-react,
//       vendor-query, vendor-i18n and vendor-ui (the bulk of the app's
//       JavaScript) were never looked at. A static `import` does NOT inherit
//       its parent script's integrity; for a preloaded module the guarantee
//       comes from the modulepreload tag alone. Stripping integrity from all
//       four left the gate printing "OK — 2 script + 1 stylesheet tag(s)".
//
//   (b) There was no non-vacuity floor. An index.html with no tags at all
//       printed "OK — 0 script + 0 stylesheet tag(s) all carry integrity=".
//       That is literally the "empty match = exit 0" class ci.yml names as
//       G-05 in the coverage-floor step and fixed there.
//
// It now classifies EVERY <script> and <link> in the document, requires
// integrity on every tag that can carry one, refuses to classify silently, and
// holds a ratchet on the count so neither a shrinking nor an unrecorded growing
// set of protected subresources passes unseen.
//
// Its denominator — what this gate does NOT look at:
//   * Only dist/index.html. Subresources referenced from inside a JS chunk at
//     runtime (dynamic import(), new Worker(), fetch of an asset) carry no tag
//     here and are not covered. Vite emits modulepreload tags for the static
//     graph, which is what this file measures.
//   * It checks that an integrity= attribute is PRESENT, not that the hash is
//     correct for the file — a wrong hash fails in the browser, loudly, which
//     is a different failure mode from a missing one (silent).
//   * Nothing outside frontend/dist (the Go-served static assets, the sites/
//     landing pages) is in scope.

const fs = require('fs')
const path = require('path')

// The number of tags in dist/index.html that must carry integrity=. Both
// directions are red on purpose:
//   fewer  → a protected subresource stopped being protected (or stopped being
//            emitted), which is the regression this gate exists for;
//   more   → the build gained a subresource whose protection nobody recorded.
//            Bump this line in the same commit and the reviewer sees the new
//            asset. Vite's manualChunks are fixed (vendor-react/query/i18n/ui),
//            so this number does not drift on ordinary dependency bumps.
const SRI_BASELINE = 13

// link rel values that load a subresource the browser executes, renders or
// installs — SRI applies and is required.
const REL_REQUIRES_SRI = new Set([
  'stylesheet',
  'modulepreload',
  'preload',
  'prefetch',
  'icon',
  'shortcut icon',
  'apple-touch-icon',
  'apple-touch-icon-precomposed',
  'mask-icon',
  'manifest',
])

// link rel values that reference no fetchable body of our own (or only a hint).
// Counted and printed as not-applicable — never silently dropped.
const REL_NOT_APPLICABLE = new Set([
  'preconnect',
  'dns-prefetch',
  'canonical',
  'alternate',
  'author',
  'license',
  'help',
  'next',
  'prev',
  'search',
  'me',
])

const indexPath = path.join(__dirname, '..', 'dist', 'index.html')

if (!fs.existsSync(indexPath)) {
  console.error(`verify-sri: ${indexPath} not found — run \`npm run build\` first.`)
  process.exit(1)
}

const html = fs.readFileSync(indexPath, 'utf8')

const attr = (tag, name) => {
  const m = tag.match(new RegExp(`\\b${name}\\s*=\\s*("([^"]*)"|'([^']*)'|([^\\s>]+))`, 'i'))
  if (!m) return null
  return (m[2] !== undefined ? m[2] : m[3] !== undefined ? m[3] : m[4]) || ''
}
const hasIntegrity = (tag) => /\bintegrity\s*=\s*["']?\s*sha(256|384|512)-/i.test(tag)

const required = [] // must carry integrity
const notApplicable = [] // counted, no body of ours to hash
const unclassified = [] // NEVER silently skipped — see below

for (const m of html.matchAll(/<script\b[^>]*>/gi)) {
  const tag = m[0]
  // Only external scripts need SRI. Inline scripts cannot carry it (and strict
  // CSP forbids them anyway).
  if (attr(tag, 'src') !== null) required.push({ tag, kind: 'script[src]' })
  else notApplicable.push({ tag, kind: 'script (inline)' })
}

for (const m of html.matchAll(/<link\b[^>]*>/gi)) {
  const tag = m[0]
  const rel = (attr(tag, 'rel') || '').trim().toLowerCase()
  if (REL_REQUIRES_SRI.has(rel)) required.push({ tag, kind: `link[rel=${rel}]` })
  else if (REL_NOT_APPLICABLE.has(rel)) notApplicable.push({ tag, kind: `link[rel=${rel}]` })
  else unclassified.push({ tag, kind: `link[rel=${rel || '<none>'}]` })
}

const offenders = required.filter((t) => !hasIntegrity(t.tag))

const problems = []

if (offenders.length > 0) {
  problems.push(
    `${offenders.length} tag(s) missing integrity=:\n` +
      offenders.map((t) => `    ${t.kind}  ${t.tag}`).join('\n') +
      '\n  Make sure vite-plugin-subresource-integrity runs in production builds.'
  )
}

// A rel this file has never seen is not "fine" — it is unmeasured. Reporting it
// as a skip and exiting 0 would be the same silent-subset lie as (a) above.
if (unclassified.length > 0) {
  problems.push(
    `${unclassified.length} tag(s) with a rel this gate does not classify — they were NOT checked:\n` +
      unclassified.map((t) => `    ${t.kind}  ${t.tag}`).join('\n') +
      '\n  Add the rel to REL_REQUIRES_SRI or REL_NOT_APPLICABLE in this file.'
  )
}

if (required.length === 0) {
  problems.push(
    'dist/index.html contains 0 subresource tags that could carry integrity=.\n' +
      '  An empty denominator is not a pass — either the build emitted nothing or\n' +
      '  the tag matcher no longer matches the emitted HTML.'
  )
} else if (required.length < SRI_BASELINE) {
  problems.push(
    `only ${required.length} integrity-carrying tag(s), baseline is ${SRI_BASELINE} — ` +
      'the set of SRI-protected subresources SHRANK.\n' +
      '  Either a subresource lost its tag, or the build stopped emitting it.'
  )
} else if (required.length > SRI_BASELINE) {
  problems.push(
    `${required.length} integrity-carrying tag(s), baseline is ${SRI_BASELINE} — the build ` +
      'gained a subresource.\n' +
      `  That is an improvement, but an unrecorded one: raise SRI_BASELINE to ` +
      `${required.length} in frontend/scripts/verify-sri.cjs in the same commit so the\n` +
      '  new asset is seen, and the floor cannot silently slip back afterwards.'
  )
}

const summary =
  `verify-sri: required: ${required.length} (baseline ${SRI_BASELINE}) · ` +
  `missing integrity: ${offenders.length} · ` +
  `not-applicable: ${notApplicable.length} · unclassified: ${unclassified.length}`

if (problems.length > 0) {
  console.error(summary)
  console.error('\nverify-sri FAILED:')
  for (const p of problems) console.error('  · ' + p)
  process.exit(1)
}

const byKind = required.reduce((acc, t) => ((acc[t.kind] = (acc[t.kind] || 0) + 1), acc), {})
console.log(summary)
console.log(
  '  OK — ' +
    Object.entries(byKind)
      .sort()
      .map(([k, n]) => `${n} ${k}`)
      .join(', ') +
    ' all carry integrity='
)
