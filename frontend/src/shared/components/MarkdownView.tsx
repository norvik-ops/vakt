import { Fragment, type ReactNode } from 'react'
import { cn } from '../../lib/utils'
import { parseMarkdownBlocks, safeHref, scanInline } from '../utils/markdown'

/**
 * Minimaler, sicherer Markdown-Renderer für Richtlinientexte (UX-23).
 *
 * Warum selbst gebaut: Das Frontend hat keinen Markdown-Renderer im Lockfile, und
 * die Richtlinienvorlagen (policy_templates_seed.go) brauchen nur einen kleinen
 * Ausschnitt: Überschriften, Listen, Absätze, Tabellen, Zitate, Trennlinien,
 * fett/kursiv, Code und Links. Eine neue Abhängigkeit samt Sanitizer wäre für
 * diesen Ausschnitt teurer als diese Datei.
 *
 * Warum sicher: Der Parser erzeugt React-Elemente, nie einen HTML-String. Es gibt
 * keine rohe HTML-Einspeisung in das DOM. Roher HTML-Code im Text (`<script>`, `<img onerror>`)
 * bleibt deshalb sichtbarer Text, den React escaped. Links bekommen nur dann ein
 * `href`, wenn das Ziel http(s), mailto oder ein Pfad in Vakt ist — `javascript:`,
 * `data:` und Ähnliches werden als reiner Text dargestellt.
 *
 * Was er NICHT kann: verschachtelte Listen (eingerückte Punkte erscheinen als
 * Punkte derselben Liste), Bilder, Fußnoten, HTML-Blöcke.
 */

function renderInline(text: string, key: string): ReactNode[] {
  return scanInline(text).map((tok, n) => {
    const k = `${key}-${n.toString()}`
    switch (tok.kind) {
      case 'text':
        return <Fragment key={k}>{tok.text}</Fragment>
      case 'code':
        return <code key={k} className="px-1 py-0.5 rounded bg-surface2 text-[0.85em] font-mono">{tok.text}</code>
      case 'strong':
        return <strong key={k} className="font-semibold text-primary">{renderInline(tok.text, k)}</strong>
      case 'em':
        return <em key={k}>{renderInline(tok.text, k)}</em>
      case 'link': {
        const href = safeHref(tok.url)
        const label = renderInline(tok.text, k)
        if (!href) return <Fragment key={k}>{label}</Fragment>
        const external = /^https?:/i.test(href)
        return (
          <a
            key={k}
            href={href}
            className="text-brand underline underline-offset-2 hover:text-brand/80"
            {...(external ? { target: '_blank', rel: 'noopener noreferrer' } : {})}
          >
            {label}
          </a>
        )
      }
    }
  })
}

const HEADING_CLASS: Record<number, string> = {
  1: 'text-lg font-semibold text-primary mt-1',
  2: 'text-base font-semibold text-primary mt-4',
  3: 'text-sm font-semibold text-primary mt-3',
  4: 'text-sm font-medium text-primary mt-2',
  5: 'text-sm font-medium text-primary mt-2',
  6: 'text-sm font-medium text-secondary mt-2',
}

export function MarkdownView({ source, className }: { source: string; className?: string }) {
  const blocks = parseMarkdownBlocks(source)
  return (
    <div className={cn('space-y-2 text-sm text-primary leading-relaxed break-words', className)} data-testid="markdown-view">
      {blocks.map((b, idx) => {
        const key = `b${idx.toString()}`
        switch (b.kind) {
          case 'heading': {
            const Tag = `h${Math.min(b.level + 1, 6).toString()}` as 'h2'
            return <Tag key={key} className={HEADING_CLASS[b.level]}>{renderInline(b.text, key)}</Tag>
          }
          case 'paragraph':
            return <p key={key}>{renderInline(b.text, key)}</p>
          case 'quote':
            return (
              <blockquote key={key} className="border-l-2 border-border pl-3 text-secondary">
                {renderInline(b.text, key)}
              </blockquote>
            )
          case 'code':
            return (
              <pre key={key} className="p-3 rounded bg-surface2 text-xs font-mono overflow-x-auto whitespace-pre">
                {b.text}
              </pre>
            )
          case 'hr':
            return <hr key={key} className="border-border" />
          case 'list': {
            const items = b.items.map((it, j) => <li key={`${key}-${j.toString()}`}>{renderInline(it, `${key}-${j.toString()}`)}</li>)
            return b.ordered
              ? <ol key={key} className="list-decimal pl-5 space-y-1">{items}</ol>
              : <ul key={key} className="list-disc pl-5 space-y-1">{items}</ul>
          }
          case 'table':
            return (
              <div key={key} className="overflow-x-auto">
                <table className="w-full text-xs border border-border">
                  <thead className="bg-surface2">
                    <tr>
                      {b.header.map((c, j) => (
                        <th key={j} className="text-left font-semibold px-2 py-1.5 border-b border-border">{renderInline(c, `${key}-h${j.toString()}`)}</th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {b.rows.map((r, ri) => (
                      <tr key={ri} className="border-b border-border last:border-0">
                        {r.map((c, j) => (
                          <td key={j} className="px-2 py-1.5 align-top">{renderInline(c, `${key}-${ri.toString()}-${j.toString()}`)}</td>
                        ))}
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )
        }
      })}
    </div>
  )
}
