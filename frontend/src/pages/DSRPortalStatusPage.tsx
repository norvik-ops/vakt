import { useParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { useFormatDate } from '../shared/hooks/useFormatDate'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface DSR {
  id: string
  org_id: string
  requester_name: string
  requester_email: string
  type: string
  description?: string
  status: string
  due_date?: string
  received_at: string
  completed_at?: string
  notes?: string
  created_at: string
  updated_at: string
}

// ---------------------------------------------------------------------------
// API helper
// ---------------------------------------------------------------------------

async function fetchDSRStatus(token: string): Promise<DSR> {
  const res = await fetch(`/api/v1/vaktprivacy/dsr-portal/status/${token}`, {
    headers: { Accept: 'application/json' },
  })
  if (res.status === 404) throw new Error('NOT_FOUND')
  if (!res.ok) throw new Error('FETCH_FAILED')
  return res.json() as Promise<DSR>
}

// ---------------------------------------------------------------------------
// Hooks for label maps
// ---------------------------------------------------------------------------

function useTypeLabels(): Record<string, string> {
  const { t } = useTranslation()
  return {
    access: t('dsr.status.typeAccess'),
    erasure: t('dsr.status.typeErasure'),
    rectification: t('dsr.status.typeRectification'),
    objection: t('dsr.status.typeObjection'),
    portability: t('dsr.status.typePortability'),
  }
}

function useStatusLabels(): Record<string, { label: string; color: string }> {
  const { t } = useTranslation()
  return {
    open: { label: t('dsr.status.statusOpen'), color: 'bg-yellow-100 text-yellow-800' },
    in_progress: { label: t('dsr.status.statusInProgress'), color: 'bg-blue-100 text-blue-800' },
    completed: { label: t('dsr.status.statusCompleted'), color: 'bg-green-100 text-green-800' },
    rejected: { label: t('dsr.status.statusRejected'), color: 'bg-red-100 text-red-800' },
  }
}

// ---------------------------------------------------------------------------
// Sub-components
// ---------------------------------------------------------------------------

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex justify-between text-sm">
      <span className="text-gray-500">{label}</span>
      <span className="text-gray-800 font-medium">{value}</span>
    </div>
  )
}

// ---------------------------------------------------------------------------
// DSRPortalStatusPage
// ---------------------------------------------------------------------------

export default function DSRPortalStatusPage() {
  const { t } = useTranslation()
  const { token } = useParams<{ token: string }>()
  const { formatDate } = useFormatDate()
  const typeLabels = useTypeLabels()
  const statusLabels = useStatusLabels()

  const { data: dsr, isLoading, isError } = useQuery({
    queryKey: ['dsr-status', token],
    queryFn: () => fetchDSRStatus(token!),
    enabled: !!token,
    retry: false,
  })

  if (isLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-50">
        <p className="text-gray-500">{t('dsr.status.loading')}</p>
      </div>
    )
  }

  if (isError || !dsr) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-50 p-4">
        <div className="max-w-md w-full bg-white rounded-xl shadow p-8 text-center">
          <div className="text-4xl mb-4">⚠️</div>
          <h1 className="text-xl font-semibold text-gray-800 mb-3">
            {t('dsr.status.notFoundTitle')}
          </h1>
          <p className="text-gray-600 text-sm">
            {t('dsr.status.notFoundHint')}
          </p>
        </div>
      </div>
    )
  }

  const statusInfo = statusLabels[dsr.status] ?? {
    label: dsr.status,
    color: 'bg-gray-100 text-gray-800',
  }

  return (
    <div className="min-h-screen bg-gray-50 flex flex-col">
      {/* Header */}
      <header className="bg-white border-b px-6 py-4 shadow-sm">
        <div className="max-w-2xl mx-auto">
          <h1 className="text-lg font-semibold text-gray-800">
            {t('dsr.status.headerTitle')}
          </h1>
          <p className="text-sm text-gray-500 mt-0.5">
            {t('dsr.status.headerSubtitle')}
          </p>
        </div>
      </header>

      <main className="flex-1 flex items-start justify-center p-4 sm:p-8">
        <div className="w-full max-w-2xl">
          <div className="bg-white rounded-xl shadow p-6 space-y-5">
            {/* Status badge */}
            <div className="flex items-center justify-between">
              <h2 className="text-base font-semibold text-gray-800">{t('dsr.status.statusTitle')}</h2>
              <span
                className={`inline-flex px-3 py-1 rounded-full text-xs font-medium ${statusInfo.color}`}
              >
                {statusInfo.label}
              </span>
            </div>

            <div className="border-t pt-4 space-y-3">
              <Row label={t('dsr.status.rowType')} value={typeLabels[dsr.type] ?? dsr.type} />
              <Row label={t('dsr.status.rowReceived')} value={formatDate(dsr.received_at)} />
              {dsr.due_date && (
                <Row label={t('dsr.status.rowDue')} value={dsr.due_date} />
              )}
              {dsr.completed_at && (
                <Row label={t('dsr.status.rowCompleted')} value={formatDate(dsr.completed_at)} />
              )}
            </div>

            {/* Status explanation */}
            <div className="bg-blue-50 rounded-lg p-4 text-sm text-blue-800">
              {dsr.status === 'open' && (
                <p>{t('dsr.status.explanationOpen')}</p>
              )}
              {dsr.status === 'in_progress' && (
                <p>{t('dsr.status.explanationInProgress')}</p>
              )}
              {dsr.status === 'completed' && (
                <p>{t('dsr.status.explanationCompleted')}</p>
              )}
              {dsr.status === 'rejected' && (
                <p>{t('dsr.status.explanationRejected')}</p>
              )}
            </div>
          </div>
        </div>
      </main>

      <footer className="py-4 text-center text-xs text-gray-500">
        {t('dsr.status.footer')}
      </footer>
    </div>
  )
}
