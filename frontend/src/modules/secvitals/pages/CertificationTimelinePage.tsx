import { useState } from 'react'
import {
  CalendarDays, Plus, ChevronLeft, ChevronRight, CheckCircle2, Trash2,
} from 'lucide-react'
import { PageHeader } from '../../../shared/components/PageHeader'
import { EmptyState } from '../../../shared/components/EmptyState'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '../../../components/ui/dialog'
import { Input } from '../../../components/ui/input'
import { Label } from '../../../components/ui/label'
import { Textarea } from '../../../components/ui/textarea'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../../../components/ui/select'
import {
  useMilestones,
  useCreateMilestone,
  useUpdateMilestone,
  useDeleteMilestone,
} from '../hooks/useMilestones'
import { useFrameworks } from '../hooks/useFrameworks'
import type { AuditMilestone, MilestoneStatus, MilestoneType, CreateMilestoneInput } from '../types'
import { formatLocale } from '../../../shared/utils/locale'

// ---- constants ----

const TYPE_LABEL: Record<MilestoneType, string> = {
  internal_audit:        'Internes Audit',
  external_audit:        'Externes Audit',
  certification_target:  'Zertifizierungsziel',
  review_deadline:       'Review-Frist',
  training_deadline:     'Schulungsfrist',
  custom:                'Benutzerdefiniert',
}

const STATUS_LABEL: Record<MilestoneStatus, string> = {
  upcoming:   'Bevorstehend',
  completed:  'Abgeschlossen',
  missed:     'Verpasst',
  cancelled:  'Abgebrochen',
}

const STATUS_CLASS: Record<MilestoneStatus, string> = {
  upcoming:  'bg-blue-500/20 text-blue-400 border-blue-500/30',
  completed: 'bg-green-500/20 text-green-400 border-green-500/30',
  missed:    'bg-red-500/20 text-red-400 border-red-500/30',
  cancelled: 'bg-slate-500/20 text-slate-400 border-slate-500/30',
}

function countdownColor(days: number | null | undefined): string {
  if (days == null) return 'text-secondary'
  if (days < 0)  return 'text-red-400'
  if (days < 30) return 'text-red-400'
  if (days < 90) return 'text-amber-400'
  return 'text-green-400'
}

function formatDate(dateStr: string): string {
  const [y, m, d] = dateStr.split('-').map(Number)
  const months = [
    'Januar', 'Februar', 'März', 'April', 'Mai', 'Juni',
    'Juli', 'August', 'September', 'Oktober', 'November', 'Dezember',
  ]
  return `${d}. ${months[m - 1]} ${y}`
}

type FilterTab = 'all' | MilestoneStatus

const TABS: { key: FilterTab; label: string }[] = [
  { key: 'all',       label: 'Alle' },
  { key: 'upcoming',  label: 'Bevorstehend' },
  { key: 'completed', label: 'Abgeschlossen' },
]

// ---- Countdown Card ----

function CountdownCard({
  m,
  onComplete,
}: {
  m: AuditMilestone
  onComplete: () => void
}) {
  const days = m.days_remaining
  const color = countdownColor(days)
  return (
    <div className="rounded-lg border border-border bg-surface p-4 flex flex-col gap-2">
      <div className="flex items-start justify-between gap-2">
        <span className={`text-[36px] font-black leading-none ${color}`}>
          {days == null ? '—' : days < 0 ? `${Math.abs(days)}d` : `${days}d`}
        </span>
        <Badge variant="outline" className="text-[10px] shrink-0 mt-1">
          {TYPE_LABEL[m.milestone_type]}
        </Badge>
      </div>
      <p className="text-[12px] text-secondary leading-tight">
        {days == null ? '' : days < 0 ? 'Überfällig seit' : 'In'}
        {' '}
        <span className={`font-semibold ${color}`}>
          {days == null ? '' : `${Math.abs(days)} Tagen`}
        </span>
      </p>
      <p className="text-[13px] font-semibold text-primary truncate">{m.title}</p>
      <p className="text-[11px] text-secondary">{formatDate(m.milestone_date)}</p>
      {m.status === 'upcoming' && (
        <Button size="sm" variant="outline" className="mt-auto text-[11px]" onClick={onComplete}>
          <CheckCircle2 className="w-3.5 h-3.5 mr-1" />
          Als erledigt markieren
        </Button>
      )}
    </div>
  )
}

// ---- Mini Calendar ----

function MiniCalendar({ milestones }: { milestones: AuditMilestone[] }) {
  const today = new Date()
  const [year, setYear] = useState(today.getFullYear())
  const [month, setMonth] = useState(today.getMonth()) // 0-indexed

  const monthNames = [
    'Januar', 'Februar', 'März', 'April', 'Mai', 'Juni',
    'Juli', 'August', 'September', 'Oktober', 'November', 'Dezember',
  ]

  const firstDay = new Date(year, month, 1).getDay() // 0=Sun
  // Adjust for Monday-first grid (ISO week)
  const startOffset = (firstDay + 6) % 7
  const daysInMonth = new Date(year, month + 1, 0).getDate()

  // Build a set of dates with milestones in this month — key: "YYYY-MM-DD"
  const milestonesByDate = new Map<string, AuditMilestone[]>()
  for (const m of milestones) {
    const [my, mm] = m.milestone_date.split('-').map(Number)
    if (my === year && mm - 1 === month) {
      const key = m.milestone_date
      milestonesByDate.set(key, [...(milestonesByDate.get(key) ?? []), m])
    }
  }

  function prevMonth() {
    if (month === 0) { setMonth(11); setYear(y => y - 1) }
    else setMonth(m => m - 1)
  }
  function nextMonth() {
    if (month === 11) { setMonth(0); setYear(y => y + 1) }
    else setMonth(m => m + 1)
  }

  const todayKey = `${today.getFullYear()}-${String(today.getMonth() + 1).padStart(2, '0')}-${String(today.getDate()).padStart(2, '0')}`

  return (
    <div className="rounded-lg border border-border bg-surface p-4">
      <div className="flex items-center justify-between mb-4">
        <h3 className="text-[13px] font-semibold text-primary">
          {monthNames[month]} {year}
        </h3>
        <div className="flex gap-1">
          <Button size="sm" variant="ghost" className="h-7 w-7 p-0" onClick={prevMonth}>
            <ChevronLeft className="w-4 h-4" />
          </Button>
          <Button size="sm" variant="ghost" className="h-7 w-7 p-0" onClick={nextMonth}>
            <ChevronRight className="w-4 h-4" />
          </Button>
        </div>
      </div>

      {/* Day headers — Mo Di Mi Do Fr Sa So */}
      <div className="grid grid-cols-7 mb-1">
        {['Mo', 'Di', 'Mi', 'Do', 'Fr', 'Sa', 'So'].map(d => (
          <span key={d} className="text-center text-[10px] font-semibold text-secondary py-1">{d}</span>
        ))}
      </div>

      {/* Calendar grid */}
      <div className="grid grid-cols-7 gap-y-1">
        {/* Empty cells before first day */}
        {Array.from({ length: startOffset }).map((_, i) => (
          <div key={`empty-${i}`} />
        ))}

        {Array.from({ length: daysInMonth }).map((_, i) => {
          const day = i + 1
          const dateKey = `${year}-${String(month + 1).padStart(2, '0')}-${String(day).padStart(2, '0')}`
          const dots = milestonesByDate.get(dateKey) ?? []
          const isToday = dateKey === todayKey

          return (
            <div key={day} className="flex flex-col items-center py-0.5 group relative">
              <span
                className={`text-[11px] w-6 h-6 flex items-center justify-center rounded-full font-medium
                  ${isToday ? 'bg-brand text-white' : 'text-primary hover:bg-border/60'}`}
              >
                {day}
              </span>
              {dots.length > 0 && (
                <div className="flex gap-0.5 mt-0.5">
                  {dots.slice(0, 3).map((m, di) => (
                    <span
                      key={di}
                      title={m.title}
                      className={`w-1.5 h-1.5 rounded-full ${
                        m.status === 'completed' ? 'bg-green-400' :
                        m.status === 'missed'    ? 'bg-red-400' :
                        m.status === 'cancelled' ? 'bg-slate-400' :
                        'bg-blue-400'
                      }`}
                    />
                  ))}
                </div>
              )}
            </div>
          )
        })}
      </div>

      {/* Legend */}
      <div className="flex flex-wrap gap-3 mt-4 pt-3 border-t border-border">
        {(['upcoming', 'completed', 'missed', 'cancelled'] as MilestoneStatus[]).map(s => (
          <div key={s} className="flex items-center gap-1.5">
            <span className={`w-2 h-2 rounded-full ${
              s === 'completed' ? 'bg-green-400' :
              s === 'missed'    ? 'bg-red-400' :
              s === 'cancelled' ? 'bg-slate-400' :
              'bg-blue-400'
            }`} />
            <span className="text-[10px] text-secondary">{STATUS_LABEL[s]}</span>
          </div>
        ))}
      </div>
    </div>
  )
}

// ---- Gantt Chart ----

const GANTT_STATUS_COLORS: Record<MilestoneStatus, string> = {
  completed: '#22c55e',
  upcoming:  '#3b82f6',
  missed:    '#ef4444',
  cancelled: '#94a3b8',
}

function GanttChart({ milestones }: { milestones: AuditMilestone[] }) {
  if (milestones.length === 0) return null

  const now = new Date()
  const sorted = [...milestones].sort(
    (a, b) => a.milestone_date.localeCompare(b.milestone_date),
  )

  // Date range: 30 days in the past → 90 days in the future
  const rangeStart = new Date(now.getTime() - 30 * 24 * 60 * 60 * 1000)
  const rangeEnd   = new Date(now.getTime() + 90 * 24 * 60 * 60 * 1000)
  const totalMs    = rangeEnd.getTime() - rangeStart.getTime()

  function xPct(date: Date): number {
    return Math.max(0, Math.min(100, ((date.getTime() - rangeStart.getTime()) / totalMs) * 100))
  }

  const todayPct = xPct(now)
  const rowH = 36
  const headerH = 20
  const svgH = sorted.length * rowH + headerH + 8

  return (
    <div className="rounded-lg border border-border bg-surface p-4 overflow-x-auto">
      <h3 className="text-[13px] font-semibold text-primary mb-3">Audit-Zeitplan (Gantt)</h3>
      <svg width="100%" height={svgH} className="min-w-[600px]">
        {/* Grid lines + month labels */}
        {Array.from({ length: 5 }, (_, i) => {
          const fraction = i / 4
          const d = new Date(rangeStart.getTime() + fraction * totalMs)
          const x = fraction * 100
          return (
            <g key={i}>
              <line
                x1={`${x}%`} y1="0"
                x2={`${x}%`} y2={svgH}
                stroke="#e5e7eb" strokeWidth="1"
              />
              <text
                x={`${x}%`} y={headerH - 4}
                fontSize="10" fill="#9ca3af" textAnchor="middle"
              >
                {d.toLocaleDateString(formatLocale(), { month: 'short', year: '2-digit' })}
              </text>
            </g>
          )
        })}

        {/* Today marker */}
        <line
          x1={`${todayPct}%`} y1="0"
          x2={`${todayPct}%`} y2={svgH}
          stroke="#4f46e5" strokeWidth="2" strokeDasharray="4 2"
        />
        <text
          x={`${todayPct}%`} y={headerH - 4}
          fontSize="9" fill="#4f46e5" textAnchor="middle"
        >
          Heute
        </text>

        {/* Milestone bars */}
        {sorted.map((m, i) => {
          const dueDate  = new Date(m.milestone_date)
          // Show each milestone as a 14-day bar ending on milestone_date
          const barStart = new Date(dueDate.getTime() - 14 * 24 * 60 * 60 * 1000)
          const x1 = xPct(barStart)
          const x2 = xPct(dueDate)
          const y  = headerH + 4 + i * rowH
          const color = GANTT_STATUS_COLORS[m.status]

          return (
            <g key={m.id}>
              <rect
                x={`${x1}%`}
                y={y}
                width={`${Math.max(x2 - x1, 0.5)}%`}
                height={rowH - 8}
                rx="4"
                fill={color}
                fillOpacity="0.85"
              />
              <text
                x={`${x1}%`}
                y={y + (rowH - 8) / 2 + 4}
                dx="6"
                fontSize="11"
                fill="white"
                fontWeight="500"
              >
                {m.title.length > 28 ? m.title.substring(0, 27) + '…' : m.title}
              </text>
            </g>
          )
        })}
      </svg>

      {/* Legend */}
      <div className="flex flex-wrap gap-4 mt-3 text-[11px] text-secondary">
        {([
          ['#22c55e', 'Abgeschlossen'],
          ['#3b82f6', 'Bevorstehend'],
          ['#ef4444', 'Verpasst'],
          ['#94a3b8', 'Abgebrochen'],
        ] as const).map(([c, l]) => (
          <div key={l} className="flex items-center gap-1">
            <div className="w-3 h-2 rounded" style={{ backgroundColor: c }} />
            <span>{l}</span>
          </div>
        ))}
      </div>
    </div>
  )
}

// ---- Add Milestone Dialog ----

const EMPTY_FORM: CreateMilestoneInput = {
  title: '',
  description: '',
  milestone_date: '',
  milestone_type: 'internal_audit',
  framework_id: '',
}

function AddMilestoneDialog({
  open,
  onClose,
}: {
  open: boolean
  onClose: () => void
}) {
  const [form, setForm] = useState<CreateMilestoneInput>(EMPTY_FORM)
  const { data: frameworks } = useFrameworks()
  const create = useCreateMilestone()

  function handleSubmit() {
    const payload: CreateMilestoneInput = {
      ...form,
      framework_id: form.framework_id || undefined,
    }
    create.mutate(payload, {
      onSuccess: () => {
        setForm(EMPTY_FORM)
        onClose()
      },
    })
  }

  return (
    <Dialog open={open} onOpenChange={(v) => { if (!v) onClose() }}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>Meilenstein hinzufügen</DialogTitle>
        </DialogHeader>

        <div className="space-y-4 py-2">
          <div className="space-y-1">
            <Label htmlFor="ms-title">Titel *</Label>
            <Input
              id="ms-title"
              value={form.title}
              onChange={e => { setForm(f => ({ ...f, title: e.target.value })); }}
              placeholder="z. B. ISO 27001 Zertifizierungsaudit"
            />
          </div>

          <div className="space-y-1">
            <Label htmlFor="ms-type">Typ *</Label>
            <Select
              value={form.milestone_type}
              onValueChange={v => { setForm(f => ({ ...f, milestone_type: v as MilestoneType })); }}
            >
              <SelectTrigger id="ms-type">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {(Object.keys(TYPE_LABEL) as MilestoneType[]).map(t => (
                  <SelectItem key={t} value={t}>{TYPE_LABEL[t]}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-1">
            <Label htmlFor="ms-date">Datum *</Label>
            <Input
              id="ms-date"
              type="date"
              value={form.milestone_date}
              onChange={e => { setForm(f => ({ ...f, milestone_date: e.target.value })); }}
            />
          </div>

          <div className="space-y-1">
            <Label htmlFor="ms-desc">Beschreibung</Label>
            <Textarea
              id="ms-desc"
              rows={3}
              value={form.description}
              onChange={e => { setForm(f => ({ ...f, description: e.target.value })); }}
              placeholder="Optionale Beschreibung oder Notizen"
            />
          </div>

          <div className="space-y-1">
            <Label htmlFor="ms-fw">Framework (optional)</Label>
            <Select
              value={form.framework_id ?? ''}
              onValueChange={v => { setForm(f => ({ ...f, framework_id: v === '__none__' ? '' : v })); }}
            >
              <SelectTrigger id="ms-fw">
                <SelectValue placeholder="Kein Framework" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="__none__">Kein Framework</SelectItem>
                {frameworks?.map(fw => (
                  <SelectItem key={fw.id} value={fw.id}>{fw.name}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={onClose} disabled={create.isPending}>
            Abbrechen
          </Button>
          <Button
            onClick={handleSubmit}
            disabled={!form.title || !form.milestone_date || create.isPending}
          >
            {create.isPending ? 'Speichern…' : 'Hinzufügen'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// ---- Table row with inline complete/delete ----

function MilestoneRow({
  m,
  onComplete,
  onDelete,
}: {
  m: AuditMilestone
  onComplete: () => void
  onDelete: () => void
}) {
  const days = m.days_remaining
  const daysStr =
    days == null ? '—' :
    days === 0   ? 'Heute' :
    days < 0     ? `${Math.abs(days)} Tage überfällig` :
    `${days} Tage`
  const daysColor = countdownColor(days)

  return (
    <tr className="border-b border-border last:border-0 hover:bg-border/20 transition-colors">
      <td className="py-2 px-3 text-[12px] text-secondary whitespace-nowrap">{formatDate(m.milestone_date)}</td>
      <td className="py-2 px-3 text-[12px] text-primary font-medium max-w-[200px] truncate">{m.title}</td>
      <td className="py-2 px-3">
        <Badge variant="outline" className="text-[10px]">{TYPE_LABEL[m.milestone_type]}</Badge>
      </td>
      <td className="py-2 px-3">
        <Badge variant="outline" className={`text-[10px] ${STATUS_CLASS[m.status]}`}>
          {STATUS_LABEL[m.status]}
        </Badge>
      </td>
      <td className={`py-2 px-3 text-[12px] font-semibold ${daysColor} whitespace-nowrap`}>{daysStr}</td>
      <td className="py-2 px-3">
        <div className="flex items-center gap-1">
          {m.status === 'upcoming' && (
            <Button size="sm" variant="ghost" className="h-7 px-2 text-[11px]" onClick={onComplete}>
              <CheckCircle2 className="w-3.5 h-3.5 mr-1" />
              Erledigt
            </Button>
          )}
          <Button
            size="sm"
            variant="ghost"
            className="h-7 w-7 p-0 text-destructive hover:text-destructive"
            onClick={onDelete}
          >
            <Trash2 className="w-3.5 h-3.5" />
          </Button>
        </div>
      </td>
    </tr>
  )
}

// ---- Main Page ----

export default function CertificationTimelinePage() {
  const [tab, setTab] = useState<FilterTab>('all')
  const [addOpen, setAddOpen] = useState(false)

  const { data: all, isLoading } = useMilestones()
  const deleteMilestone = useDeleteMilestone()

  // We do client-side filtering to avoid extra query keys
  const milestones = all ?? []
  const filtered =
    tab === 'all' ? milestones : milestones.filter(m => m.status === tab)

  // Top 3 upcoming countdown cards
  const upcoming = milestones
    .filter(m => m.status === 'upcoming')
    .sort((a, b) => a.milestone_date.localeCompare(b.milestone_date))
    .slice(0, 3)

  return (
    <div className="flex flex-col gap-6 p-6 max-w-6xl mx-auto">
      <PageHeader
        title="Zertifizierungs-Timeline"
        description="Audits, Zertifizierungsziele und Fristen im Überblick"
        actions={
          <Button onClick={() => { setAddOpen(true); }} size="sm">
            <Plus className="w-4 h-4 mr-1.5" />
            Meilenstein hinzufügen
          </Button>
        }
      />

      {/* Countdown Cards — Top 3 upcoming */}
      {upcoming.length > 0 && (
        <section>
          <h2 className="text-[12px] font-semibold text-secondary uppercase tracking-wider mb-3">
            Nächste Meilensteine
          </h2>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
            {upcoming.map(m => (
              <CountdownCardWrapper key={m.id} m={m} />
            ))}
          </div>
        </section>
      )}

      {/* Gantt Chart */}
      {milestones.length > 0 && (
        <section>
          <h2 className="text-[12px] font-semibold text-secondary uppercase tracking-wider mb-3">
            Zeitplan-Übersicht
          </h2>
          <GanttChart milestones={milestones} />
        </section>
      )}

      {/* Middle: Table */}
      <section className="rounded-lg border border-border bg-surface">
        <div className="flex items-center justify-between px-4 pt-4 pb-2 border-b border-border">
          <h2 className="text-[13px] font-semibold text-primary">Alle Meilensteine</h2>
          <div className="flex gap-1">
            {TABS.map(t => (
              <button
                key={t.key}
                onClick={() => { setTab(t.key); }}
                className={`px-3 py-1 rounded text-[11px] font-medium transition-colors ${
                  tab === t.key
                    ? 'bg-brand text-white'
                    : 'text-secondary hover:text-primary hover:bg-border/60'
                }`}
              >
                {t.label}
              </button>
            ))}
          </div>
        </div>

        {isLoading ? (
          <div className="p-8 text-center text-secondary text-[13px]">Laden…</div>
        ) : filtered.length === 0 ? (
          <EmptyState
            icon={CalendarDays}
            title="Keine Meilensteine"
            description={
              tab === 'all'
                ? 'Fügen Sie Ihren ersten Meilenstein hinzu.'
                : `Keine ${STATUS_LABEL[tab]} Meilensteine vorhanden.`
            }
          />
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead>
                <tr className="text-left border-b border-border">
                  <th className="py-2 px-3 text-[11px] font-semibold text-secondary uppercase tracking-wider">Datum</th>
                  <th className="py-2 px-3 text-[11px] font-semibold text-secondary uppercase tracking-wider">Titel</th>
                  <th className="py-2 px-3 text-[11px] font-semibold text-secondary uppercase tracking-wider">Typ</th>
                  <th className="py-2 px-3 text-[11px] font-semibold text-secondary uppercase tracking-wider">Status</th>
                  <th className="py-2 px-3 text-[11px] font-semibold text-secondary uppercase tracking-wider">Tage verbleibend</th>
                  <th className="py-2 px-3 text-[11px] font-semibold text-secondary uppercase tracking-wider">Aktionen</th>
                </tr>
              </thead>
              <tbody>
                {filtered.map(m => (
                  <MilestoneRowWrapper
                    key={m.id}
                    m={m}
                    onDelete={() => { deleteMilestone.mutate(m.id); }}
                  />
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      {/* Bottom: Calendar */}
      <section>
        <h2 className="text-[12px] font-semibold text-secondary uppercase tracking-wider mb-3">
          Kalenderansicht
        </h2>
        <MiniCalendar milestones={milestones} />
      </section>

      <AddMilestoneDialog open={addOpen} onClose={() => { setAddOpen(false); }} />
    </div>
  )
}

// ---- Wrapper components that own the mutation hooks ----

function CountdownCardWrapper({ m }: { m: AuditMilestone }) {
  const update = useUpdateMilestone(m.id)
  return (
    <CountdownCard
      m={m}
      onComplete={() => { update.mutate({ status: 'completed' }); }}
    />
  )
}

function MilestoneRowWrapper({
  m,
  onDelete,
}: {
  m: AuditMilestone
  onDelete: () => void
}) {
  const update = useUpdateMilestone(m.id)
  return (
    <MilestoneRow
      m={m}
      onComplete={() => { update.mutate({ status: 'completed' }); }}
      onDelete={onDelete}
    />
  )
}
