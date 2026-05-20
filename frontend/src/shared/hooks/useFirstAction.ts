import { useEffect, useRef } from 'react'
import { toast } from './useToast'

const STORAGE_KEY = 'vakt_first_action_seen'

interface GuidanceMessage {
  /** Text shown to the user (German). */
  text: string
  /** Optional action label — wires a button into the toast for one-click follow-through. */
  actionLabel?: string
  /** Path to navigate to when the action is clicked. */
  actionHref?: string
}

const GUIDANCE: Record<string, GuidanceMessage> = {
  'control:first-created': {
    text: '💡 Erstes Control angelegt — als Nächstes Evidenz hochladen, damit es als „umgesetzt" zählt.',
    actionLabel: 'Evidenz hochladen',
  },
  'incident:first-created': {
    text: '💡 Erster Vorfall registriert — vergiss nicht, im Status-Verlauf jeden Schritt zu dokumentieren (Audit-Nachweis).',
  },
  'risk:first-created': {
    text: '💡 Erstes Risiko angelegt — wähle eine Behandlungsmethode (akzeptieren / mindern / übertragen / vermeiden), damit das Risiko bei der Bewertung zählt.',
  },
  'policy:first-created': {
    text: '💡 Erste Richtlinie veröffentlicht — Mitarbeiter müssen sie akzeptieren; das geht über den öffentlichen Link unter „Akzeptanz".',
  },
  'asset:first-created': {
    text: '💡 Erstes Asset registriert — Scanner-Findings können jetzt diesem Asset zugeordnet werden (Trivy / Nuclei via Pull-Integration).',
  },
  'supplier:first-created': {
    text: '💡 Erster Lieferant angelegt — schicke ihm einen Selbstauskunft-Fragebogen, der als Compliance-Evidenz zählt.',
  },
  'evidence:first-uploaded': {
    text: '🎉 Erste Evidenz hochgeladen — das Control wechselt automatisch auf „umgesetzt", sobald alle Pflicht-Evidenzen vorhanden sind.',
  },
  'campaign:first-launched': {
    text: '💡 Erste Phishing-Kampagne läuft — denk daran, im Betriebsrat-Modus zu bleiben (keine personenbezogenen Daten gespeichert).',
  },
}

function loadSeen(): Set<string> {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return new Set()
    const parsed = JSON.parse(raw) as unknown
    if (Array.isArray(parsed)) return new Set(parsed as string[])
    return new Set()
  } catch {
    return new Set()
  }
}

function saveSeen(seen: Set<string>): void {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify([...seen]))
  } catch {
    // localStorage may be unavailable
  }
}

/**
 * Fires a one-time educational toast the first time the user creates an entity
 * of a given kind. Subsequent creations are silent — the goal is to teach the
 * model, not to nag.
 *
 * Persistence is per browser via localStorage (`vakt_first_action_seen`).
 * Reset by clearing localStorage — useful for demoing the onboarding.
 *
 * Usage in a list page:
 *
 *   const items = useItems()  // some hook returning [] when empty
 *   useFirstAction('control:first-created', items.length > 0)
 *
 * The hook fires only when items transitions from empty (or undefined) to
 * non-empty during the component's lifetime — it doesn't fire on the initial
 * page load when items are already > 0 from a previous session.
 *
 * @param key   Stable identifier for the action (e.g. 'control:first-created').
 *              Must be in the GUIDANCE map.
 * @param condition Boolean that becomes true when the first item exists.
 */
export function useFirstAction(key: string, condition: boolean): void {
  const wasTrue = useRef(condition)

  useEffect(() => {
    if (!condition) {
      wasTrue.current = false
      return
    }
    // Only fire on the transition false → true, not on initial mount-with-true.
    if (wasTrue.current) return
    wasTrue.current = true

    const seen = loadSeen()
    if (seen.has(key)) return

    const guidance = GUIDANCE[key]
    if (!guidance) return

    seen.add(key)
    saveSeen(seen)

    toast(guidance.text, {
      variant: 'info',
      duration: 8000,
    })
  }, [key, condition])
}
