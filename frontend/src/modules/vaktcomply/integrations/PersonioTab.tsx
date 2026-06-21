import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { ShieldAlert, CheckCircle2, AlertCircle, Copy } from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
import {
  usePersonioConfig,
  useSavePersonioConfig,
  usePersonioStatus,
} from '../../../hooks/useCloud'
import { toast } from '../../../shared/hooks/useToast'
import { useFormatDate } from '../../../shared/hooks/useFormatDate'

// --- Personio tab ---

export function PersonioTab() {
  const { t } = useTranslation()
  const { data: cfg, isLoading } = usePersonioConfig()
  const { data: status } = usePersonioStatus()
  const saveConfig = useSavePersonioConfig()
  const { formatDateTime } = useFormatDate()

  const [webhookSecret, setWebhookSecret] = useState('')
  const [initialized, setInitialized] = useState(false)
  const [copied, setCopied] = useState(false)

  if (cfg && !initialized) {
    setWebhookSecret(cfg.webhook_secret)
    setInitialized(true)
  }

  async function handleSave(e: React.FormEvent) {
    e.preventDefault()
    try {
      await saveConfig.mutateAsync({ webhook_secret: webhookSecret })
      toast(t('integrations.page.saved'), 'success')
    } catch (err) {
      toast(err instanceof Error ? err.message : t('integrations.page.saveFailed'), 'error')
    }
  }

  function handleCopyURL() {
    if (!status?.webhook_url) return
    const fullURL = window.location.origin + status.webhook_url
    void navigator.clipboard.writeText(fullURL).then(() => {
      setCopied(true)
      setTimeout(() => { setCopied(false) }, 2000)
    })
  }

  if (isLoading) return <div className="flex items-center justify-center h-32"><Spinner size="md" /></div>

  const lastWebhookFormatted = status?.last_sync_at
    ? formatDateTime(status.last_sync_at, { dateStyle: 'short', timeStyle: 'short' })
    : null

  const fullWebhookURL = status?.webhook_url
    ? window.location.origin + status.webhook_url
    : null

  return (
    <div>
      <div className="mb-4">
        <h2 className="text-sm font-semibold text-primary">Personio HRIS Webhook</h2>
        <p className="text-xs text-secondary mt-0.5">
          Automatisches Offboarding-Checklisten-Trigger bei <code>employee.departed</code>-Events aus Personio.
          Vakt empfängt den Webhook und startet die HR-Offboarding-Checkliste. Kein Pull aus Personio — Push-only.
        </p>
      </div>

      {/* DSGVO notice */}
      <div className="flex items-start gap-3 p-3 mb-5 rounded-lg border border-amber-200 bg-amber-50">
        <ShieldAlert className="w-4 h-4 text-amber-600 shrink-0 mt-0.5" />
        <p className="text-xs text-amber-800">
          <strong>DSGVO-Hinweis:</strong> Vakt speichert aus dem Personio-Webhook ausschließlich die
          numerische <code>employee_id</code> und das <code>departure_date</code>. Keine Namen, E-Mail-Adressen
          oder andere personenbezogenen Daten werden persistiert (Art. 5 Abs. 1 lit. c DSGVO — Datensparsamkeit).
        </p>
      </div>

      {/* Status row */}
      {status && (
        <div className="flex items-center gap-3 mb-5 p-3 rounded-lg border border-border bg-surface">
          <div className="flex-1 min-w-0">
            <p className="text-xs text-secondary">
              {lastWebhookFormatted ? t('integrations.page.lastSync', { date: lastWebhookFormatted }) : t('integrations.page.neverSynced')}
              {status.offboardings_triggered > 0 && ` · ${status.offboardings_triggered} Offboardings ausgelöst`}
              {status.offboardings_completed_on_time > 0 && ` · ${status.offboardings_completed_on_time} fristgerecht`}
            </p>
          </div>
          {status.webhook_configured ? (
            <span className="inline-flex items-center gap-1 text-xs font-medium text-emerald-700 bg-emerald-50 border border-emerald-200 rounded-full px-2 py-0.5">
              <CheckCircle2 className="w-3 h-3" /> {t('integrations.page.saved')}
            </span>
          ) : (
            <span className="inline-flex items-center gap-1 text-xs font-medium text-amber-700 bg-amber-50 border border-amber-200 rounded-full px-2 py-0.5">
              <AlertCircle className="w-3 h-3" /> {t('integrations.page.syncPending')}
            </span>
          )}
        </div>
      )}

      {/* Webhook URL display */}
      {fullWebhookURL && (
        <div className="mb-5 max-w-lg">
          <p className="text-xs font-medium text-secondary mb-1">Webhook URL (in Personio eintragen)</p>
          <div className="flex items-center gap-2 bg-bg border border-border rounded-md px-3 py-2">
            <code className="text-xs text-primary flex-1 break-all">{fullWebhookURL}</code>
            <button onClick={handleCopyURL} title="URL kopieren"
              className="p-1 rounded text-secondary hover:text-primary transition-colors shrink-0">
              {copied ? <CheckCircle2 className="w-4 h-4 text-emerald-500" /> : <Copy className="w-4 h-4" />}
            </button>
          </div>
          <p className="text-[11px] text-secondary mt-1">
            In Personio unter Einstellungen → Integrationen → Webhooks → Add Webhook eintragen.
            Methode: <code>POST</code>, Event: <code>employee.departed</code>.
          </p>
        </div>
      )}

      <form onSubmit={(e) => { void handleSave(e) }} className="space-y-4 max-w-lg">
        <div>
          <label className="block text-xs font-medium text-secondary mb-1">Webhook Secret</label>
          <input type="password" value={webhookSecret} onChange={(e) => { setWebhookSecret(e.target.value); }}
            placeholder={cfg?.is_configured && cfg.webhook_secret === '****' ? '****' : 'Webhook Secret aus Personio'}
            className="w-full border border-border rounded-md px-3 py-2 text-sm bg-bg text-primary placeholder:text-secondary focus:outline-none focus:ring-2 focus:ring-brand/30 font-mono"
            required />
          <p className="text-[11px] text-secondary mt-1">
            Das Secret wird von Personio zum Signieren der Webhook-Payloads verwendet (HMAC-SHA256, Header: <code>X-Personio-Signature</code>).
            Secret wird AES-256-GCM verschlüsselt gespeichert.
          </p>
        </div>
        <div className="flex gap-2 pt-1">
          <button type="submit" disabled={saveConfig.isPending}
            className="px-4 py-1.5 text-xs font-medium bg-brand text-white rounded-md hover:bg-brand/90 transition-colors disabled:opacity-50">
            {saveConfig.isPending ? t('integrations.page.saving') : t('integrations.page.save')}
          </button>
        </div>
      </form>
    </div>
  )
}
