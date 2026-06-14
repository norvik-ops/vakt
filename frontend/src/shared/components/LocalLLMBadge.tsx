import { ShieldCheck, Cloud } from 'lucide-react'
import { useTranslation } from 'react-i18next'

/**
 * LocalLLMBadge — Sprint 15 S15-8.
 *
 * Zeigt sichtbar an, ob die AI-Antwort von einem lokalen LLM (Ollama,
 * LM-Studio, llm-proxy) oder von einem Cloud-Provider (OpenAI, Mistral,
 * Groq, …) kam. Differential für das No-Phone-Home-Versprechen — wenn ein
 * Kunde unsicher ist ob "Daten das Haus verlassen", zeigt das Badge es
 * direkt am AI-Output.
 *
 * Erkennung über den `providerHost`-Prop: wenn der Backend-Host einer der
 * bekannten Container-Service-Discovery-Namen ist (ollama, ai-llm,
 * llm-proxy, lm-studio), gilt der Output als lokal. Sonst Cloud.
 *
 * Die Eingabe `providerHost` kommt aus dem AI-Status-Endpoint (bzw.
 * /health-artigen Feldern). Wenn nichts übergeben wurde, fällt das Badge
 * konservativ auf "lokal" zurück (Vakt-Default).
 */
interface Props {
  providerHost?: string
  model?: string
}

const LOCAL_HOSTS = new Set(['ollama', 'ai-llm', 'llm-proxy', 'lm-studio'])

function isLocal(host: string | undefined): boolean {
  if (!host) return true // konservativer Default: ohne Info als lokal anzeigen
  const lower = host.toLowerCase()
  for (const local of LOCAL_HOSTS) {
    if (lower.includes(local)) return true
  }
  // Loopback/RFC1918 wären auch lokal — der SSRF-Guard im Backend lehnt
  // andere private Adressen aktiv ab, also reicht der Service-Name-Check.
  return false
}

export function LocalLLMBadge({ providerHost, model }: Props) {
  const { t } = useTranslation()
  const local = isLocal(providerHost)

  if (local) {
    return (
      <span
        className="inline-flex items-center gap-1.5 text-[11px] font-medium text-emerald-700 bg-emerald-50 border border-emerald-200 rounded-full px-2 py-0.5"
        title={t('ai.localBadge.tooltip')}
      >
        <ShieldCheck className="w-3 h-3" aria-hidden="true" />
        {t('ai.localBadge.local', { model: model ?? 'lokal' })}
      </span>
    )
  }

  return (
    <span
      className="inline-flex items-center gap-1.5 text-[11px] font-medium text-amber-700 bg-amber-50 border border-amber-200 rounded-full px-2 py-0.5"
      title={t('ai.localBadge.cloudTooltip')}
    >
      <Cloud className="w-3 h-3" aria-hidden="true" />
      {t('ai.localBadge.cloud', { model: model ?? 'cloud' })}
    </span>
  )
}
