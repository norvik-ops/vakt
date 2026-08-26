import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import AuditorPortalPage from './AuditorPortalPage'
import type {
  AuditorControl,
  CursorMeta,
  Paginated,
  useAuditorControls,
  useAuditorRisks,
} from '../hooks/useAuditorPortal'

// ---------------------------------------------------------------------------
// R1-11-D01 — the auditor controls endpoint answers with a pagination ENVELOPE
// ({data, pagination}), never a bare array. Both branches of
// vaktcomply.Handler.ListControls wrap: pagination.WrapCursor (cursor mode,
// the default) and pagination.Wrap (?page= mode). The hook used to type it as
// AuditorControl[], so `const { data: controls = [] }` never applied its
// default — an envelope is truthy — and `controls.map()` threw a TypeError.
//
// The fixtures below deliberately mirror the REAL server shape. Flattening
// them to a bare array would make the regression invisible.
// ---------------------------------------------------------------------------

const FRAMEWORKS = [
  {
    id: 'fw-1',
    name: 'ISO 27001',
    version: '2022',
    is_builtin: true,
    readiness_score: 42,
    created_at: '2026-01-01T00:00:00Z',
  },
]

const CONTROLS_ENVELOPE = {
  data: [
    {
      id: 'ctrl-1',
      control_id: 'A.5.1',
      title: 'Richtlinien für Informationssicherheit',
      description: '',
      domain: 'Organisatorisch',
      status: 'implemented',
      manual_status: '',
    },
  ],
  pagination: { limit: 25, has_more: false },
}

// ListRisks is dual-branch and no `?page=` is sent, so the cursor branch runs:
// the meta is {limit, next_cursor?, has_more}, NOT {page, total, total_pages}.
// Using offset meta here would bake in the very mistake this file guards against.
const RISKS_ENVELOPE = { data: [], pagination: { limit: 25, has_more: false } }
// ListIncidents / ListPolicies wrap unconditionally with offset meta.
const OFFSET_ENVELOPE = { data: [], pagination: { page: 1, limit: 25, total: 0, total_pages: 1 } }

function mockAuditorApi() {
  // auditorFetch always calls fetch() with a template-string URL, so a string
  // parameter is the honest signature here.
  const fetchMock = vi.fn((url: string) => {
    let body: unknown = null
    if (url.includes('/frameworks/') && url.includes('/controls')) {
      body = CONTROLS_ENVELOPE
    } else if (url.endsWith('/frameworks')) {
      body = FRAMEWORKS
    } else if (url.endsWith('/risks')) {
      body = RISKS_ENVELOPE
    } else {
      body = OFFSET_ENVELOPE
    }
    return Promise.resolve({
      ok: true,
      status: 200,
      json: () => Promise.resolve(body),
    })
  })
  vi.stubGlobal('fetch', fetchMock)
  return fetchMock
}

function renderPortal() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <AuditorPortalPage />
    </QueryClientProvider>,
  )
}

describe('AuditorPortalPage — controls envelope (R1-11-D01)', () => {
  beforeEach(() => {
    sessionStorage.setItem('auditor_session_token', 'auditor-test-token')
    mockAuditorApi()
  })

  afterEach(() => {
    sessionStorage.clear()
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  it('renders the control list after expanding a framework row', async () => {
    renderPortal()

    await waitFor(() => {
      expect(screen.getByText('ISO 27001')).toBeInTheDocument()
    })

    // Expanding triggers useAuditorControls. With the old bare-array typing
    // this render threw `controls.map is not a function`.
    fireEvent.click(screen.getByText('ISO 27001'))

    await waitFor(() => {
      expect(screen.getByText('A.5.1')).toBeInTheDocument()
    })
    expect(
      screen.getByText('Richtlinien für Informationssicherheit'),
    ).toBeInTheDocument()
  })
})

// ---------------------------------------------------------------------------
// COMPILE-TIME contract ratchet.
//
// An earlier version of this block asserted `Array.isArray(data) === false` at
// runtime. That was vacuous: it only inspected the shape of this file's own
// mock, so it stayed green even with the hook re-typed as a bare array — it
// passed in the rolled-back run that the render test correctly failed.
//
// The real contract here is a TYPE, so it has to be enforced by `tsc`, which
// runs in the DoD chain and in CI. Vitest never typechecks, so these are
// deliberately type-only declarations: they make `npm run typecheck` / `tsc -b`
// fail the moment the hooks are re-typed, and cost nothing at runtime.
// ---------------------------------------------------------------------------

// NOTE ON INSTRUMENT CHOICE: `@ts-expect-error` is the obvious tool here and it
// is the WRONG one under this tsconfig. `noUnusedLocals: true` means an unused
// declaration is itself an error, so the directive is "satisfied" by that error
// alone and stays green no matter how the hook is typed — vacuous in exactly the
// way the runtime check it replaced was vacuous. Measured, not assumed: an
// unused `@ts-expect-error` on a deliberately-broken probe did not turn tsc red.
//
// So the locks below are EXACT-TYPE EQUALITIES that resolve to `false` on
// regression and are then assigned to `true`, which is a hard assignment error.
// Each one is asserted at runtime too, so nothing here is dead code.

type Exact<A, B> = [A] extends [B] ? ([B] extends [A] ? true : false) : false

type ControlsData = NonNullable<ReturnType<typeof useAuditorControls>['data']>
type RisksMeta = NonNullable<ReturnType<typeof useAuditorRisks>['data']>['pagination']

// (1) Controls must be the cursor envelope, not a bare array. Re-typing the hook
//     as `AuditorControl[]` collapses Exact<> to `false` and fails the build.
const controlsAreACursorEnvelope: Exact<ControlsData, Paginated<AuditorControl, CursorMeta>> = true

// (2) Risks travels the CURSOR branch (no `?page=` is sent), so its meta is
//     CursorMeta. Reverting it to OffsetMeta — the A3 defect, which promised a
//     `pagination.total` that is `undefined` at runtime — fails the build.
const risksMetaIsCursorNotOffset: Exact<RisksMeta, CursorMeta> = true

describe('auditor hook contract (type-level)', () => {
  it('is enforced by tsc — these consts cannot be `true` if the typing regresses', () => {
    expect(controlsAreACursorEnvelope).toBe(true)
    expect(risksMetaIsCursorNotOffset).toBe(true)
  })
})
