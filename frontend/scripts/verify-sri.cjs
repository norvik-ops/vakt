#!/usr/bin/env node
// verify-sri.cjs — fails the build when the Vite production output is
// missing Subresource Integrity hashes on its <script> / <link rel=stylesheet>
// elements.
//
// Audit response F[1]: strict CSP closes the inline-injection path; SRI
// closes the supply-chain path (asset swap at the CDN/proxy layer). If
// either layer regresses, deployments to self-hosters silently lose the
// guarantee — this check makes the regression a CI failure.

const fs = require('fs')
const path = require('path')

const indexPath = path.join(__dirname, '..', 'dist', 'index.html')

if (!fs.existsSync(indexPath)) {
  console.error(`verify-sri: ${indexPath} not found — run \`npm run build\` first.`)
  process.exit(1)
}

const html = fs.readFileSync(indexPath, 'utf8')

const scriptTags = [...html.matchAll(/<script\b[^>]*>/g)].map((m) => m[0])
const linkTags = [...html.matchAll(/<link\b[^>]*rel=["']?stylesheet["']?[^>]*>/g)].map((m) => m[0])

const offenders = []
for (const tag of scriptTags) {
  // Only external scripts (src=) need SRI. Inline scripts can't (and
  // strict CSP forbids them anyway).
  if (/\bsrc=/.test(tag) && !/\bintegrity=/.test(tag)) {
    offenders.push(tag)
  }
}
for (const tag of linkTags) {
  if (!/\bintegrity=/.test(tag)) {
    offenders.push(tag)
  }
}

if (offenders.length > 0) {
  console.error('verify-sri: tags missing integrity= attribute:')
  for (const t of offenders) console.error('  ' + t)
  console.error('\nMake sure vite-plugin-subresource-integrity runs in production builds.')
  process.exit(1)
}

console.log(`verify-sri: OK — ${scriptTags.length} script + ${linkTags.length} stylesheet tag(s) all carry integrity=`)
