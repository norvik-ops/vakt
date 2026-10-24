import { useState, useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { PageHeader } from '../shared/components/PageHeader'
import { Spinner } from '../components/Spinner'
import { Button } from '../components/ui/button'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'
import { useScoreConfig, useUpdateScoreConfig, ScoreConfig } from '../hooks/useDashboard'

// ─── Toast (minimal inline) ───────────────────────────────────────────────────

function useToast() {
  const [message, setMessage] = useState<string | null>(null)
  const timerRef = useRef<ReturnType<typeof setTimeout>>()
  useEffect(() => () => { clearTimeout(timerRef.current); }, [])
  function show(msg: string) {
    setMessage(msg)
    timerRef.current = setTimeout(() => { setMessage(null); }, 3000)
  }
  return { message, show }
}

// ─── Field row ────────────────────────────────────────────────────────────────

interface FieldRowProps {
  label: string
  field: keyof ScoreConfig
  value: number
  error: string | null
  onChange: (field: keyof ScoreConfig, value: number) => void
}

function FieldRow({ label, field, value, error, onChange }: FieldRowProps) {
  return (
    <div className="space-y-1">
      <Label className="text-xs text-secondary">{label}</Label>
      <Input
        type="number"
        min={1}
        max={100}
        value={value}
        onChange={(e) => { onChange(field, Number(e.target.value)); }}
        className={`h-8 text-sm ${error ? 'border-red-500 focus-visible:ring-red-500' : ''}`}
      />
      {error && <p className="text-[11px] text-red-500">{error}</p>}
    </div>
  )
}

// ─── Page ─────────────────────────────────────────────────────────────────────

const DEFAULT_CONFIG: ScoreConfig = {
  base_score: 70,
  crit_penalty: 5,
  crit_penalty_cap: 30,
  high_penalty: 2,
  high_penalty_cap: 10,
  breach_penalty: 20,
  breach_penalty_cap: 20,
  fw_bonus: 10,
  fw_bonus_cap: 30,
}

// Only these weights still reach the score. Findings (Vakt Scan) and data
// breaches (Vakt Privacy) are archived, their counts are always 0, so the
// crit/high/breach penalties have no effect. They stay in the API and are sent
// back unchanged on save, so an older configuration survives a later restore.
const VISIBLE_FIELDS: (keyof ScoreConfig)[] = ['base_score', 'fw_bonus', 'fw_bonus_cap']

type Errors = Partial<Record<keyof ScoreConfig, string>>

function validate(form: ScoreConfig, fieldError: string): Errors {
  const errors: Errors = {}
  for (const key of VISIBLE_FIELDS) {
    if (form[key] < 1 || form[key] > 100) {
      errors[key] = fieldError
    }
  }
  return errors
}

export default function ScoreConfigPage() {
  const { t } = useTranslation()
  const { data, isLoading } = useScoreConfig()
  const update = useUpdateScoreConfig()
  const toast = useToast()

  const [form, setForm] = useState<ScoreConfig>(DEFAULT_CONFIG)
  const [errors, setErrors] = useState<Errors>({})

  useEffect(() => {
    if (data) setForm(data)
  }, [data])

  function handleChange(field: keyof ScoreConfig, value: number) {
    const next = { ...form, [field]: value }
    setForm(next)
    const errs = validate(next, t('scoreConfig.fieldError'))
    setErrors(errs)
  }

  function handleSave() {
    const errs = validate(form, t('scoreConfig.fieldError'))
    setErrors(errs)
    if (Object.keys(errs).length > 0) return
    update.mutate(form, {
      onSuccess: () => { toast.show(t('scoreConfig.saved')); },
    })
  }

  const hasErrors = Object.keys(errors).length > 0

  const FIELDS = VISIBLE_FIELDS.map((field) => ({ label: t(`scoreConfig.fields.${field}`), field }))

  function handleReset() {
    const next = { ...form }
    for (const key of VISIBLE_FIELDS) next[key] = DEFAULT_CONFIG[key]
    setForm(next)
    setErrors({})
  }

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title={t('scoreConfig.title')}
        description={t('scoreConfig.description')}
      />

      {toast.message && (
        <div className="mx-6 mt-4 px-4 py-2 bg-green-50 border border-green-200 rounded-lg text-sm text-green-800">
          {toast.message}
        </div>
      )}

      <div className="flex-1 p-6 overflow-auto">
        <div className="max-w-2xl space-y-6">

          {/* Formula */}
          <div className="bg-surface border border-border rounded-xl p-5">
            <h2 className="text-sm font-semibold text-primary mb-2">{t('scoreConfig.formulaTitle')}</h2>
            <p className="text-xs text-secondary font-mono leading-relaxed">
              Score = Basis + FWBonus(max Cap)
            </p>
            <p className="text-[11px] text-secondary mt-2">{t('scoreConfig.formulaHint')}</p>
          </div>

          {/* Fields */}
          <div className="bg-surface border border-border rounded-xl p-5">
            <h2 className="text-sm font-semibold text-primary mb-4">{t('scoreConfig.weightsTitle')}</h2>
            {isLoading ? (
              <div className="flex items-center justify-center h-16">
                <Spinner size="sm" />
              </div>
            ) : (
              <div className="grid grid-cols-2 gap-4">
                {FIELDS.map(({ label, field }) => (
                  <FieldRow
                    key={field}
                    label={label}
                    field={field}
                    value={form[field]}
                    error={errors[field] ?? null}
                    onChange={handleChange}
                  />
                ))}
              </div>
            )}
          </div>

          <div className="flex justify-end gap-2">
            <Button
              variant="outline"
              onClick={handleReset}
              disabled={update.isPending || isLoading}
              className="h-8 text-sm"
            >
              {t('scoreConfig.reset')}
            </Button>
            <Button
              onClick={handleSave}
              disabled={hasErrors || update.isPending || isLoading}
              className="h-8 text-sm"
            >
              {update.isPending ? t('scoreConfig.saving') : t('scoreConfig.save')}
            </Button>
          </div>
        </div>
      </div>
    </div>
  )
}
