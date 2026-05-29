import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { CalendarClock, AlertCircle, CheckCircle2 } from 'lucide-react'
import { PageHeader } from '../../../shared/components/PageHeader'
import { Card, CardContent, CardHeader, CardTitle } from '../../../components/ui/card'
import { Badge } from '../../../components/ui/badge'
import { Button } from '../../../components/ui/button'
import {
  Dialog, DialogContent, DialogHeader, DialogTitle,
} from '../../../components/ui/dialog'
import { useOverdueControls } from '../hooks/useControlReviews'
import { ControlReviewPanel } from '../components/ControlReviewPanel'
import type { Control } from '../types'
import { useFormatDate } from '../../../shared/hooks/useFormatDate'

function daysOverdue(nextReviewDue?: string): number {
  if (!nextReviewDue) return 0
  const due = new Date(nextReviewDue)
  const now = new Date()
  return Math.floor((now.getTime() - due.getTime()) / (1000 * 60 * 60 * 24))
}

function groupByDomain(controls: Control[]): Record<string, Control[]> {
  return controls.reduce<Record<string, Control[]>>((acc, c) => {
    const key = c.domain || 'Ohne Bereich'
    if (!acc[key]) acc[key] = []
    acc[key].push(c)
    return acc
  }, {})
}

export default function OverdueReviewsPage() {
  const { data: controls, isLoading } = useOverdueControls()
  const navigate = useNavigate()
  const { formatDate: fmtDate } = useFormatDate()
  function formatDate(iso?: string): string {
    if (!iso) return 'Noch nie überprüft'
    return fmtDate(iso)
  }
  const [selectedControl, setSelectedControl] = useState<Control | null>(null)

  const grouped = groupByDomain(controls ?? [])

  return (
    <div className="p-6 max-w-4xl mx-auto space-y-6">
      <PageHeader
        title="Überfällige Kontrollen"
        description="ISO 27001 erfordert eine regelmäßige Überprüfung aller Kontrollen. Die folgenden Kontrollen haben ihr nächstes Überprüfungsdatum überschritten."
      />

      {isLoading && (
        <p className="text-sm text-muted-foreground">Lade Daten...</p>
      )}

      {!isLoading && (!controls || controls.length === 0) && (
        <Card>
          <CardContent className="py-12 text-center space-y-2">
            <CheckCircle2 className="w-10 h-10 mx-auto text-green-500" />
            <p className="font-medium">Keine überfälligen Kontrollen</p>
            <p className="text-sm text-muted-foreground">
              Alle Kontrollen mit konfiguriertem Überprüfungszyklus sind aktuell.
            </p>
          </CardContent>
        </Card>
      )}

      {Object.entries(grouped).map(([domain, domainControls]) => (
        <Card key={domain}>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm text-muted-foreground uppercase tracking-wide">
              {domain}
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-2">
            {domainControls.map((ctrl) => {
              const days = daysOverdue(ctrl.next_review_due)
              return (
                <div
                  key={ctrl.id}
                  className="flex items-center justify-between px-3 py-2.5 rounded-md border border-border hover:border-border/80 transition-colors"
                >
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      <AlertCircle className="w-3.5 h-3.5 text-destructive shrink-0" />
                      <button
                        className="text-sm font-medium hover:underline truncate text-left"
                        onClick={() => { navigate(`/vaktcomply/controls/${ctrl.id}`); }}
                      >
                        {ctrl.title}
                      </button>
                      <Badge variant="secondary" className="text-[10px] shrink-0">
                        {ctrl.control_id}
                      </Badge>
                    </div>
                    <div className="mt-1 flex items-center gap-3 text-xs text-muted-foreground ml-5">
                      <span>Zuletzt: {formatDate(ctrl.last_reviewed_at)}</span>
                      {days > 0 && (
                        <span className="text-destructive font-medium">{days} Tage überfällig</span>
                      )}
                    </div>
                  </div>
                  <Button
                    size="sm"
                    variant="outline"
                    className="ml-4 h-7 text-xs shrink-0"
                    onClick={() => { setSelectedControl(ctrl); }}
                  >
                    <CalendarClock className="w-3 h-3 mr-1" />
                    Jetzt überprüfen
                  </Button>
                </div>
              )
            })}
          </CardContent>
        </Card>
      ))}

      {selectedControl && (
        <Dialog open={!!selectedControl} onOpenChange={(open) => { if (!open) { setSelectedControl(null); } }}>
          <DialogContent className="max-w-md">
            <DialogHeader>
              <DialogTitle className="text-sm font-semibold">
                {selectedControl.title}
              </DialogTitle>
            </DialogHeader>
            <ControlReviewPanel
              controlId={selectedControl.id}
              lastReviewedAt={selectedControl.last_reviewed_at}
              nextReviewDue={selectedControl.next_review_due}
              isOverdue={selectedControl.is_review_overdue}
              reviewIntervalDays={selectedControl.review_interval_days}
              lastReviewedBy={selectedControl.last_reviewed_by}
            />
          </DialogContent>
        </Dialog>
      )}
    </div>
  )
}
