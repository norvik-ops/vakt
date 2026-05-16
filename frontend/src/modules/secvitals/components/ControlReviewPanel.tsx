import { useState } from 'react'
import { CalendarClock, CheckCircle2, AlertCircle, ChevronDown, ChevronUp, Save } from 'lucide-react'
import { Button } from '../../../components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '../../../components/ui/card'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { Textarea } from '../../../components/ui/textarea'
import { Badge } from '../../../components/ui/badge'
import { useControlReviews, useRecordControlReview } from '../hooks/useControlReviews'

interface ControlReviewPanelProps {
  controlId: string
  lastReviewedAt?: string
  nextReviewDue?: string
  isOverdue?: boolean
  reviewIntervalDays?: number
  lastReviewedBy?: string
}

function formatDate(iso?: string): string {
  if (!iso) return '–'
  return new Date(iso).toLocaleDateString('de-DE', {
    day: '2-digit',
    month: '2-digit',
    year: 'numeric',
  })
}

function intervalLabel(days?: number): string {
  if (!days) return 'Alle 12 Monate'
  if (days <= 30) return 'Monatlich'
  if (days <= 90) return 'Vierteljährlich'
  if (days <= 180) return 'Halbjährlich'
  if (days <= 365) return 'Jährlich'
  return `Alle ${Math.round(days / 365)} Jahre`
}

function daysOverdue(nextReviewDue?: string): number {
  if (!nextReviewDue) return 0
  const due = new Date(nextReviewDue)
  const now = new Date()
  return Math.floor((now.getTime() - due.getTime()) / (1000 * 60 * 60 * 24))
}

export function ControlReviewPanel({
  controlId,
  lastReviewedAt,
  nextReviewDue,
  isOverdue,
  reviewIntervalDays,
  lastReviewedBy,
}: ControlReviewPanelProps) {
  const [showForm, setShowForm] = useState(false)
  const [showHistory, setShowHistory] = useState(false)
  const [reviewedBy, setReviewedBy] = useState('')
  const [reviewNote, setReviewNote] = useState('')
  const [intervalDays, setIntervalDays] = useState<string>('')

  const { data: reviews } = useControlReviews(showHistory ? controlId : undefined)
  const { mutate: recordReview, isPending } = useRecordControlReview(controlId)

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!reviewedBy.trim()) return
    const payload: { reviewed_by: string; review_note?: string; review_interval_days?: number } = {
      reviewed_by: reviewedBy.trim(),
    }
    if (reviewNote.trim()) payload.review_note = reviewNote.trim()
    const parsed = parseInt(intervalDays, 10)
    if (!isNaN(parsed) && parsed >= 30) payload.review_interval_days = parsed
    recordReview(payload, {
      onSuccess: () => {
        setShowForm(false)
        setReviewedBy('')
        setReviewNote('')
        setIntervalDays('')
      },
    })
  }

  const overdueDays = daysOverdue(nextReviewDue)

  return (
    <Card>
      <CardHeader className="pb-2">
        <CardTitle className="text-sm flex items-center gap-2">
          <CalendarClock className="w-4 h-4 text-muted-foreground" />
          Überprüfungszyklus
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        {/* Status badge */}
        <div className="flex items-center justify-between">
          <div>
            {isOverdue ? (
              <Badge variant="destructive" className="flex items-center gap-1">
                <AlertCircle className="w-3 h-3" />
                Überprüfung fällig
                {overdueDays > 0 && ` (${overdueDays}d)`}
              </Badge>
            ) : nextReviewDue ? (
              <Badge variant="success" className="flex items-center gap-1">
                <CheckCircle2 className="w-3 h-3" />
                Aktuell
              </Badge>
            ) : (
              <Badge variant="secondary">Noch nicht überprüft</Badge>
            )}
          </div>
          <span className="text-xs text-muted-foreground">{intervalLabel(reviewIntervalDays)}</span>
        </div>

        {/* Review metadata */}
        <div className="grid grid-cols-2 gap-x-4 gap-y-1 text-xs text-muted-foreground">
          <span>Zuletzt überprüft</span>
          <span className="text-foreground font-medium">{formatDate(lastReviewedAt)}</span>
          {lastReviewedAt && lastReviewedBy && (
            <>
              <span>Prüfer</span>
              <span className="text-foreground font-medium">{lastReviewedBy}</span>
            </>
          )}
          <span>Nächste Überprüfung</span>
          <span className={`font-medium ${isOverdue ? 'text-destructive' : 'text-foreground'}`}>
            {formatDate(nextReviewDue)}
          </span>
        </div>

        {/* Inline review form */}
        {showForm ? (
          <form onSubmit={handleSubmit} className="space-y-3 pt-1">
            <div className="space-y-1">
              <Label htmlFor="cr-reviewer" className="text-xs">Prüfer *</Label>
              <Input
                id="cr-reviewer"
                value={reviewedBy}
                onChange={(e) => setReviewedBy(e.target.value)}
                placeholder="Name oder E-Mail"
                className="h-8 text-xs"
                required
              />
            </div>
            <div className="space-y-1">
              <Label htmlFor="cr-note" className="text-xs">Prüfnotiz</Label>
              <Textarea
                id="cr-note"
                value={reviewNote}
                onChange={(e) => setReviewNote(e.target.value)}
                placeholder="Ergebnis der Überprüfung..."
                className="text-xs resize-none h-20"
              />
            </div>
            <div className="space-y-1">
              <Label htmlFor="cr-interval" className="text-xs">
                Prüfintervall (Tage, optional)
              </Label>
              <Input
                id="cr-interval"
                type="number"
                value={intervalDays}
                onChange={(e) => setIntervalDays(e.target.value)}
                placeholder={String(reviewIntervalDays ?? 365)}
                min={30}
                max={3650}
                className="h-8 text-xs"
              />
            </div>
            <div className="flex gap-2">
              <Button type="submit" size="sm" disabled={isPending} className="h-7 text-xs">
                <Save className="w-3 h-3 mr-1" />
                {isPending ? 'Speichern...' : 'Überprüfung speichern'}
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                onClick={() => setShowForm(false)}
                className="h-7 text-xs"
              >
                Abbrechen
              </Button>
            </div>
          </form>
        ) : (
          <Button
            variant={isOverdue ? 'default' : 'outline'}
            size="sm"
            onClick={() => setShowForm(true)}
            className="h-7 text-xs w-full"
          >
            <CalendarClock className="w-3 h-3 mr-1" />
            Jetzt überprüfen
          </Button>
        )}

        {/* Review history toggle */}
        <button
          type="button"
          onClick={() => setShowHistory(!showHistory)}
          className="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors"
        >
          {showHistory ? <ChevronUp className="w-3 h-3" /> : <ChevronDown className="w-3 h-3" />}
          Überprüfungshistorie
        </button>

        {showHistory && (
          <div className="space-y-2">
            {!reviews || reviews.length === 0 ? (
              <p className="text-xs text-muted-foreground italic">Noch keine Überprüfungen erfasst.</p>
            ) : (
              reviews.slice(0, 5).map((rv) => (
                <div key={rv.id} className="border border-border rounded-md px-3 py-2 space-y-0.5">
                  <div className="flex items-center justify-between">
                    <span className="text-xs font-medium">{rv.reviewed_by}</span>
                    <span className="text-[11px] text-muted-foreground">{formatDate(rv.reviewed_at)}</span>
                  </div>
                  {rv.review_note && (
                    <p className="text-[11px] text-muted-foreground">{rv.review_note}</p>
                  )}
                  {rv.status_at_review && (
                    <span className="text-[10px] text-secondary-foreground/60">Status: {rv.status_at_review}</span>
                  )}
                </div>
              ))
            )}
          </div>
        )}
      </CardContent>
    </Card>
  )
}
