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
const JUMP_KEY = 'vakt_milestone_last_jump_score'
const JUMP_THRESHOLD = 10

/**
 * Hook that fires a one-time toast whenever `score` crosses a milestone
 * threshold for the first time (persisted in localStorage per browser).
 * Also fires a one-time celebratory toast whenever the score jumps by
 * 10 percentage points or more compared to the previous baseline.
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

    // Threshold crossings (one-time per browser, persisted).
    const seen = new Set<number>(
      JSON.parse(localStorage.getItem(STORAGE_KEY) ?? '[]') as number[],
    )

    let firedThreshold = false
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
        firedThreshold = true
        break // only one toast at a time
      }
    }

    // Score-jump detection: notify on +10 percentage points since last baseline,
    // suppressed if a threshold toast already fired this tick to avoid double-toast.
    if (!firedThreshold) {
      const lastJump = parseFloat(localStorage.getItem(JUMP_KEY) ?? '')
      const baseline = !isNaN(lastJump) ? lastJump : prevScore.current
      if (baseline != null && score - baseline >= JUMP_THRESHOLD) {
        const delta = Math.round(score - baseline)
        localStorage.setItem(JUMP_KEY, String(score))
        toast(`📈 Score +${delta.toString()}% — du bist auf einem guten Weg!`, {
          variant: 'success',
          duration: 5000,
        })
      } else if (baseline == null) {
        // First observation in this browser — set baseline silently.
        localStorage.setItem(JUMP_KEY, String(score))
      }
    } else {
      // A threshold toast fired this tick. We must still advance the jump
      // baseline — otherwise a long-running session where every score change
      // happens to cross a threshold leaves JUMP_KEY frozen at the first
      // observed value, and a later remount with the same score then triggers
      // a phantom "+N%" toast (the very bug this branch was added to fix).
      localStorage.setItem(JUMP_KEY, String(score))
    }

    prevScore.current = score
  }, [score])
}
