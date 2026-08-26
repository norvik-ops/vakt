import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import AuditLogPage from './AuditLogPage'
import { fetchAllAuditLogEntries, type AuditLogEntry } from '../hooks/useAuditLog'
import { useAuthStore } from '../shared/stores/auth'
import { mockApiServer, type ApiServerHandle } from '../test-utils/apiServer'

/**
 * R1-20-A7 — the CSV export handed out 25 of 81 entries with no hint of it.
 *
 * The button serialised whatever page the table was holding. The file looked
 * like a complete audit log and was not; an auditor taking it as evidence had
 * no way to notice. That makes the defect worse than a crash, so the tests
 * below check the two things that matter for evidence: every matching row is
 * in the file, and the filters the user set are the ones the file reflects.
 */

const TOTAL = 81
const PAGE_LIMIT = 500

function entry(i: number): AuditLogEntry {
  return {
    id: `a${String(i)}`,
    org_id: 'o1',
    user_email: `user${String(i)}@example.com`,
    action: 'update',
    resource_type: 'control',
    ip_address: `10.0.0.${String(i % 250)}`,
    created_at: '2026-08-01T10:00:00Z',
  }
}

let server: ApiServerHandle
let downloaded: Blob[] = []
// .bind(URL) rather than a bare read: @typescript-eslint/unbound-method
// flags detaching a method from its object, and binding is its stated remedy.
const origCreateObjectURL = URL.createObjectURL.bind(URL)
const origRevokeObjectURL = URL.revokeObjectURL.bind(URL)

beforeEach(() => {
  downloaded = []
  useAuthStore.setState({
    user: { id: 'u1', email: 'admin@example.com', display_name: 'Admin', roles: ['Admin'] },
    hydrating: false,
  })

  // Capture what the export would have written to disk. Only these two statics
  // are replaced — stubbing the whole URL global would take the constructor
  // with it, which apiFetch and the tests below both need.
  URL.createObjectURL = (blob: Blob) => { downloaded.push(blob); return 'blob:stub' }
  URL.revokeObjectURL = () => { /* nothing to release for a stubbed URL */ }
  vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => { /* no navigation in jsdom */ })
})

afterEach(() => {
  server.restore()
  URL.createObjectURL = origCreateObjectURL
  URL.revokeObjectURL = origRevokeObjectURL
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

async function csvLines(): Promise<string[]> {
  const text = await downloaded[0].text()
  return text.replace('\ufeff', '').split('\n')
}

/**
 * Quote-aware field split. A naive `line.split(',')` is wrong here: the
 * formatted timestamp contains a comma and is therefore quoted, so splitting
 * on every comma would count seven fields in a six-field row and make the
 * column-count assertion below fail for the wrong reason.
 */
function csvFields(line: string): string[] {
  const fields: string[] = []
  let current = ''
  let inQuotes = false
  for (let i = 0; i < line.length; i++) {
    const ch = line[i]
    if (ch === '"') {
      if (inQuotes && line[i + 1] === '"') { current += '"'; i++ } else { inQuotes = !inQuotes }
    } else if (ch === ',' && !inQuotes) {
      fields.push(current); current = ''
    } else {
      current += ch
    }
  }
  fields.push(current)
  return fields
}

function armServer(total = TOTAL) {
  server = mockApiServer({
    'GET /audit-log': (req) => {
      const q = new URL(req.url, 'http://x').searchParams
      const limit = Number(q.get('limit') ?? '25')
      const offset = Number(q.get('offset') ?? '0')
      const all = Array.from({ length: total }, (_, i) => entry(i))
      return { body: { entries: all.slice(offset, offset + limit), total } }
    },
  })
}

function renderPage() {
  return render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <MemoryRouter>
        <AuditLogPage />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('fetchAllAuditLogEntries (R1-20-A7)', () => {
  it('returns every matching entry, not the 25 the table is showing', async () => {
    armServer()
    const res = await fetchAllAuditLogEntries()
    expect(res.entries).toHaveLength(TOTAL)
    expect(res.total).toBe(TOTAL)
    expect(res.complete).toBe(true)
    expect(server.requests[0].url).toContain(`limit=${String(PAGE_LIMIT)}`)
  })

  it('walks past the server page cap for a log larger than one page', async () => {
    armServer(1200)
    const res = await fetchAllAuditLogEntries()
    expect(res.entries).toHaveLength(1200)
    expect(server.requests).toHaveLength(3)
  })

  it('carries the active filters into every page request', async () => {
    armServer()
    await fetchAllAuditLogEntries({ from: '2026-01-01T00:00:00Z', action: 'delete' })
    const q = new URL(server.requests[0].url, 'http://x').searchParams
    expect(q.get('from')).toBe('2026-01-01T00:00:00Z')
    expect(q.get('action')).toBe('delete')
  })
})

describe('AuditLogPage export button (R1-20-A7)', () => {
  it('writes all 81 rows, not the 25 on screen', async () => {
    armServer()
    renderPage()

    const button = await screen.findByRole('button', { name: /CSV/i })
    await waitFor(() => { expect(button).not.toBeDisabled() })
    fireEvent.click(button)

    await waitFor(() => { expect(downloaded).toHaveLength(1) })
    // Header line plus one line per entry.
    const lines = await csvLines()
    expect(lines).toHaveLength(TOTAL + 1)
    expect(lines[lines.length - 1]).toContain('user80@example.com')
  })

  it('names every column it writes — the IP used to sit under no heading', async () => {
    armServer(2)
    renderPage()

    const button = await screen.findByRole('button', { name: /CSV/i })
    await waitFor(() => { expect(button).not.toBeDisabled() })
    fireEvent.click(button)

    await waitFor(() => { expect(downloaded).toHaveLength(1) })
    const lines = await csvLines()
    expect(csvFields(lines[0])).toHaveLength(csvFields(lines[1]).length)
    // The unlabelled column was the IP, so name it explicitly rather than
    // relying on the counts alone.
    const headerFields = csvFields(lines[0])
    const rowFields = csvFields(lines[1])
    expect(headerFields[headerFields.length - 1]).toBe('IP-Adresse')
    expect(rowFields[rowFields.length - 1]).toMatch(/^10\.0\.0\./)
  })
})
