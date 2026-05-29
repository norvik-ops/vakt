import { useState } from 'react'
import { Lock, Trash2, Plus, ChevronDown, ChevronRight } from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
import { Card, CardContent, CardHeader, CardTitle } from '../../../components/ui/card'
import { Badge } from '../../../components/ui/badge'
import { Button } from '../../../components/ui/button'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../../../components/ui/select'
import { cn } from '../../../lib/utils'
import {
  useMeasures,
  useCreateMeasure,
  useDeleteMeasure,
  type ControlMeasure,
  type CreateMeasureInput,
} from '../hooks/useMeasures'

// ── Difficulty badge ──────────────────────────────────────────────────────────

const DIFFICULTY_CONFIG: Record<
  ControlMeasure['difficulty'],
  { label: string; variant: React.ComponentProps<typeof Badge>['variant'] }
> = {
  easy:   { label: 'Einfach', variant: 'success' },
  medium: { label: 'Mittel',  variant: 'warning' },
  hard:   { label: 'Komplex', variant: 'destructive' },
}

function DifficultyBadge({ difficulty }: { difficulty: ControlMeasure['difficulty'] }) {
  const cfg = DIFFICULTY_CONFIG[difficulty]
  return <Badge variant={cfg.variant}>{cfg.label}</Badge>
}

// ── Single measure row ────────────────────────────────────────────────────────

function MeasureRow({
  measure,
  controlId,
}: {
  measure: ControlMeasure
  controlId: string
}) {
  const [expanded, setExpanded] = useState(false)
  const deleteMeasure = useDeleteMeasure(controlId)

  return (
    <li className="border border-border rounded-lg overflow-hidden">
      <button
        type="button"
        className="w-full flex items-center gap-3 px-4 py-3 text-left hover:bg-surface2 transition-colors"
        onClick={() => { setExpanded((v) => !v); }}
      >
        {expanded ? (
          <ChevronDown className="w-4 h-4 text-secondary shrink-0" />
        ) : (
          <ChevronRight className="w-4 h-4 text-secondary shrink-0" />
        )}
        <span className="flex-1 text-sm font-medium">{measure.title}</span>
        <DifficultyBadge difficulty={measure.difficulty} />
        {measure.is_builtin ? (
          <Lock className="w-3.5 h-3.5 text-secondary shrink-0" aria-label="Integrierte Maßnahme" />
        ) : (
          <button
            type="button"
            onClick={(e) => {
              e.stopPropagation()
              deleteMeasure.mutate(measure.id)
            }}
            disabled={deleteMeasure.isPending}
            className="text-muted-foreground hover:text-destructive transition-colors shrink-0"
            title="Maßnahme löschen"
          >
            <Trash2 className="w-3.5 h-3.5" />
          </button>
        )}
      </button>
      {expanded && measure.description && (
        <div className="px-4 pb-4 pt-1 bg-surface2">
          <p className="text-sm text-secondary leading-relaxed whitespace-pre-wrap">
            {measure.description}
          </p>
        </div>
      )}
    </li>
  )
}

// ── Add measure form ──────────────────────────────────────────────────────────

function AddMeasureForm({
  controlId,
  onCancel,
}: {
  controlId: string
  onCancel: () => void
}) {
  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [difficulty, setDifficulty] = useState<CreateMeasureInput['difficulty']>('medium')
  const createMeasure = useCreateMeasure(controlId)

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!title.trim()) return
    createMeasure.mutate(
      { title: title.trim(), description: description.trim(), difficulty },
      {
        onSuccess: () => {
          setTitle('')
          setDescription('')
          setDifficulty('medium')
          onCancel()
        },
      },
    )
  }

  return (
    <form
      onSubmit={handleSubmit}
      className="border border-border rounded-lg p-4 space-y-3 bg-surface2"
    >
      <div className="space-y-1.5">
        <Label htmlFor="measure-title">Titel</Label>
        <Input
          id="measure-title"
          value={title}
          onChange={(e) => { setTitle(e.target.value); }}
          placeholder="z.B. Backup-Konzept erstellen"
          required
          minLength={3}
          maxLength={200}
          className="h-8 text-sm"
        />
      </div>
      <div className="space-y-1.5">
        <Label htmlFor="measure-desc">Beschreibung</Label>
        <textarea
          id="measure-desc"
          rows={3}
          className="w-full rounded-md border border-border bg-surface text-primary px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-brand resize-none"
          value={description}
          onChange={(e) => { setDescription(e.target.value); }}
          placeholder="Konkrete Schritte, Hilfsmittel, Nachweistypen …"
          maxLength={2000}
        />
      </div>
      <div className="space-y-1.5">
        <Label>Schwierigkeitsgrad</Label>
        <Select
          value={difficulty}
          onValueChange={(v) => { setDifficulty(v as CreateMeasureInput['difficulty']); }}
        >
          <SelectTrigger className="h-8 text-sm w-40">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="easy">Einfach</SelectItem>
            <SelectItem value="medium">Mittel</SelectItem>
            <SelectItem value="hard">Komplex</SelectItem>
          </SelectContent>
        </Select>
      </div>
      <div className="flex gap-2 justify-end">
        <Button type="button" variant="outline" size="sm" onClick={onCancel}>
          Abbrechen
        </Button>
        <Button type="submit" size="sm" disabled={!title.trim() || createMeasure.isPending}>
          {createMeasure.isPending ? 'Wird gespeichert…' : 'Maßnahme hinzufügen'}
        </Button>
      </div>
    </form>
  )
}

// ── Main component ────────────────────────────────────────────────────────────

export function MeasuresList({ controlId }: { controlId: string }) {
  const { data: measures, isLoading } = useMeasures(controlId)
  const [addOpen, setAddOpen] = useState(false)

  const builtinCount = measures?.filter((m) => m.is_builtin).length ?? 0
  const customCount = (measures?.length ?? 0) - builtinCount

  return (
    <Card>
      <CardHeader className="pb-3">
        <div className="flex items-center justify-between gap-2">
          <CardTitle className="text-sm">
            Empfohlene Maßnahmen
            {measures && measures.length > 0 && (
              <span className={cn('ml-2 text-xs font-normal text-secondary')}>
                {builtinCount > 0 && `${builtinCount.toString()} integriert`}
                {builtinCount > 0 && customCount > 0 && ', '}
                {customCount > 0 && `${customCount.toString()} benutzerdefiniert`}
              </span>
            )}
          </CardTitle>
          {!addOpen && (
            <Button size="sm" variant="outline" className="h-7 text-xs" onClick={() => { setAddOpen(true); }}>
              <Plus className="w-3.5 h-3.5 mr-1" />
              Maßnahme hinzufügen
            </Button>
          )}
        </div>
      </CardHeader>
      <CardContent className="space-y-3">
        {isLoading ? (
          <div className="flex justify-center py-6">
            <Spinner size="md" />
          </div>
        ) : (
          <>
            {measures && measures.length > 0 ? (
              <ul className="space-y-2">
                {measures.map((measure) => (
                  <MeasureRow key={measure.id} measure={measure} controlId={controlId} />
                ))}
              </ul>
            ) : !addOpen ? (
              <p className="text-xs text-muted-foreground">
                Noch keine Maßnahmen vorhanden. Füge eigene hinzu oder warte auf das nächste Startup-Seeding.
              </p>
            ) : null}
            {addOpen && (
              <AddMeasureForm controlId={controlId} onCancel={() => { setAddOpen(false); }} />
            )}
          </>
        )}
      </CardContent>
    </Card>
  )
}
