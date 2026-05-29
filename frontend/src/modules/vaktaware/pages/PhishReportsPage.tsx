import { useState, useEffect } from 'react'
import { Flag, ShieldAlert, ShieldCheck, Copy, RefreshCw, Info } from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
import { PageHeader } from '../../../shared/components/PageHeader'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '../../../components/ui/table'
import { EmptyState } from '../../../shared/components/EmptyState'
import { usePhishReports, usePhishReportStats, useRegeneratePhishToken } from '../hooks/usePhishReports'
import { useFormatDate } from '../../../shared/hooks/useFormatDate'

function StatCard({ label, value, icon: Icon, accent }: { label: string; value: number; icon: React.ElementType; accent?: string }) {
  return (
    <div className="rounded-lg border border-border bg-surface p-5 flex items-center gap-4">
      <div className={`p-2.5 rounded-lg ${accent ?? 'bg-brand/10'}`}>
        <Icon className={`w-5 h-5 ${accent ? 'text-current' : 'text-brand'}`} />
      </div>
      <div>
        <p className="text-2xl font-bold tabular-nums">{value}</p>
        <p className="text-xs text-secondary mt-0.5">{label}</p>
      </div>
    </div>
  )
}

export default function PhishReportsPage() {
  const { data: reports, isLoading } = usePhishReports()
  const { data: stats } = usePhishReportStats()
  const { formatDateTime } = useFormatDate()
  const regenerate = useRegeneratePhishToken()
  const [activeToken, setActiveToken] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)

  useEffect(() => {
    if (!copied) return
    const id = setTimeout(() => { setCopied(false); }, 2000)
    return () => { clearTimeout(id); }
  }, [copied])


  function handleRegenerate() {
    regenerate.mutate(undefined, {
      onSuccess: (res) => { setActiveToken(res.token); },
    })
  }

  function handleCopy(token: string) {
    void navigator.clipboard.writeText(token)
    setCopied(true)
  }

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title="Phish-Berichte"
        description="Gemeldete Phishing-E-Mails aus dem Outlook- und Gmail-Add-in."
      />

      <div className="flex-1 p-6 space-y-6 overflow-auto">
        {/* Stats */}
        <div className="grid grid-cols-3 gap-4">
          <StatCard label="Gesamt gemeldet" value={stats?.total ?? 0} icon={Flag} />
          <StatCard
            label="Simulation erkannt"
            value={stats?.simulations ?? 0}
            icon={ShieldCheck}
            accent="bg-green-100 dark:bg-green-900/30 text-green-600"
          />
          <StatCard
            label="Echte Bedrohungen"
            value={stats?.real_threats ?? 0}
            icon={ShieldAlert}
            accent="bg-red-100 dark:bg-red-900/30 text-red-600"
          />
        </div>

        {/* Table */}
        <div>
          <h2 className="text-sm font-semibold mb-3">Gemeldete Mails</h2>
          {isLoading ? (
            <div className="flex justify-center py-16">
              <Spinner size="md" />
            </div>
          ) : !reports || reports.length === 0 ? (
            <EmptyState
              icon={Flag}
              title="Noch keine Meldungen"
              description="Sobald Mitarbeiter verdächtige E-Mails über das Add-in melden, erscheinen sie hier."
            />
          ) : (
            <div className="rounded-md border border-border bg-surface overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Melder</TableHead>
                    <TableHead>Betreff</TableHead>
                    <TableHead>Absender</TableHead>
                    <TableHead>Typ</TableHead>
                    <TableHead>Gemeldet am</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {reports.map((r) => (
                    <TableRow key={r.id}>
                      <TableCell className="font-medium text-sm">{r.reporter_email}</TableCell>
                      <TableCell className="text-sm text-secondary max-w-[200px] truncate">
                        {r.subject ?? '—'}
                      </TableCell>
                      <TableCell className="text-sm text-secondary max-w-[160px] truncate">
                        {r.sender ?? '—'}
                      </TableCell>
                      <TableCell>
                        {r.is_simulation ? (
                          <Badge variant="success" className="text-xs">Simulation</Badge>
                        ) : (
                          <Badge variant="destructive" className="text-xs">Echte Bedrohung</Badge>
                        )}
                      </TableCell>
                      <TableCell className="text-sm text-secondary">
                        {formatDateTime(r.reported_at)}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </div>

        {/* Add-in Configuration */}
        <div className="rounded-lg border border-border bg-surface p-5 space-y-4">
          <div className="flex items-start gap-3">
            <Info className="w-4 h-4 mt-0.5 text-brand shrink-0" />
            <div>
              <h2 className="text-sm font-semibold">Phish-Button konfigurieren</h2>
              <p className="text-xs text-secondary mt-1">
                Installiere das Vakt-Add-in in Outlook oder Gmail und trage deinen Org-Token ein.
                Wenn ein Mitarbeiter auf "Als Phishing melden" klickt, sendet das Add-in einen Webhook
                an <code className="bg-bg rounded px-1 text-[11px]">/api/v1/vaktaware/phish-report</code> mit diesem Token.
              </p>
            </div>
          </div>

          <div className="space-y-2">
            <p className="text-xs font-medium text-secondary uppercase tracking-wide">Dein Org-Token</p>
            {activeToken ? (
              <div className="flex items-center gap-2">
                <code className="flex-1 bg-bg rounded-md border border-border px-3 py-2 text-xs font-mono break-all select-all">
                  {activeToken}
                </code>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => { handleCopy(activeToken); }}
                >
                  <Copy className="w-3.5 h-3.5 mr-1" />
                  {copied ? 'Kopiert!' : 'Kopieren'}
                </Button>
              </div>
            ) : (
              <p className="text-xs text-secondary italic">
                Klicke auf "Token generieren" um einen neuen Token zu erstellen.
              </p>
            )}
          </div>

          <Button
            variant="outline"
            size="sm"
            onClick={handleRegenerate}
            disabled={regenerate.isPending}
          >
            <RefreshCw className={`w-3.5 h-3.5 mr-1.5 ${regenerate.isPending ? 'animate-spin' : ''}`} />
            {activeToken ? 'Token neu generieren' : 'Token generieren'}
          </Button>

          {activeToken && (
            <p className="text-xs text-amber-600">
              Hinweis: Durch Neu-Generieren wird der alte Token ungültig. Aktualisiere das Add-in entsprechend.
            </p>
          )}

          <div className="border-t border-border pt-4 space-y-2">
            <p className="text-xs font-medium">Webhook-Endpunkt</p>
            <code className="block bg-bg rounded-md border border-border px-3 py-2 text-xs font-mono">
              POST /api/v1/vaktaware/phish-report
            </code>
            <p className="text-xs font-medium mt-2">Payload-Beispiel</p>
            <pre className="bg-bg rounded-md border border-border px-3 py-2 text-xs font-mono overflow-x-auto">{`{
  "org_token": "<dein-token>",
  "reporter_email": "max.mustermann@firma.de",
  "subject": "Urgent: Ihr Konto wurde gesperrt",
  "sender": "noreply@suspicious-domain.com"
}`}</pre>
          </div>
        </div>
      </div>
    </div>
  )
}
