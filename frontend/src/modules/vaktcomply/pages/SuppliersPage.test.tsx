import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import SuppliersPage from './SuppliersPage'
import type { Supplier } from '../types'
import type { SupplierFilters } from '../hooks/useSuppliers'

// Mock auth store
vi.mock('../../../shared/stores/auth', () => ({
  useAuthStore: (selector: (s: { token: string | null }) => unknown) =>
    selector({ token: 'test-token' }),
}))

const mockExpiredSupplier: Supplier = {
  id: 's-1',
  org_id: 'org-1',
  name: 'Expired Corp GmbH',
  criticality: 'critical',
  nis2_relevant: true,
  dora_relevant: true,
  contract_status: 'expired',
  assessment_status: 'none',
  created_at: '2024-01-01T00:00:00Z',
  updated_at: '2024-01-01T00:00:00Z',
}

const mockExpiringSoonSupplier: Supplier = {
  id: 's-2',
  org_id: 'org-1',
  name: 'Soon Corp GmbH',
  criticality: 'important',
  nis2_relevant: false,
  dora_relevant: true,
  contract_status: 'expiring_soon',
  assessment_status: 'pending',
  created_at: '2024-01-01T00:00:00Z',
  updated_at: '2024-01-01T00:00:00Z',
}

const mockActiveSupplier: Supplier = {
  id: 's-3',
  org_id: 'org-1',
  name: 'Active Corp GmbH',
  criticality: 'standard',
  nis2_relevant: false,
  dora_relevant: false,
  contract_status: 'active',
  assessment_status: 'completed',
  created_at: '2024-01-01T00:00:00Z',
  updated_at: '2024-01-01T00:00:00Z',
}

const mockUseSuppliersResult = {
  data: [mockExpiredSupplier, mockExpiringSoonSupplier, mockActiveSupplier],
  isLoading: false,
  isError: false,
}
// eslint-disable-next-line @typescript-eslint/no-unused-vars
const mockUseSuppliers = vi.fn((_filters?: SupplierFilters) => mockUseSuppliersResult)

vi.mock('../hooks/useSuppliers', () => ({
  useSuppliers: (filters?: SupplierFilters) => mockUseSuppliers(filters),
  useCreateSupplier: () => ({ mutate: vi.fn(), isPending: false }),
  useUpdateSupplier: () => ({ mutate: vi.fn(), isPending: false }),
  useDeleteSupplier: () => ({ mutate: vi.fn(), isPending: false }),
  useImportSuppliersCSV: () => ({ mutate: vi.fn(), isPending: false }),
}))

vi.mock('../hooks/useAssessments', () => ({
  useSupplierStatus: (id: string) => ({
    data: id === 's-1'
      ? { supplier_id: id, status: 'red', score: 0, details: {} }
      : id === 's-2'
        ? { supplier_id: id, status: 'yellow', score: 50, details: {} }
        : { supplier_id: id, status: 'green', score: 100, details: {} },
  }),
  statusToVariant: (status: string) =>
    status === 'green' ? 'success' : status === 'yellow' ? 'warning' : 'destructive',
}))

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <SuppliersPage />
    </QueryClientProvider>,
  )
}

describe('SuppliersPage', () => {
  it('renders contract-status-badge for expired supplier', () => {
    renderPage()
    const badges = screen.getAllByTestId('contract-status-badge')
    expect(badges.length).toBeGreaterThan(0)
    const expiredBadge = badges.find((b) => b.textContent === 'Abgelaufen')
    expect(expiredBadge).toBeTruthy()
  })

  it('renders contract-status-badge for expiring_soon supplier', () => {
    renderPage()
    const badges = screen.getAllByTestId('contract-status-badge')
    const expiringSoonBadge = badges.find((b) => b.textContent === 'Läuft ab')
    expect(expiringSoonBadge).toBeTruthy()
  })

  it('renders contract-status-badge for active supplier', () => {
    renderPage()
    const badges = screen.getAllByTestId('contract-status-badge')
    const activeBadge = badges.find((b) => b.textContent === 'Aktiv')
    expect(activeBadge).toBeTruthy()
  })

  it('renders CSV exportieren button', () => {
    renderPage()
    const btn = screen.getByText('CSV exportieren')
    expect(btn).toBeTruthy()
  })

  it('shows supplier names', () => {
    renderPage()
    expect(screen.getByText('Expired Corp GmbH')).toBeTruthy()
    expect(screen.getByText('Soon Corp GmbH')).toBeTruthy()
    expect(screen.getByText('Active Corp GmbH')).toBeTruthy()
  })

  // Story 29.1: filter toolbar
  it('renders filter toolbar with criticality and assessment_status selects', () => {
    renderPage()
    const toolbar = screen.getByTestId('filter-toolbar')
    expect(toolbar).toBeTruthy()
    // The toolbar contains the filter labels
    expect(toolbar.textContent).toContain('Kritikalität')
    expect(toolbar.textContent).toContain('Bewertungsstatus')
  })

  // Story 29.1: assessment-status badges
  it('renders assessment-status-badge with none for expired supplier', () => {
    renderPage()
    const badges = screen.getAllByTestId('assessment-status-badge')
    expect(badges.length).toBeGreaterThan(0)
    const noneBadge = badges.find((b) => b.textContent === 'Nicht bewertet')
    expect(noneBadge).toBeTruthy()
  })

  it('renders assessment-status-badge with pending variant', () => {
    renderPage()
    const badges = screen.getAllByTestId('assessment-status-badge')
    const pendingBadge = badges.find((b) => b.textContent === 'Ausstehend')
    expect(pendingBadge).toBeTruthy()
    // pending should have amber styling
    expect(pendingBadge?.className).toContain('amber')
  })

  it('renders assessment-status-badge with completed variant', () => {
    renderPage()
    const badges = screen.getAllByTestId('assessment-status-badge')
    const completedBadge = badges.find((b) => b.textContent === 'Abgeschlossen')
    expect(completedBadge).toBeTruthy()
    // completed should have green styling
    expect(completedBadge?.className).toContain('green')
  })

  // Story 29.1: CSV import button
  it('renders CSV importieren button', () => {
    renderPage()
    const btn = screen.getByTestId('import-csv-button')
    expect(btn).toBeTruthy()
    expect(btn.textContent).toContain('CSV importieren')
  })

  it('renders hidden file input for CSV import', () => {
    renderPage()
    const input = screen.getByTestId('csv-file-input')
    expect(input).toBeTruthy()
  })

  // Story 29.1: filter values flow to useSuppliers
  it('calls useSuppliers with no active filters on initial render', () => {
    mockUseSuppliers.mockClear()
    renderPage()
    // Initial state: both filters are empty — hook receives undefined for both
    const calls = mockUseSuppliers.mock.calls
    const lastCall = calls[calls.length - 1][0]
    expect(lastCall?.criticality).toBeUndefined()
    expect(lastCall?.assessmentStatus).toBeUndefined()
  })

  it('hides reset-filter button when no filters are active', () => {
    renderPage()
    // "Filter zurücksetzen" button only appears when at least one filter is set
    expect(screen.queryByText('Filter zurücksetzen')).toBeNull()
  })

  // Story 29.4: traffic-light status badges
  it('renders supplier-status-badge for each supplier', () => {
    renderPage()
    const badges = screen.getAllByTestId('supplier-status-badge')
    expect(badges.length).toBeGreaterThanOrEqual(3)
  })

  it('supplier-status-badge shows red for s-1', () => {
    renderPage()
    const badges = screen.getAllByTestId('supplier-status-badge')
    const redBadge = badges.find((b) => b.getAttribute('data-status') === 'red')
    expect(redBadge).toBeTruthy()
    expect(redBadge?.textContent).toBe('Rot')
  })

  it('supplier-status-badge shows green for s-3', () => {
    renderPage()
    const badges = screen.getAllByTestId('supplier-status-badge')
    const greenBadge = badges.find((b) => b.getAttribute('data-status') === 'green')
    expect(greenBadge).toBeTruthy()
    expect(greenBadge?.textContent).toBe('Grün')
  })
})
