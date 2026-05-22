import { useState } from 'react'
import { useParams } from 'react-router-dom'
import { Spinner } from '../components/Spinner'
import { useQuery, useMutation } from '@tanstack/react-query'
import { ShieldCheck, FileText, AlertCircle, CheckCircle2 } from 'lucide-react'
import { fetchAcceptanceInfo, submitAcceptance } from '../modules/secvitals/hooks/usePolicyAcceptance'
import { formatLocale } from '../shared/utils/locale'

// ---------------------------------------------------------------------------
// Page
// ---------------------------------------------------------------------------

export default function PolicyAcceptPage() {
  const { token = '' } = useParams<{ token: string }>()
  const [accepted, setAccepted] = useState(false)
  const [acceptedAt, setAcceptedAt] = useState<string | null>(null)

  const { data: info, isLoading, error } = useQuery({
    queryKey: ['policy-accept', token],
    queryFn: () => fetchAcceptanceInfo(token),
    enabled: !!token,
    retry: false,
  })

  const acceptMutation = useMutation({
    mutationFn: () => submitAcceptance(token),
    onSuccess: () => {
      setAccepted(true)
      setAcceptedAt(new Date().toLocaleString(formatLocale(), {
        day: '2-digit', month: '2-digit', year: 'numeric',
        hour: '2-digit', minute: '2-digit',
      }))
    },
  })

  // Already accepted before this session
  const alreadyAccepted = info?.accepted_at

  return (
    <div className="min-h-screen bg-background flex items-center justify-center px-4 py-12">
      <div className="w-full max-w-lg">
        {/* Header */}
        <div className="flex items-center gap-2 mb-8">
          <ShieldCheck className="text-brand" size={28} />
          <span className="text-xl font-semibold">Vakt Compliance</span>
        </div>

        {isLoading && (
          <div className="flex justify-center py-20">
            <Spinner size="lg" />
          </div>
        )}

        {error && (
          <div className="rounded-lg border border-destructive/50 bg-destructive/10 p-6 flex gap-3">
            <AlertCircle className="text-destructive mt-0.5 shrink-0" size={20} />
            <div>
              <p className="font-medium text-sm">Link nicht gefunden</p>
              <p className="text-sm text-muted-foreground mt-1">
                Dieser Bestätigungslink ist ungültig oder wurde bereits verwendet. Bitte wenden Sie sich an Ihr Compliance-Team.
              </p>
            </div>
          </div>
        )}

        {info && !error && (
          <div className="space-y-6">
            {/* Org + policy info */}
            <div className="rounded-lg border border-border bg-card p-6 space-y-3">
              <p className="text-xs text-muted-foreground font-medium uppercase tracking-wide">
                {info.org_name}
              </p>
              <div className="flex items-start gap-3">
                <FileText className="text-muted-foreground mt-0.5 shrink-0" size={20} />
                <div>
                  <p className="font-semibold text-base">{info.policy_title}</p>
                  {info.message && (
                    <p className="text-sm text-muted-foreground mt-2 whitespace-pre-wrap">{info.message}</p>
                  )}
                </div>
              </div>
              {info.deadline && (
                <p className="text-xs text-amber-500 border border-amber-500/30 bg-amber-500/10 rounded px-2 py-1 inline-block">
                  Deadline: {info.deadline}
                </p>
              )}
            </div>

            {/* Success state (accepted in this session) */}
            {accepted && (
              <div className="rounded-lg border border-green-500/40 bg-green-500/10 p-6 flex gap-3">
                <CheckCircle2 className="text-green-500 mt-0.5 shrink-0" size={22} />
                <div>
                  <p className="font-semibold text-sm text-green-500">Richtlinie bestätigt</p>
                  <p className="text-sm text-muted-foreground mt-1">
                    Vielen Dank. Ihre Bestätigung wurde am {acceptedAt} Uhr erfasst und als Compliance-Nachweis gespeichert.
                  </p>
                </div>
              </div>
            )}

            {/* Already accepted (from server) */}
            {alreadyAccepted && !accepted && (
              <div className="rounded-lg border border-green-500/40 bg-green-500/10 p-6 flex gap-3">
                <CheckCircle2 className="text-green-500 mt-0.5 shrink-0" size={22} />
                <div>
                  <p className="font-semibold text-sm text-green-500">Bereits bestätigt</p>
                  <p className="text-sm text-muted-foreground mt-1">
                    Sie haben diese Richtlinie bereits bestätigt.
                  </p>
                </div>
              </div>
            )}

            {/* Action button */}
            {!accepted && !alreadyAccepted && (
              <div className="space-y-3">
                <p className="text-sm text-muted-foreground">
                  Mit Klick auf den Button bestätigen Sie, dass Sie die Richtlinie gelesen haben und mit deren Inhalt einverstanden sind.
                </p>
                <button
                  onClick={() => acceptMutation.mutate()}
                  disabled={acceptMutation.isPending}
                  className="w-full rounded-lg bg-brand text-brand-foreground px-6 py-3 font-medium text-sm transition-opacity hover:opacity-90 disabled:opacity-50"
                  style={{ backgroundColor: 'hsl(var(--brand))', color: 'hsl(var(--brand-foreground, 0 0% 100%))' }}
                >
                  {acceptMutation.isPending
                    ? 'Wird gespeichert...'
                    : 'Ich habe die Richtlinie gelesen und akzeptiere sie'}
                </button>
                {acceptMutation.isError && (
                  <p className="text-sm text-destructive text-center">
                    Fehler: {acceptMutation.error?.message ?? 'Unbekannter Fehler'}
                  </p>
                )}
              </div>
            )}

            <p className="text-xs text-muted-foreground text-center">
              Diese E-Mail wurde automatisch von Vakt generiert. Dieser Link ist persönlich.
            </p>
          </div>
        )}
      </div>
    </div>
  )
}
