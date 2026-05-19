import { useState, useCallback, useEffect } from 'react'
import * as DropdownMenu from '@radix-ui/react-dropdown-menu'
import { Columns } from 'lucide-react'
import { Button } from '../../components/ui/button'
import { cn } from '../../lib/utils'

interface Column {
  key: string
  label: string
  defaultVisible?: boolean
}

interface ColumnPickerProps {
  columns: Column[]
  visibleColumns: string[]
  onVisibilityChange: (visibleColumns: string[]) => void
  storageKey?: string
}

function readFromStorage(key: string, fallback: string[]): string[] {
  try {
    const raw = localStorage.getItem(key)
    if (!raw) return fallback
    const parsed = JSON.parse(raw)
    return Array.isArray(parsed) ? (parsed as string[]) : fallback
  } catch {
    return fallback
  }
}

function writeToStorage(key: string, value: string[]): void {
  try {
    localStorage.setItem(key, JSON.stringify(value))
  } catch {
    // localStorage may be unavailable
  }
}

export function ColumnPicker({
  columns,
  visibleColumns,
  onVisibilityChange,
  storageKey,
}: ColumnPickerProps) {
  const [initialized, setInitialized] = useState(false)

  useEffect(() => {
    if (initialized || !storageKey) return
    const stored = readFromStorage(storageKey, visibleColumns)
    if (stored.join(',') !== visibleColumns.join(',')) {
      onVisibilityChange(stored)
    }
    setInitialized(true)
  }, [initialized, storageKey, visibleColumns, onVisibilityChange])

  const toggleColumn = useCallback(
    (key: string) => {
      const isVisible = visibleColumns.includes(key)
      if (isVisible && visibleColumns.length === 1) return
      const next = isVisible
        ? visibleColumns.filter((k) => k !== key)
        : [...visibleColumns, key]
      onVisibilityChange(next)
      if (storageKey) writeToStorage(storageKey, next)
    },
    [visibleColumns, onVisibilityChange, storageKey],
  )

  return (
    <DropdownMenu.Root>
      <DropdownMenu.Trigger asChild>
        <Button variant="outline" size="sm" className="gap-1.5">
          <Columns className="w-3.5 h-3.5" aria-hidden="true" />
          Spalten
        </Button>
      </DropdownMenu.Trigger>
      <DropdownMenu.Portal>
        <DropdownMenu.Content
          align="end"
          sideOffset={4}
          className={cn(
            'z-50 min-w-[180px] rounded-lg border border-border bg-surface shadow-md',
            'p-1 text-[13px]',
            'animate-in fade-in-0 zoom-in-95',
          )}
        >
          <p className="px-2 py-1.5 text-[11px] font-semibold text-secondary uppercase tracking-wider opacity-60">
            Spalten anzeigen
          </p>
          {columns.map((col) => {
            const isVisible = visibleColumns.includes(col.key)
            const isLast = isVisible && visibleColumns.length === 1
            return (
              <DropdownMenu.Item
                key={col.key}
                onSelect={(e) => {
                  e.preventDefault()
                  toggleColumn(col.key)
                }}
                disabled={isLast}
                className={cn(
                  'flex items-center gap-2 px-2 py-1.5 rounded-md cursor-pointer select-none',
                  'text-primary outline-none transition-colors duration-100',
                  'data-[highlighted]:bg-muted/50',
                  'data-[disabled]:opacity-40 data-[disabled]:cursor-not-allowed',
                )}
              >
                <span
                  className={cn(
                    'flex h-4 w-4 shrink-0 items-center justify-center rounded border border-border transition-colors',
                    isVisible ? 'bg-brand border-brand' : 'bg-transparent',
                  )}
                  aria-hidden="true"
                >
                  {isVisible && (
                    <svg
                      xmlns="http://www.w3.org/2000/svg"
                      viewBox="0 0 12 12"
                      fill="none"
                      className="w-2.5 h-2.5"
                    >
                      <path
                        d="M2 6l3 3 5-5"
                        stroke="white"
                        strokeWidth="1.5"
                        strokeLinecap="round"
                        strokeLinejoin="round"
                      />
                    </svg>
                  )}
                </span>
                {col.label}
              </DropdownMenu.Item>
            )
          })}
        </DropdownMenu.Content>
      </DropdownMenu.Portal>
    </DropdownMenu.Root>
  )
}
