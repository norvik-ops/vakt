/**
 * Pro-Plakette im Framework-Katalog (UX-02) — abgeglichen mit dem Go-Code.
 *
 * Liest frameworkFeatureGate (backend/internal/modules/vaktcomply/handler.go),
 * ProTier und UnsoldFeatures (features/tiers.go) und die Konstanten aus
 * license.go. Erwartung je Katalogkarte:
 *   - Merkmal in ProTier      → Plakette „Pro“, Knopf „Aktivieren“ bleibt
 *   - Merkmal unverkauft      → keine Pro-Plakette, „Nicht im Angebot“
 *   - kein Gate (NIS2, ISO)   → weder noch
 * Prüft NICHT: Karten, die im Go-Katalog fehlen, oder den Text der Plakette im Browser.
 */
import { describe, it, expect, vi } from 'vitest'
import { render, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import FrameworksPage from './FrameworksPage'

vi.mock('../hooks/useFrameworks', () => ({
  useFrameworks: () => ({ data: [], isLoading: false, isError: false }),
  useEnableFramework: () => ({ mutate: vi.fn(), isPending: false, error: null }),
  useDeleteFramework: () => ({ mutate: vi.fn(), isPending: false }),
  useSwitchDORAVariant: () => ({ mutate: vi.fn(), isPending: false }),
}))

const BACKEND = join(process.cwd(), '..', 'backend')
const read = (rel: string) => readFileSync(join(BACKEND, rel), 'utf-8')

function constants(): Map<string, string> {
  const out = new Map<string, string>()
  for (const m of read('internal/license/license.go').matchAll(/^\s*(Feature\w+)\s*=\s*"([^"]+)"/gm)) out.set(m[1], m[2])
  return out
}

function tierList(name: string): Set<string> {
  const block = new RegExp(`var ${name} = \\[\\]string\\{([\\s\\S]*?)\\n\\}`).exec(read('internal/shared/platform/features/tiers.go'))
  if (!block) throw new Error(`${name} nicht gefunden`)
  const c = constants()
  // Kommentarzeilen im Block enthalten ebenfalls Feature-Namen — nur Code-Zeilen zählen.
  const code = block[1].split('\n').filter((l) => !l.trim().startsWith('//')).join('\n')
  return new Set([...code.matchAll(/\b(Feature\w+)\b/g)].map((m) => c.get(m[1]) ?? m[1]))
}

function gates(): Map<string, string> {
  const block = /var frameworkFeatureGate = map\[string\]features\.Feature\{([\s\S]*?)\n\}/.exec(read('internal/modules/vaktcomply/handler.go'))
  if (!block) throw new Error('frameworkFeatureGate nicht gefunden')
  const c = constants()
  const out = new Map<string, string>()
  for (const m of block[1].matchAll(/"([^"]+)":\s*features\.(Feature\w+)/g)) out.set(m[1], c.get(m[2]) ?? m[2])
  return out
}

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <FrameworksPage />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('FrameworksPage — Tier-Kennzeichnung der Katalogkarten', () => {
  it('trägt die Pro-Plakette genau auf den Pro-gegateten Frameworks', () => {
    const pro = tierList('ProTier')
    const unsold = tierList('UnsoldFeatures')
    const gate = gates()
    expect(gate.size).toBeGreaterThan(5)

    const { container } = renderPage()
    const cards = [...container.querySelectorAll<HTMLElement>('[data-framework-key]')]
    expect(cards.length).toBeGreaterThan(10)

    let proCards = 0
    for (const card of cards) {
      const key = (card.dataset.frameworkKey ?? '').toUpperCase()
      const feature = gate.get(key)
      const badge = within(card).queryByTestId('tier-pro')
      if (feature && pro.has(feature)) {
        proCards++
        expect(badge, `${key} (${feature}) braucht die Pro-Plakette`).not.toBeNull()
        expect(within(card).getByRole('button', { name: /Aktivieren/ })).toBeEnabled()
      } else if (feature && unsold.has(feature)) {
        expect(badge, `${key} ist unverkauft, keine Pro-Plakette`).toBeNull()
        expect(within(card).getByText('Nicht im Angebot')).toBeInTheDocument()
      } else {
        expect(badge, `${key} ist nicht gegatet`).toBeNull()
      }
    }
    expect(proCards).toBeGreaterThanOrEqual(8)
  })
})
