import { useState } from 'react'
import { GitBranch, Plus, ChevronDown, ChevronUp, KeyRound } from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
import { PageHeader } from '../../../shared/components/PageHeader'
import { InfoBanner } from '../../../shared/components/InfoBanner'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { EmptyState } from '../../../shared/components/EmptyState'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '../../../components/ui/dialog'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { useGitScans, useTriggerGitScan, useGitScanResults, useDismissScanResult } from '../hooks/useGitScans'
import type { GitScan } from '../types'
import { jobStatusVariant } from '../../../lib/statusMapping'
import { useFormatDate } from '../../../shared/hooks/useFormatDate'

const statusVariant = jobStatusVariant

function ScanResultsPanel({ scanId }: { scanId: string }) {
  const { data: results, isLoading } = useGitScanResults(scanId, true)
  const dismiss = useDismissScanResult()
  const [dismissingId, setDismissingId] = useState<string | null>(null)
  const [dismissReason, setDismissReason] = useState('')

  function handleDismiss() {
    if (!dismissingId) return
    dismiss.mutate({ resultId: dismissingId, reason: dismissReason }, {
      onSuccess: () => {
        setDismissingId(null)
        setDismissReason('')
      },
    })
  }

  if (isLoading) {
    return (
      <div className="flex justify-center py-4">
        <Spinner size="sm" />
      </div>
    )
  }

  if (!results || results.length === 0) {
    return <p className="text-sm text-secondary py-4 text-center">No findings.</p>
  }

  const active = results.filter((r) => !r.dismissed)
  const dismissed = results.filter((r) => r.dismissed)

  return (
    <div className="mt-3 space-y-2">
      {active.map((result) => (
        <div key={result.id} className="p-3 bg-red-500/10 border border-red-500/30 rounded-md text-sm">
          <div className="flex items-start justify-between gap-4">
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2 mb-1">
                <Badge variant="destructive" className="text-xs">{result.secret_type}</Badge>
                <span className="font-mono text-xs text-secondary truncate">{result.file_path}:{result.line_number}</span>
              </div>
              <code className="text-xs text-primary font-mono block truncate">{result.snippet}</code>
            </div>
            <Button
              size="sm"
              variant="outline"
              className="shrink-0"
              onClick={() => { setDismissingId(result.id); setDismissReason('') }}
            >
              Dismiss
            </Button>
          </div>
        </div>
      ))}

      {dismissed.length > 0 && (
        <p className="text-xs text-secondary">{dismissed.length} dismissed finding{dismissed.length !== 1 ? 's' : ''}</p>
      )}

      <Dialog open={!!dismissingId} onOpenChange={(open) => { if (!open) { setDismissingId(null); } }}>
        <DialogContent>
          <DialogHeader><DialogTitle>Dismiss Finding</DialogTitle></DialogHeader>
          <div className="py-4 space-y-1.5">
            <Label htmlFor="dismiss-reason">Reason</Label>
            <Input
              id="dismiss-reason"
              placeholder="False positive, already rotated, etc."
              value={dismissReason}
              onChange={(e) => { setDismissReason(e.target.value); }}
            />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setDismissingId(null); }}>Abbrechen</Button>
            <Button onClick={handleDismiss} disabled={dismiss.isPending}>
              {dismiss.isPending ? 'Dismissing…' : 'Dismiss'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}

function ScanRow({ scan }: { scan: GitScan }) {
  const [expanded, setExpanded] = useState(false)
  const { formatDate } = useFormatDate()

  return (
    <div className="border border-border rounded-lg bg-surface overflow-hidden">
      <div
        className="flex items-center gap-4 p-4 cursor-pointer hover:bg-surface2"
        onClick={() => { setExpanded(!expanded); }}
      >
        <GitBranch className="w-4 h-4 text-secondary shrink-0" />
        <span className="font-mono text-sm text-primary flex-1 truncate">{scan.repo_url}</span>
        <Badge variant={statusVariant[scan.status]} className="capitalize">{scan.status}</Badge>
        {scan.result_count != null && scan.result_count > 0 && (
          <Badge variant="destructive">{scan.result_count} finding{scan.result_count !== 1 ? 's' : ''}</Badge>
        )}
        <span className="text-xs text-secondary">{formatDate(scan.created_at)}</span>
        {expanded ? <ChevronUp className="w-4 h-4 text-secondary" /> : <ChevronDown className="w-4 h-4 text-secondary" />}
      </div>
      {expanded && scan.status === 'completed' && (
        <div className="border-t border-border px-4 pb-4">
          <ScanResultsPanel scanId={scan.id} />
        </div>
      )}
    </div>
  )
}

export default function GitScansPage() {
  const { data: scans, isLoading } = useGitScans()
  const triggerScan = useTriggerGitScan()
  const [open, setOpen] = useState(false)
  const [repoUrl, setRepoUrl] = useState('')

  function handleTrigger(e: React.FormEvent) {
    e.preventDefault()
    triggerScan.mutate({ repo_url: repoUrl }, {
      onSuccess: () => {
        setOpen(false)
        setRepoUrl('')
      },
    })
  }

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title="Git Scans"
        description="Repositories nach geleakten Zugangsdaten und Secrets durchsuchen."
        actions={
          <Button onClick={() => { setOpen(true); }}>
            <Plus className="w-4 h-4 mr-1" />
            New Scan
          </Button>
        }
      />

      <InfoBanner icon={KeyRound} title="Git-Repos auf Secrets scannen">
        <p>Gib die URL eines öffentlichen oder privaten Repositories ein (HTTPS oder SSH). Vakt Vault sucht mit <strong>Gitleaks</strong> nach versehentlich eingecheckten Passwörtern, Tokens und API-Keys.</p>
        <p className="mt-1">Für <strong>private Repositories</strong>: trage zuerst ein Personal Access Token (GitHub/GitLab/Bitbucket) unter <strong>Settings → Integrationen</strong> ein.</p>
      </InfoBanner>

      <div className="flex-1 p-6 space-y-3">
        {isLoading ? (
          <div className="flex justify-center py-16">
            <Spinner size="md" />
          </div>
        ) : !scans || scans.length === 0 ? (
          <EmptyState
            icon={GitBranch}
            title="Keine Git-Scans"
            description="Verbinde dein erstes Repository."
            action={
              <Button onClick={() => { setOpen(true); }}>
                <Plus className="w-4 h-4 mr-1" />Scan starten
              </Button>
            }
          />
        ) : (
          scans.map((scan) => <ScanRow key={scan.id} scan={scan} />)
        )}
      </div>

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader><DialogTitle>Scan Repository</DialogTitle></DialogHeader>
          <form onSubmit={(e) => { handleTrigger(e) }}>
            <div className="py-4 space-y-1.5">
              <Label htmlFor="repo-url">Repository URL</Label>
              <Input
                id="repo-url"
                placeholder="https://github.com/org/repo"
                value={repoUrl}
                onChange={(e) => { setRepoUrl(e.target.value); }}
                required
              />
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => { setOpen(false); }}>Abbrechen</Button>
              <Button type="submit" disabled={triggerScan.isPending}>
                {triggerScan.isPending ? 'Starting…' : 'Start Scan'}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </div>
  )
}
