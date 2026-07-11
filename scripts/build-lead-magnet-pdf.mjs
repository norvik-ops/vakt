/**
 * Rendert einen Lead-Magneten aus docs/marketing/lead-magnets/*.md in ein
 * gebrandetes PDF unter sites/vakt/public/downloads/.
 *
 * Bewusst NICHT im CI-Build: Das PDF ist ein Marketing-Asset, das sich selten
 * ändert; Playwright im Sites-Build zu installieren wäre unverhältnismäßig.
 * Das erzeugte PDF ist eingecheckt — dieses Skript macht die Erzeugung
 * reproduzierbar, wenn die Quell-Markdown sich ändert.
 *
 * Voraussetzung: Playwright-Chromium (npx playwright install chromium)
 *
 *   node scripts/build-lead-magnet-pdf.mjs nis2-checkliste
 */
import { readFileSync, mkdirSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { createRequire } from 'node:module'

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const require = createRequire(import.meta.url)

// Playwright ist bewusst KEINE Dependency der Sites — sonst zieht jeder
// CI-Site-Build die Browser-Binaries mit. Das Skript sucht es stattdessen
// dort, wo es realistisch liegt, und sagt sonst klar, was zu tun ist.
function loadChromium() {
  const paths = [process.env.PLAYWRIGHT_DIR, ROOT, resolve(ROOT, 'sites/vakt')].filter(Boolean)
  for (const p of paths) {
    try {
      return require(require.resolve('playwright', { paths: [p] })).chromium
    } catch {
      /* nächster Kandidat */
    }
  }
  throw new Error(
    'Playwright nicht gefunden.\n' +
      '  Entweder:  npm i -D playwright && npx playwright install chromium\n' +
      '  Oder:      PLAYWRIGHT_DIR=/pfad/zu/projekt/mit/playwright node scripts/build-lead-magnet-pdf.mjs <slug>',
  )
}
const chromium = loadChromium()
const slug = process.argv[2]
if (!slug) {
  console.error('Usage: node scripts/build-lead-magnet-pdf.mjs <slug>')
  process.exit(1)
}

const src = resolve(ROOT, `docs/marketing/lead-magnets/${slug}.md`)
const out = resolve(ROOT, `sites/vakt/public/downloads/${slug}.pdf`)
mkdirSync(dirname(out), { recursive: true })

// Die Quell-Markdown trägt oben eine interne Redaktionsnotiz als Blockquote
// (Build-Anweisung, Sprint-Referenz, ADR-Verweise). Die darf nicht ins Kunden-PDF.
//
// Bewusst STRUKTURELL abgegrenzt statt über Stichwörter: Alles vor dem ersten
// `---` ist Redaktionsbereich (H1 + interne Notiz), danach beginnt der
// Kundeninhalt. Eine Stichwortliste wäre still gebrochen, sobald jemand die
// Notiz umformuliert — genau das ist beim Schreiben dieses Skripts passiert.
function stripInternalNote(raw) {
  const lines = raw.split('\n')
  const sep = lines.findIndex((l) => /^---+$/.test(l.trim()))
  if (sep === -1) return raw // kein Trenner → nichts zu strippen
  const head = lines.slice(0, sep).filter((l) => !l.trimStart().startsWith('>'))
  return [...head, ...lines.slice(sep)].join('\n')
}

const md = stripInternalNote(readFileSync(src, 'utf-8'))

const esc = (s) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
const inline = (s) =>
  esc(s)
    .replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
    .replace(/`(.+?)`/g, '<code>$1</code>')
    .replace(/(?<!\*)\*(?!\*)(.+?)(?<!\*)\*(?!\*)/g, '<em>$1</em>')

// Minimaler Markdown-Renderer — deckt genau das ab, was die Lead-Magneten
// nutzen (Überschriften, Checkbox-Listen, Aufzählungen, Absätze, hr, Zitate).
const html = []
let inList = false
const closeList = () => { if (inList) { html.push('</ul>'); inList = false } }

for (const raw of md.split('\n')) {
  const line = raw.trimEnd()
  if (!line.trim()) { closeList(); continue }

  const cb = line.match(/^- \[ \] (.*)$/)
  if (cb) {
    if (!inList) { html.push('<ul class="checks">'); inList = true }
    html.push(`<li><span class="box"></span><span>${inline(cb[1])}</span></li>`)
    continue
  }
  const li = line.match(/^[-*] (.*)$/)
  if (li) {
    if (!inList) { html.push('<ul class="bullets">'); inList = true }
    html.push(`<li>${inline(li[1])}</li>`)
    continue
  }
  closeList()

  const h = line.match(/^(#{1,4}) (.*)$/)
  if (h) { html.push(`<h${h[1].length}>${inline(h[2])}</h${h[1].length}>`); continue }
  if (/^---+$/.test(line)) { html.push('<hr/>'); continue }
  if (line.startsWith('> ')) { html.push(`<blockquote>${inline(line.slice(2))}</blockquote>`); continue }
  html.push(`<p>${inline(line)}</p>`)
}
closeList()

const page = `
<style>
  @page { size: A4; margin: 20mm 16mm 18mm; }
  * { box-sizing: border-box; }
  body { font-family: -apple-system, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
         color: #1e293b; font-size: 10.5pt; line-height: 1.55; margin: 0; }
  .brand { display: flex; align-items: center; justify-content: space-between;
           border-bottom: 3px solid #6366f1; padding-bottom: 10px; margin-bottom: 22px; }
  .brand .name { font-size: 17pt; font-weight: 800; color: #0f0f1a; letter-spacing: -0.02em; }
  .brand .name span { color: #6366f1; }
  .brand .meta { font-size: 8pt; color: #64748b; text-align: right; line-height: 1.4; }
  h1 { font-size: 19pt; color: #0f0f1a; margin: 0 0 4px; letter-spacing: -0.02em; }
  h2 { font-size: 13pt; color: #0f0f1a; margin: 22px 0 8px; padding-top: 10px;
       border-top: 1px solid #e2e8f0; page-break-after: avoid; }
  h3 { font-size: 11pt; color: #4338ca; margin: 16px 0 6px; page-break-after: avoid; }
  h4 { font-size: 10pt; color: #334155; margin: 12px 0 4px; page-break-after: avoid; }
  p { margin: 6px 0; }
  hr { border: 0; border-top: 1px solid #e2e8f0; margin: 18px 0; }
  blockquote { margin: 10px 0; padding: 8px 12px; background: #f1f5f9;
               border-left: 3px solid #6366f1; color: #475569; font-size: 9.5pt; }
  code { background: #f1f5f9; padding: 1px 4px; border-radius: 3px;
         font-family: ui-monospace, Menlo, Consolas, monospace; font-size: 9pt; }
  ul { margin: 6px 0; padding: 0; list-style: none; }
  ul.bullets li { position: relative; padding-left: 16px; margin: 3px 0; }
  ul.bullets li::before { content: "•"; position: absolute; left: 4px; color: #6366f1; }
  ul.checks li { display: flex; gap: 8px; align-items: flex-start; margin: 5px 0;
                 page-break-inside: avoid; }
  ul.checks .box { flex: 0 0 auto; width: 11px; height: 11px; margin-top: 3px;
                   border: 1.5px solid #94a3b8; border-radius: 2.5px; }
  .foot { margin-top: 26px; padding-top: 10px; border-top: 1px solid #e2e8f0;
          font-size: 8pt; color: #94a3b8; }
</style>

<div class="brand">
  <div class="name">Vakt<span>.</span></div>
  <div class="meta">Selbst gehostetes ISMS<br/>vakt.norvikops.de</div>
</div>

${html.join('\n')}

<div class="foot">
  Norvik Ops UG · vakt.norvikops.de · hello@norvikops.de — Diese Checkliste ist frei
  weitergebbar. Keine Anmeldung, keine E-Mail-Adresse nötig.
</div>
`

const browser = await chromium.launch()
const p = await browser.newPage()
await p.setContent(page, { waitUntil: 'load' })
await p.pdf({
  path: out,
  format: 'A4',
  printBackground: true,
  displayHeaderFooter: true,
  headerTemplate: '<div></div>',
  footerTemplate:
    '<div style="width:100%;font-size:7pt;color:#94a3b8;padding:0 16mm;text-align:right;">' +
    'Vakt — NIS2-Checkliste · Seite <span class="pageNumber"></span>/<span class="totalPages"></span></div>',
  margin: { top: '20mm', bottom: '18mm', left: '16mm', right: '16mm' },
})
await browser.close()
console.log('PDF geschrieben:', out)
