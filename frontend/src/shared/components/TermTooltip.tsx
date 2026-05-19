import { useState } from 'react'
import { HelpCircle } from 'lucide-react'

interface TermTooltipProps {
  term: string
  explanation: string
  children?: React.ReactNode
}

let _id = 0
function nextId() { return `term-tooltip-${++_id}` }

export function TermTooltip({ term, explanation, children }: TermTooltipProps) {
  const [visible, setVisible] = useState(false)
  const [tooltipId] = useState(nextId)

  return (
    <span
      className="relative inline-flex items-center gap-1 group cursor-help border-b border-dashed border-slate-500"
      tabIndex={0}
      role="button"
      aria-describedby={tooltipId}
      onMouseEnter={() => setVisible(true)}
      onMouseLeave={() => setVisible(false)}
      onFocus={() => setVisible(true)}
      onBlur={() => setVisible(false)}
    >
      {children ?? term}
      <HelpCircle className="w-3 h-3 text-slate-500 shrink-0" aria-hidden="true" />
      <span
        id={tooltipId}
        role="tooltip"
        className={[
          'pointer-events-none absolute bottom-full left-1/2 -translate-x-1/2 mb-2 z-50',
          'w-64 rounded-md bg-gray-900 px-3 py-2 text-xs leading-relaxed text-white shadow-lg',
          'transition-opacity duration-150',
          visible ? 'opacity-100' : 'opacity-0',
        ].join(' ')}
      >
        {explanation}
        <span className="absolute top-full left-1/2 -translate-x-1/2 border-4 border-transparent border-t-gray-900" />
      </span>
    </span>
  )
}
