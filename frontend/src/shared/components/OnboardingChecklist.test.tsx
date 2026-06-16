import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { MemoryRouter } from 'react-router-dom'
import { OnboardingChecklist } from './OnboardingChecklist'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (k: string) => k }),
}))

const mockDismiss = vi.fn()
const mockUseQuery = vi.fn()
vi.mock('@tanstack/react-query', () => ({
  useQuery: () => mockUseQuery(),
  useMutation: () => ({ mutate: mockDismiss }),
  useQueryClient: () => ({ invalidateQueries: vi.fn() }),
}))

function wrapper({ children }: { children: React.ReactNode }) {
  return <MemoryRouter>{children}</MemoryRouter>
}

const progress = {
  steps: [
    { key: 'scope', done: true, path: '/vaktcomply/isms-scope' },
    { key: 'assets', done: false, path: '/vaktscan/assets' },
    { key: 'risks', done: true, path: '/vaktcomply/risks' },
    { key: 'framework', done: false, path: '/vaktcomply/frameworks' },
  ],
  completed_count: 2,
  total: 4,
  percent_done: 50,
  dismissed: false,
  all_complete: false,
}

describe('OnboardingChecklist', () => {
  beforeEach(() => {
    mockDismiss.mockClear()
  })

  it('renders nothing while loading', () => {
    mockUseQuery.mockReturnValue({ data: undefined, isLoading: true, isError: false })
    const { container } = render(<OnboardingChecklist />, { wrapper })
    expect(container.firstChild).toBeNull()
  })

  it('renders nothing once dismissed', () => {
    mockUseQuery.mockReturnValue({ data: { ...progress, dismissed: true }, isLoading: false, isError: false })
    const { container } = render(<OnboardingChecklist />, { wrapper })
    expect(container.firstChild).toBeNull()
  })

  it('renders steps with done detection + links', () => {
    mockUseQuery.mockReturnValue({ data: progress, isLoading: false, isError: false })
    render(<OnboardingChecklist />, { wrapper })
    expect(screen.getByTestId('onboarding-checklist')).toBeTruthy()
    // Each step links to its real route.
    const scope = screen.getByTestId('onboarding-step-scope')
    expect(scope.getAttribute('href')).toBe('/vaktcomply/isms-scope')
    const assets = screen.getByTestId('onboarding-step-assets')
    expect(assets.getAttribute('href')).toBe('/vaktscan/assets')
    // Progress bar reflects percentage.
    expect(screen.getByTestId('onboarding-progress-bar').getAttribute('style')).toContain('50%')
  })

  it('dismisses on button click', () => {
    mockUseQuery.mockReturnValue({ data: progress, isLoading: false, isError: false })
    render(<OnboardingChecklist />, { wrapper })
    fireEvent.click(screen.getByTestId('onboarding-dismiss'))
    expect(mockDismiss).toHaveBeenCalled()
  })
})
