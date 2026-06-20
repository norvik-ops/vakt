import { Skeleton } from '../../components/ui/skeleton'

/**
 * A table skeleton — rows x cols of shimmering blocks that mimic a data table.
 * Cells alternate between wide (60%) and narrow (30%) widths to look realistic.
 */
export function SkeletonTable({ rows = 5, cols = 4 }: { rows?: number; cols?: number }) {
  return (
    <div className="vakt-fade-in space-y-2" role="status" aria-busy="true" aria-label="Tabelle wird geladen">
      {/* Header row */}
      <div className="flex gap-3 px-1 pb-2 border-b border-border">
        {Array.from({ length: cols }).map((_, i) => (
          <Skeleton
            key={i}
            className="h-4 rounded"
            style={{ width: i % 2 === 0 ? '15%' : '10%' }}
          />
        ))}
      </div>
      {/* Data rows */}
      {Array.from({ length: rows }).map((_, rowIdx) => (
        <div key={rowIdx} className="flex gap-3 items-center px-1 py-2 border-b border-border/40">
          {Array.from({ length: cols }).map((_, colIdx) => (
            <Skeleton
              key={colIdx}
              className="h-4 rounded"
              style={{ width: colIdx % 2 === 0 ? '60%' : '30%' }}
            />
          ))}
        </div>
      ))}
    </div>
  )
}

/**
 * A card grid skeleton — count cards in a responsive grid.
 * Each card shows a title block, 2-3 line blocks, and a badge block.
 */
export function SkeletonCardGrid({ count = 6 }: { count?: number }) {
  return (
    <div
      className="vakt-fade-in grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4"
      role="status"
      aria-busy="true"
      aria-label="Karten werden geladen"
    >
      {Array.from({ length: count }).map((_, i) => (
        <div
          key={i}
          className="rounded-lg border border-border bg-surface p-5 space-y-3"
        >
          {/* Title */}
          <Skeleton className="h-4 w-3/4 rounded" />
          {/* Subtitle */}
          <Skeleton className="h-3 w-1/2 rounded" />
          {/* Body lines */}
          <div className="space-y-2 pt-1">
            <Skeleton className="h-3 w-full rounded" />
            <Skeleton className="h-3 w-5/6 rounded" />
          </div>
          {/* Badge */}
          <div className="flex gap-2 pt-1">
            <Skeleton className="h-5 w-16 rounded-full" />
            <Skeleton className="h-5 w-20 rounded-full" />
          </div>
        </div>
      ))}
    </div>
  )
}

/**
 * A detail page skeleton — full-width header, two-column layout (2/3 + 1/3).
 */
export function SkeletonDetailPage() {
  return (
    <div className="vakt-fade-in space-y-6" role="status" aria-busy="true" aria-label="Seite wird geladen">
      {/* Full-width header block */}
      <div className="space-y-2">
        <Skeleton className="h-7 w-1/3 rounded" />
        <Skeleton className="h-4 w-2/3 rounded" />
      </div>

      {/* Two-column layout */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Main content — 2/3 */}
        <div className="lg:col-span-2 space-y-6">
          {/* Section 1 */}
          <div className="rounded-lg border border-border bg-surface p-5 space-y-4">
            <Skeleton className="h-5 w-1/4 rounded" />
            <div className="space-y-2">
              <Skeleton className="h-4 w-full rounded" />
              <Skeleton className="h-4 w-5/6 rounded" />
              <Skeleton className="h-4 w-4/6 rounded" />
            </div>
          </div>
          {/* Section 2 */}
          <div className="rounded-lg border border-border bg-surface p-5 space-y-4">
            <Skeleton className="h-5 w-1/3 rounded" />
            <div className="space-y-2">
              <Skeleton className="h-4 w-full rounded" />
              <Skeleton className="h-4 w-3/4 rounded" />
            </div>
          </div>
          {/* Section 3 */}
          <div className="rounded-lg border border-border bg-surface p-5 space-y-4">
            <Skeleton className="h-5 w-1/4 rounded" />
            <div className="space-y-3">
              <Skeleton className="h-12 w-full rounded" />
              <Skeleton className="h-12 w-full rounded" />
            </div>
          </div>
        </div>

        {/* Sidebar — 1/3 */}
        <div className="space-y-4">
          <div className="rounded-lg border border-border bg-surface p-5 space-y-3">
            <Skeleton className="h-5 w-1/2 rounded" />
            <Skeleton className="h-4 w-full rounded" />
            <Skeleton className="h-4 w-3/4 rounded" />
            <Skeleton className="h-4 w-2/3 rounded" />
          </div>
          <div className="rounded-lg border border-border bg-surface p-5 space-y-3">
            <Skeleton className="h-5 w-1/3 rounded" />
            <Skeleton className="h-4 w-full rounded" />
            <Skeleton className="h-4 w-1/2 rounded" />
          </div>
        </div>
      </div>
    </div>
  )
}

/**
 * A single stat card skeleton — for dashboard widgets.
 * Shows a number block and a label block.
 */
export function SkeletonStatCard() {
  return (
    <div
      className="vakt-fade-in rounded-lg border border-border bg-surface p-5 space-y-2"
      role="status"
      aria-busy="true"
      aria-label="Statistik wird geladen"
    >
      <Skeleton className="h-8 w-1/2 rounded" />
      <Skeleton className="h-4 w-2/3 rounded" />
    </div>
  )
}
