import { useEffect, useRef } from 'react'
import { toast } from '../hooks/useToast'

const MILESTONES = [
  { threshold: 25, label: 'Einstiegsbasis erreicht', emoji: '🏁' },
  { threshold: 50, label: 'Halbzeit — gute Arbeit!', emoji: '⭐' },
  { threshold: 75, label: 'Fortgeschrittener Stand', emoji: '🚀' },
  { threshold: 90, label: 'Exzellente Compliance!', emoji: '🏆' },
  { threshold: 100, label: 'Vollständige Compliance erreicht!', emoji: '🎉' },
]

const STORAGE_KEY = 'vakt_milestone_seen'

/**
 * Hook that fires a one-time toast whenever `score` crosses a milestone
 * threshold for the first time (persisted in localStorage per browser).
 *
 * Usage: call at the top of any component that has a numeric compliance score.
 *
 * @param score  Current compliance/readiness score as a percentage (0–100).
 *               Pass `undefined` while data is still loading.
 */
export function useMilestoneToast(score: number | undefined) {
  const prevScore = useRef<number | undefined>(undefined)

  useEffect(() => {
    if (score == null) return

    const seen = new Set<number>(
      JSON.parse(localStorage.getItem(STORAGE_KEY) ?? '[]') as number[],
    )

    for (const m of MILESTONES) {
      if (
        score >= m.threshold &&
        !seen.has(m.threshold) &&
        (prevScore.current == null || prevScore.current < m.threshold)
      ) {
        seen.add(m.threshold)
        localStorage.setItem(STORAGE_KEY, JSON.stringify([...seen]))
        toast(`${m.emoji} ${m.label} — Compliance-Score: ${Math.round(score)}%`, {
          variant: 'success',
          duration: 6000,
        })
        break // only one toast at a time
      }
    }

    prevScore.current = score
  }, [score])
}
