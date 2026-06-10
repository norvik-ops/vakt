import { useState } from 'react'
import { ShieldCheck, Plus, Trash2, ScanLine, AlertTriangle, Check, X } from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
import { PageHeader } from '../../../shared/components/PageHeader'
import { EmptyState } from '../../../shared/components/EmptyState'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '../../../components/ui/dialog'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '../../../components/ui/table'
import { ConfirmDeleteDialog } from '../../../shared/components/ConfirmDeleteDialog'
import { useCertificates, useCreateCertificate, useDeleteCertificate, useScanCertificate } from '../hooks/useCertificates'
import type { Certificate } from '../types'
import { cn } from '../../../lib/utils'
import { useFormatDate } from '../../../shared/hooks/useFormatDate'

const statusClass: Record<Certificate['status'], string> = {
  valid:    'border-transparent bg-green-500/20 text-green-400',
  expiring: 'border-transparent bg-amber-500/20 text-amber-400',
  expired:  'border-transparent bg-red-500/20 text-red-400',
  error:    'border-transparent bg-red-500/10 text-red-300',
  unknown:  'border-transparent bg-surface2 text-muted',
}

const statusIcon: Record<Certificate['status'], React.ReactNode> = {
  valid:    <Check className="w-3 h-3" />,
  expiring: <AlertTriangle className="w-3 h-3" />,
  expired:  <X className="w-3 h-3" />,
  error:    <X className="w-3 h-3" />,
  unknown:  null,
}

function daysUntil(dateStr?: string | null): number | null {
  if (!dateStr) return null
  const diff = new Date(dateStr).getTime() - Date.now()
  return Math.floor(diff / (1000 * 60 * 60 * 24))
}

function ExpiryCell({ notAfter }: { notAfter?: string | null }) {
  const { formatDate } = useFormatDate()
  const days = daysUntil(notAfter)
  if (!notAfter) return <span className="text-secondary">—</span>
  const label = formatDate(notAfter)
  if (days === null) return <span className="text-secondary">{label}</span>
  if (days < 0) return <span className="text-red-400">{label} (abgelaufen)</span>
  if (days <= 30) return <span className="text-amber-400">{label} ({days}d)</span>
  return <span className="text-primary">{label}</span>
}

export default function CertificatesPage() {
  const [addOpen, setAddOpen] = useState(false)
  const [domain, setDomain] = useState('')
  const [deleteId, setDeleteId] = useState<string | null>(null)
  const [scanningId, setScanningId] = useState<string | null>(null)

  const { data, isLoading, isError } = useCertificates()
  const createCert = useCreateCertificate()
  const deleteCert = useDeleteCertificate()
  const scanCert = useScanCertificate()

  const certs = data?.data ?? []

  const deleteTarget = deleteId ? certs.find((c) => c.id === deleteId) : null

  function handleAdd() {
    if (!domain.trim()) return
    createCert.mutate({ domain: domain.trim() }, {
      onSuccess: () => {
        setDomain('')
        setAddOpen(false)
      },
    })
  }

  async function handleScan(id: string) {
    setScanningId(id)
    try {
      await scanCert.mutateAsync(id)
    } finally {
      setScanningId(null)
    }
  }

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title="TLS-Zertifikate"
        description="Überwachung von TLS-Zertifikaten — Ablaufdaten, Aussteller und SANs im Blick."
        actions={
          <Button onClick={() => { setAddOpen(true); }}>
            <Plus className="w-4 h-4 mr-1" />
            Zertifikat hinzufügen
          </Button>
        }
      />

      <div className="flex-1 p-6">
        {isLoading && (
          <div className="flex justify-center py-16">
            <Spinner size="md" />
          </div>
        )}
        {isError && (
          <p className="text-sm text-red-400 p-4 bg-red-500/10 rounded-lg">
            Zertifikate konnten nicht geladen werden.
          </p>
        )}
        {!isLoading && !isError && certs.length === 0 && (
          <EmptyState
            icon={ShieldCheck}
            title="Keine Zertifikate"
            description="Füge Domains hinzu, um deren TLS-Zertifikate zu überwachen."
            action={
              <Button onClick={() => { setAddOpen(true); }}>
                <Plus className="w-4 h-4 mr-1" />
                Zertifikat hinzufügen
              </Button>
            }
          />
        )}
        {!isLoading && !isError && certs.length > 0 && (
          <div className="rounded-md border border-border bg-surface overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Domain</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Ablauf</TableHead>
                  <TableHead>Aussteller</TableHead>
                  <TableHead>Zuletzt geprüft</TableHead>
                  <TableHead className="w-24" />
                </TableRow>
              </TableHeader>
              <TableBody>
                {certs.map((cert) => (
                  <TableRow key={cert.id}>
                    <TableCell className="font-mono text-xs">{cert.domain}</TableCell>
                    <TableCell>
                      <Badge className={cn('inline-flex items-center gap-1', statusClass[cert.status])} variant="outline">
                        {statusIcon[cert.status]}
                        {cert.status}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <ExpiryCell notAfter={cert.not_after} />
                    </TableCell>
                    <TableCell className="text-secondary text-xs max-w-[180px] truncate">
                      {cert.issuer || '—'}
                    </TableCell>
                    <TableCell className="text-secondary text-xs">
                      {cert.last_checked_at
                        ? new Date(cert.last_checked_at).toLocaleDateString('de-DE')
                        : '—'}
                    </TableCell>
                    <TableCell>
                      <div className="flex items-center gap-1 justify-end">
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-7 w-7"
                          title="Jetzt scannen"
                          disabled={scanningId === cert.id}
                          onClick={() => { void handleScan(cert.id) }}
                        >
                          {scanningId === cert.id
                            ? <Spinner size="sm" />
                            : <ScanLine className="w-3.5 h-3.5" />
                          }
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-7 w-7 text-red-400 hover:text-red-300"
                          title="Löschen"
                          onClick={() => { setDeleteId(cert.id); }}
                        >
                          <Trash2 className="w-3.5 h-3.5" />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        )}
      </div>

      {/* Add Certificate Dialog */}
      <Dialog open={addOpen} onOpenChange={setAddOpen}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>Zertifikat hinzufügen</DialogTitle>
          </DialogHeader>
          <div className="space-y-3 py-2">
            <div className="space-y-1.5">
              <Label>Domain *</Label>
              <Input
                placeholder="example.com"
                value={domain}
                onChange={(e) => { setDomain(e.target.value); }}
                onKeyDown={(e) => { if (e.key === 'Enter') { handleAdd(); } }}
              />
              <p className="text-xs text-muted-foreground">
                Domain ohne https:// — Port optional, z.B. example.com oder smtp.example.com:465
              </p>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setAddOpen(false); }}>Abbrechen</Button>
            <Button
              onClick={handleAdd}
              disabled={!domain.trim() || createCert.isPending}
            >
              {createCert.isPending ? 'Wird gespeichert…' : 'Hinzufügen'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete Confirmation */}
      {deleteTarget && (
        <ConfirmDeleteDialog
          open={deleteId != null}
          onOpenChange={(o) => { if (!o) setDeleteId(null); }}
          resourceName={deleteTarget.domain}
          resourceType="Zertifikat"
          onConfirm={() => {
            if (deleteId) {
              deleteCert.mutate(deleteId, { onSuccess: () => { setDeleteId(null); } })
            }
          }}
          isPending={deleteCert.isPending}
        />
      )}
    </div>
  )
}
