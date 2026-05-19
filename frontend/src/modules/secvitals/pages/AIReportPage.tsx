import { useState, useEffect } from 'react'
import { Bot, AlertTriangle, Copy, Download, CheckCircle } from 'lucide-react'
import { Button } from '../../../components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '../../../components/ui/card'
import { PageHeader } from '../../../shared/components/PageHeader'
import { ProGate } from '../../../shared/components/ProGate'
import { FeatureLockedError } from '../../../api/client'

const REPORT_TYPES = [
  {
    type: 'gap_analysis',
    title: 'Gap-Analyse',
    description: 'Detaillierte Analyse fehlender Controls mit Handlungsempfehlungen',
    icon: '🔍',
  },
  {
    type: 'risk_summary',
    title: 'Risiko-Übersicht',
    description: 'Zusammenfassung der wichtigsten Risiken und Sofortmaßnahmen',
    icon: '⚠️',
  },
  {
    type: 'executive_summary',
    title: 'Executive Summary',
    description: 'Management-gerechte Zusammenfassung der Sicherheitslage',
    icon: '📊',
  },
]

export default function AIReportPage() {
  const [available, setAvailable] = useState<boolean | null>(null)
  const [aiModel, setAiModel] = useState<string | null>(null)
  const [selected, setSelected] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [report, setReport] = useState<string | null>(null)
  const [error, setError] = useState<unknown>(null)
  const [copied, setCopied] = useState(false)

  useEffect(() => {
    const checkStatus = async () => {
      try {
        const res = await fetch('/api/v1/secvitals/ai/status', {
          credentials: 'include',
        })
        if (res.ok) {
          const data = await res.json()
          setAvailable(data.available === true)
          if (data.model) setAiModel(data.model)
        } else {
          // 404 = AI routes not registered (provider=disabled)
          setAvailable(false)
        }
      } catch {
        setAvailable(false)
      }
    }
    checkStatus()
  }, [])

  const generate = async () => {
    setLoading(true)
    setError(null)
    setReport(null)
    try {
      const res = await fetch('/api/v1/secvitals/ai/report', {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ type: selected }),
      })
      if (res.status === 402) throw new FeatureLockedError('ai-report')
      if (!res.ok) throw new Error('Generierung fehlgeschlagen')
      const data = await res.json()
      setReport(data.report)
    } catch (e: unknown) {
      setError(e)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    if (!copied) return
    const id = setTimeout(() => setCopied(false), 2000)
    return () => clearTimeout(id)
  }, [copied])

  const copyToClipboard = async () => {
    if (!report) return
    await navigator.clipboard.writeText(report)
    setCopied(true)
  }

  const downloadAsTxt = () => {
    if (!report) return
    const selectedType = REPORT_TYPES.find((r) => r.type === selected)
    const filename = `${selected ?? 'bericht'}_${new Date().toISOString().slice(0, 10)}.txt`
    const header = selectedType ? `${selectedType.title}\nErstellt: ${new Date().toLocaleString('de-DE')}\n\n` : ''
    const blob = new Blob([header + report], { type: 'text/plain;charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = filename
    a.click()
    URL.revokeObjectURL(url)
  }

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title="KI-gestützte Compliance-Berichte"
        description="Generiere automatisch professionelle Compliance-Berichte basierend auf Ihren aktuellen Sicherheitsdaten — via konfigurierbarem AI-Provider (Mistral, OpenAI, Ollama u.v.m.)."
      />

      <ProGate error={error}>
      <div className="p-6 space-y-6">
        {/* AI provider unavailable notice */}
        {available === false && (
          <Card className="border-amber-500/40 bg-amber-500/5">
            <CardContent className="pt-5">
              <div className="flex items-start gap-3">
                <AlertTriangle className="w-5 h-5 text-amber-400 mt-0.5 shrink-0" />
                <div className="space-y-2">
                  <p className="text-sm font-semibold text-amber-300">KI-Provider nicht konfiguriert</p>
                  <p className="text-xs text-secondary leading-relaxed">
                    KI-Berichte benötigen einen konfigurierten AI-Provider. Setzen Sie in Ihrer{' '}
                    <code className="text-primary">.env</code>-Datei:
                  </p>
                  <pre className="text-xs bg-surface rounded px-3 py-2 border border-border font-mono text-primary whitespace-pre">{`SHIELDSTACK_AI_PROVIDER=openai
SHIELDSTACK_AI_BASE_URL=https://api.mistral.ai/v1
SHIELDSTACK_AI_API_KEY=sk-...
SHIELDSTACK_AI_MODEL=mistral-small-latest`}</pre>
                  <p className="text-xs text-secondary">
                    Empfohlen: <strong>Mistral AI</strong> (EU-Server, DSGVO-freundlich, ~€0,001/Bericht).
                    Alternativ: OpenAI, Groq oder Ollama (lokal, GPU empfohlen).
                  </p>
                </div>
              </div>
            </CardContent>
          </Card>
        )}

        {/* Available — show report type selector */}
        {available === true && (
          <>
            <div>
              <p className="text-sm text-secondary mb-3">Berichtstyp auswählen:</p>
              <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
                {REPORT_TYPES.map((rt) => (
                  <button
                    key={rt.type}
                    onClick={() => {
                      setSelected(rt.type)
                      setReport(null)
                      setError(null)
                    }}
                    className={`text-left rounded-lg border p-4 transition-colors ${
                      selected === rt.type
                        ? 'border-brand bg-brand/10'
                        : 'border-border bg-surface hover:border-brand/50'
                    }`}
                  >
                    <div className="text-2xl mb-2">{rt.icon}</div>
                    <p className="text-sm font-semibold text-primary mb-1">{rt.title}</p>
                    <p className="text-xs text-secondary leading-snug">{rt.description}</p>
                  </button>
                ))}
              </div>
            </div>

            {selected && (
              <div className="flex items-center gap-3">
                <Button
                  onClick={() => void generate()}
                  disabled={loading}
                  className="gap-2"
                >
                  {loading ? (
                    <>
                      <svg
                        className="animate-spin w-4 h-4"
                        xmlns="http://www.w3.org/2000/svg"
                        fill="none"
                        viewBox="0 0 24 24"
                      >
                        <circle
                          className="opacity-25"
                          cx="12"
                          cy="12"
                          r="10"
                          stroke="currentColor"
                          strokeWidth="4"
                        />
                        <path
                          className="opacity-75"
                          fill="currentColor"
                          d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
                        />
                      </svg>
                      KI analysiert Ihre Compliance-Daten...
                    </>
                  ) : (
                    <>
                      <Bot className="w-4 h-4" />
                      Bericht generieren
                    </>
                  )}
                </Button>
                {!loading && (
                  <span className="text-xs text-secondary">
                    Modell: {REPORT_TYPES.find((r) => r.type === selected)?.title}
                  </span>
                )}
              </div>
            )}

            {error && !(error instanceof FeatureLockedError) && (
              <div className="flex items-center gap-2 text-sm text-red-400 bg-red-500/10 border border-red-500/30 rounded-lg px-4 py-3">
                <AlertTriangle className="w-4 h-4 shrink-0" />
                {error instanceof Error ? error.message : 'Unbekannter Fehler'}
              </div>
            )}

            {report && (
              <Card>
                <CardHeader className="pb-3">
                  <div className="flex items-center justify-between gap-3">
                    <CardTitle className="text-base flex items-center gap-2">
                      <CheckCircle className="w-4 h-4 text-green-400" />
                      {REPORT_TYPES.find((r) => r.type === selected)?.title}
                    </CardTitle>
                    <div className="flex items-center gap-2">
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => void copyToClipboard()}
                        className="gap-1.5 text-xs"
                      >
                        <Copy className="w-3.5 h-3.5" />
                        {copied ? 'Kopiert!' : 'Kopieren'}
                      </Button>
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={downloadAsTxt}
                        className="gap-1.5 text-xs"
                      >
                        <Download className="w-3.5 h-3.5" />
                        Als Text speichern
                      </Button>
                    </div>
                  </div>
                  <p className="text-xs text-secondary">
                    Erstellt am {new Date().toLocaleString('de-DE')}{aiModel ? ` — Modell: ${aiModel}` : ''}
                  </p>
                </CardHeader>
                <CardContent>
                  <pre className="text-sm text-primary leading-relaxed whitespace-pre-wrap font-sans">
                    {report}
                  </pre>
                </CardContent>
              </Card>
            )}
          </>
        )}

        {/* Loading state for availability check */}
        {available === null && (
          <div className="flex items-center gap-2 text-sm text-secondary">
            <svg
              className="animate-spin w-4 h-4"
              xmlns="http://www.w3.org/2000/svg"
              fill="none"
              viewBox="0 0 24 24"
            >
              <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
              <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
            </svg>
            Verbindung zu Ollama wird geprüft...
          </div>
        )}
      </div>
      </ProGate>
    </div>
  )
}
