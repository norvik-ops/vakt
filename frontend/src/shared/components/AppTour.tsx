import { useState, useEffect, useCallback, useRef } from 'react'
import { X, ChevronRight } from 'lucide-react'

const TOUR_COMPLETED_KEY = 'vakt_tour_completed'

interface TourStep {
  id: string
  /** CSS selector of the element to highlight */
  selector: string
  title: string
  text: string
}

const TOUR_STEPS: TourStep[] = [
  {
    id: 'dashboard',
    selector: 'a[href="/"][aria-current="page"], a[href="/"]',
    title: 'Dashboard',
    text: 'Hier siehst du deinen aktuellen Compliance-Status auf einen Blick.',
  },
  {
    id: 'frameworks',
    selector: 'a[href="/secvitals/frameworks"]',
    title: 'Frameworks (Vakt Comply)',
    text: 'Wähle dein Framework und dokumentiere Controls — NIS2, ISO 27001, BSI und mehr.',
  },
  {
    id: 'findings',
    selector: 'a[href="/secpulse/findings"]',
    title: 'Findings (Vakt Scan)',
    text: 'Scanner-Ergebnisse landen hier zur Priorisierung und Nachverfolgung.',
  },
  {
    id: 'policies',
    selector: 'a[href="/secvitals/policies"]',
    title: 'Richtlinien',
    text: 'Erstelle und versioniere Sicherheitsrichtlinien — mit KI-Unterstützung und Vorlagen.',
  },
  {
    id: 'checklist',
    selector: '[data-tour="getting-started"]',
    title: 'Getting Started Checkliste',
    text: 'Diese Checkliste führt dich durch die Ersteinrichtung deiner Sicherheitsplattform.',
  },
]

interface HighlightRect {
  top: number
  left: number
  width: number
  height: number
}

function getElementRect(selector: string): HighlightRect | null {
  // Try multiple selectors (comma-separated)
  const selectors = selector.split(',').map((s) => s.trim())
  for (const sel of selectors) {
    try {
      const el = document.querySelector(sel)
      if (el) {
        const rect = el.getBoundingClientRect()
        return {
          top: rect.top + window.scrollY,
          left: rect.left + window.scrollX,
          width: rect.width,
          height: rect.height,
        }
      }
    } catch {
      // Invalid selector — skip
    }
  }
  return null
}

interface TooltipPosition {
  top: number
  left: number
}

function computeTooltipPosition(
  rect: HighlightRect,
  tooltipW: number,
  tooltipH: number,
): TooltipPosition {
  const PADDING = 12
  const viewW = window.innerWidth
  const viewH = window.innerHeight

  // Prefer below, then above, then right, then left
  let top = rect.top + rect.height + PADDING
  let left = rect.left

  // Clamp horizontally
  if (left + tooltipW > viewW - PADDING) {
    left = viewW - tooltipW - PADDING
  }
  if (left < PADDING) left = PADDING

  // If below goes off screen, put above
  if (top + tooltipH > viewH + window.scrollY - PADDING) {
    top = rect.top - tooltipH - PADDING
  }
  if (top < PADDING) top = PADDING

  return { top, left }
}

export function AppTour() {
  const [active, setActive] = useState(false)
  const [step, setStep] = useState(0)
  const [rect, setRect] = useState<HighlightRect | null>(null)
  const [tooltipPos, setTooltipPos] = useState<TooltipPosition>({ top: 0, left: 0 })
  const tooltipRef = useRef<HTMLDivElement>(null)

  // Check if tour should be shown
  useEffect(() => {
    const done = localStorage.getItem(TOUR_COMPLETED_KEY)
    if (!done) {
      // Small delay to let DOM settle
      const t = setTimeout(() => setActive(true), 800)
      return () => clearTimeout(t)
    }
  }, [])

  const updatePosition = useCallback(() => {
    if (!active) return
    const currentStep = TOUR_STEPS[step]
    if (!currentStep) return
    const r = getElementRect(currentStep.selector)
    setRect(r)
    if (r && tooltipRef.current) {
      const tw = tooltipRef.current.offsetWidth || 280
      const th = tooltipRef.current.offsetHeight || 120
      setTooltipPos(computeTooltipPosition(r, tw, th))
    }
  }, [active, step])

  useEffect(() => {
    if (!active) return
    updatePosition()
    window.addEventListener('resize', updatePosition)
    window.addEventListener('scroll', updatePosition)
    return () => {
      window.removeEventListener('resize', updatePosition)
      window.removeEventListener('scroll', updatePosition)
    }
  }, [active, updatePosition])

  // Re-compute after tooltip renders
  useEffect(() => {
    if (!active) return
    const t = requestAnimationFrame(updatePosition)
    return () => cancelAnimationFrame(t)
  }, [active, step, updatePosition])

  function complete() {
    localStorage.setItem(TOUR_COMPLETED_KEY, '1')
    setActive(false)
  }

  function next() {
    if (step < TOUR_STEPS.length - 1) {
      setStep((s) => s + 1)
    } else {
      complete()
    }
  }

  if (!active) return null

  const currentStep = TOUR_STEPS[step]
  const isLast = step === TOUR_STEPS.length - 1

  return (
    <>
      {/* Dark overlay */}
      <div
        className="fixed inset-0 z-[9000] pointer-events-none"
        aria-hidden="true"
        style={{ background: 'rgba(0,0,0,0.45)' }}
      />

      {/* Pulsing highlight ring */}
      {rect && (
        <div
          aria-hidden="true"
          className="fixed z-[9001] pointer-events-none rounded-lg"
          style={{
            top: rect.top - 4,
            left: rect.left - 4,
            width: rect.width + 8,
            height: rect.height + 8,
            boxShadow: '0 0 0 4px rgba(99,102,241,0.8), 0 0 0 8px rgba(99,102,241,0.3)',
            animation: 'vakt-tour-pulse 2s ease-in-out infinite',
          }}
        />
      )}

      {/* Tooltip */}
      <div
        ref={tooltipRef}
        role="dialog"
        aria-modal="false"
        aria-label={currentStep.title}
        className="fixed z-[9002] w-72 bg-surface border border-border rounded-xl shadow-2xl p-4"
        style={{
          top: tooltipPos.top,
          left: tooltipPos.left,
        }}
      >
        {/* Skip button */}
        <button
          onClick={complete}
          aria-label="Tour überspringen"
          className="absolute top-3 right-3 text-secondary hover:text-primary transition-colors"
        >
          <X className="w-4 h-4" aria-hidden="true" />
        </button>

        {/* Step indicator */}
        <div className="flex gap-1 mb-3">
          {TOUR_STEPS.map((_, i) => (
            <span
              key={i}
              className={`h-1 rounded-full flex-1 transition-colors ${i === step ? 'bg-brand' : 'bg-border'}`}
              aria-hidden="true"
            />
          ))}
        </div>

        <p className="text-xs font-semibold text-brand mb-1">
          Schritt {step + 1} von {TOUR_STEPS.length}
        </p>
        <h3 className="text-sm font-semibold text-primary mb-1.5">{currentStep.title}</h3>
        <p className="text-xs text-secondary leading-relaxed mb-4">{currentStep.text}</p>

        <div className="flex items-center justify-between">
          <button
            onClick={complete}
            className="text-xs text-secondary hover:text-primary transition-colors underline"
          >
            Tour überspringen
          </button>
          <button
            onClick={next}
            className="flex items-center gap-1 px-3 py-1.5 text-xs font-medium bg-brand text-white rounded-md hover:bg-brand/90 transition-colors"
          >
            {isLast ? 'Fertig' : 'Weiter'}
            {!isLast && <ChevronRight className="w-3 h-3" aria-hidden="true" />}
          </button>
        </div>
      </div>

      {/* Inline keyframe animation */}
      <style>{`
        @keyframes vakt-tour-pulse {
          0%, 100% { opacity: 1; transform: scale(1); }
          50% { opacity: 0.7; transform: scale(1.03); }
        }
      `}</style>
    </>
  )
}
