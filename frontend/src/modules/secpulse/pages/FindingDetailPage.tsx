import { useState, useEffect } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft } from 'lucide-react'
import { PageHeader } from '../../../shared/components/PageHeader'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '../../../components/ui/card'
import { Label } from '../../../components/ui/label'
import { Select, SelectTrigger, SelectContent, SelectItem, SelectValue } from '../../../components/ui/select'
import { useFinding, usePatchFinding } from '../hooks/useFindings'
import type { Finding } from '../types'
import { cn } from '../../../lib/utils'

const severityClass: Record<Finding['severity'], string> = {
  info:     'bg-[#374151] text-[#94a3b8] border-transparent',
  low:      'bg-[#1e3a5f] text-[#93c5fd] border-transparent',
  medium:   'bg-[#78350f] text-[#f59e0b] border-transparent',
  high:     'bg-[#7c2d12] text-[#f97316] border-transparent',
  critical: 'bg-[#7f1d1d] text-[#ef4444] border-transparent',
}

export default function FindingDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { data: finding, isLoading, error } = useFinding(id ?? '')
  const patch = usePatchFinding(id ?? '')

  const [status, setStatus] = useState<Finding['status'] | ''>('')
  const [notes, setNotes] = useState('')
  const [saved, setSaved] = useState(false)

  useEffect(() => {
    if (!saved) return
    const id = setTimeout(() => setSaved(false), 2000)
    return () => clearTimeout(id)
  }, [saved])

  function currentStatus(): Finding['status'] | '' {
    return status || finding?.status || ''
  }

  async function handleSave() {
    if (!id) return
    await patch.mutateAsync({
      ...(status ? { status: status as Finding['status'] } : {}),
      notes: notes || undefined,
    })
    setSaved(true)
  }

  if (isLoading) return (
    <div className="flex justify-center py-16">
      <div className="w-5 h-5 border-2 border-brand border-t-transparent rounded-full animate-spin" />
    </div>
  )

  if (error || !finding) return (
    <div className="p-6">
      <p className="text-sm text-red-600">{error?.message ?? 'Finding not found'}</p>
      <Button variant="outline" className="mt-4" onClick={() => navigate('/secpulse/findings')}>
        <ArrowLeft className="w-4 h-4 mr-1" />Back
      </Button>
    </div>
  )

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title={finding.title}
        actions={
          <Button variant="outline" onClick={() => navigate('/secpulse/findings')}>
            <ArrowLeft className="w-4 h-4 mr-1" />Back
          </Button>
        }
      />

      <div className="p-6 grid grid-cols-3 gap-6">
        <div className="col-span-2 space-y-6">
          <Card>
            <CardHeader><CardTitle>Beschreibung</CardTitle></CardHeader>
            <CardContent>
              <p className="text-sm text-secondary whitespace-pre-wrap">{finding.description}</p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader><CardTitle>Status aktualisieren</CardTitle></CardHeader>
            <CardContent className="space-y-4">
              <div className="space-y-1">
                <Label>Status</Label>
                <Select
                  value={currentStatus()}
                  onValueChange={(v) => setStatus(v as Finding['status'])}
                >
                  <SelectTrigger className="w-48">
                    <SelectValue placeholder="Status wählen" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="open">Offen</SelectItem>
                    <SelectItem value="in_progress">In Bearbeitung</SelectItem>
                    <SelectItem value="accepted_risk">Akzeptiertes Risiko</SelectItem>
                    <SelectItem value="false_positive">Falsch positiv</SelectItem>
                    <SelectItem value="resolved">Behoben</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-1">
                <Label htmlFor="notes">Notizen</Label>
                <textarea
                  id="notes"
                  rows={4}
                  className="w-full rounded-md border border-border px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-brand"
                  placeholder="Untersuchungsnotizen hinzufügen…"
                  defaultValue={finding.notes ?? ''}
                  onChange={(e) => setNotes(e.target.value)}
                />
              </div>

              {patch.isError && <p className="text-sm text-red-600">{patch.error.message}</p>}
              {saved && <p className="text-sm text-green-600">Saved.</p>}

              <Button onClick={() => { void handleSave() }} disabled={patch.isPending}>
                {patch.isPending ? 'Saving…' : 'Save Changes'}
              </Button>
            </CardContent>
          </Card>
        </div>

        <div className="space-y-4">
          <Card>
            <CardHeader><CardTitle>Details</CardTitle></CardHeader>
            <CardContent>
              <dl className="space-y-3 text-sm">
                <div>
                  <dt className="text-secondary">Severity</dt>
                  <dd className="mt-0.5">
                    <Badge className={cn('capitalize', severityClass[finding.severity])}>{finding.severity}</Badge>
                  </dd>
                </div>
                <div>
                  <dt className="text-secondary">Status</dt>
                  <dd className="mt-0.5 capitalize text-primary">{finding.status.replace(/_/g, ' ')}</dd>
                </div>
                {finding.cve_id && (
                  <div>
                    <dt className="text-secondary">CVE</dt>
                    <dd className="mt-0.5 font-mono text-xs text-primary">{finding.cve_id}</dd>
                  </div>
                )}
                {finding.cvss_score != null && (
                  <div>
                    <dt className="text-secondary">CVSS Score</dt>
                    <dd className="mt-0.5 text-primary font-semibold">{finding.cvss_score.toFixed(1)}</dd>
                  </div>
                )}
                <div>
                  <dt className="text-secondary">Discovered</dt>
                  <dd className="mt-0.5 text-primary">{new Date(finding.created_at).toLocaleDateString()}</dd>
                </div>
              </dl>
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  )
}
