import { useParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
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
  const res = await fetch(`/api/v1/secprivacy/dsr-portal/status/${token}`, {
    headers: { Accept: 'application/json' },
  })
  if (res.status === 404) throw new Error('NOT_FOUND')
  if (!res.ok) throw new Error('FETCH_FAILED')
  return res.json() as Promise<DSR>
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const TYPE_LABELS: Record<string, string> = {
  access: 'Auskunft (Art. 15)',
  erasure: 'Löschung (Art. 17)',
  rectification: 'Berichtigung (Art. 16)',
  objection: 'Widerspruch (Art. 21)',
  portability: 'Datenübertragbarkeit (Art. 20)',
}

const STATUS_LABELS: Record<string, { label: string; color: string }> = {
  open: { label: 'Offen', color: 'bg-yellow-100 text-yellow-800' },
  in_progress: { label: 'In Bearbeitung', color: 'bg-blue-100 text-blue-800' },
  completed: { label: 'Abgeschlossen', color: 'bg-green-100 text-green-800' },
  rejected: { label: 'Abgelehnt', color: 'bg-red-100 text-red-800' },
}

// ---------------------------------------------------------------------------
// DSRPortalStatusPage
// ---------------------------------------------------------------------------

export default function DSRPortalStatusPage() {
  const { token } = useParams<{ token: string }>()
  const { formatDate } = useFormatDate()

  const { data: dsr, isLoading, isError } = useQuery({
    queryKey: ['dsr-status', token],
    queryFn: () => fetchDSRStatus(token!),
    enabled: !!token,
    retry: false,
  })

  if (isLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-50">
        <p className="text-gray-500">Status wird geladen…</p>
      </div>
    )
  }

  if (isError || !dsr) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-50 p-4">
        <div className="max-w-md w-full bg-white rounded-xl shadow p-8 text-center">
          <div className="text-4xl mb-4">⚠️</div>
          <h1 className="text-xl font-semibold text-gray-800 mb-3">
            Anfrage nicht gefunden
          </h1>
          <p className="text-gray-600 text-sm">
            Zu diesem Token wurde keine Datenschutzanfrage gefunden. Bitte prüfen Sie
            den Token und versuchen Sie es erneut.
          </p>
        </div>
      </div>
    )
  }

  const statusInfo = STATUS_LABELS[dsr.status] ?? {
    label: dsr.status,
    color: 'bg-gray-100 text-gray-800',
  }

  return (
    <div className="min-h-screen bg-gray-50 flex flex-col">
      {/* Header */}
      <header className="bg-white border-b px-6 py-4 shadow-sm">
        <div className="max-w-2xl mx-auto">
          <h1 className="text-lg font-semibold text-gray-800">
            Status Ihrer Datenschutzanfrage
          </h1>
          <p className="text-sm text-gray-500 mt-0.5">
            Betroffenenanfrage nach Art. 15–21 DSGVO
          </p>
        </div>
      </header>

      <main className="flex-1 flex items-start justify-center p-4 sm:p-8">
        <div className="w-full max-w-2xl">
          <div className="bg-white rounded-xl shadow p-6 space-y-5">
            {/* Status badge */}
            <div className="flex items-center justify-between">
              <h2 className="text-base font-semibold text-gray-800">Anfragestatus</h2>
              <span
                className={`inline-flex px-3 py-1 rounded-full text-xs font-medium ${statusInfo.color}`}
              >
                {statusInfo.label}
              </span>
            </div>

            <div className="border-t pt-4 space-y-3">
              <Row label="Anfragetyp" value={TYPE_LABELS[dsr.type] ?? dsr.type} />
              <Row label="Eingegangen am" value={formatDate(dsr.received_at)} />
              {dsr.due_date && (
                <Row label="Antwort-Frist" value={dsr.due_date} />
              )}
              {dsr.completed_at && (
                <Row label="Abgeschlossen am" value={formatDate(dsr.completed_at)} />
              )}
            </div>

            {/* Status explanation */}
            <div className="bg-blue-50 rounded-lg p-4 text-sm text-blue-800">
              {dsr.status === 'open' && (
                <p>
                  Ihre Anfrage ist eingegangen und wird bearbeitet. Gemäß Art. 12 Abs. 3
                  DSGVO haben Sie Anspruch auf eine Antwort innerhalb von 30 Tagen.
                </p>
              )}
              {dsr.status === 'in_progress' && (
                <p>
                  Ihre Anfrage wird derzeit von der zuständigen Stelle bearbeitet. Sie
                  werden innerhalb der gesetzlichen Frist eine Antwort erhalten.
                </p>
              )}
              {dsr.status === 'completed' && (
                <p>
                  Ihre Anfrage wurde abgeschlossen. Sie sollten bereits eine Antwort per
                  E-Mail erhalten haben.
                </p>
              )}
              {dsr.status === 'rejected' && (
                <p>
                  Ihre Anfrage wurde abgelehnt. Die Gründe dafür sollten Ihnen per
                  E-Mail mitgeteilt worden sein. Bei Fragen wenden Sie sich bitte an den
                  Datenschutzbeauftragten.
                </p>
              )}
            </div>
          </div>
        </div>
      </main>

      <footer className="py-4 text-center text-xs text-gray-500">
        Datenschutz-Self-Service · Powered by Vakt
      </footer>
    </div>
  )
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex justify-between text-sm">
      <span className="text-gray-500">{label}</span>
      <span className="text-gray-800 font-medium">{value}</span>
    </div>
  )
}
