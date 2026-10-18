import { useEffect, useState } from 'react'
import { Switch } from './ui/switch'
import { Button } from './ui/button'
import { ProGate } from '../shared/components/ProGate'
import { FeatureLockedError } from '../api/client'
import { useUserPermissions, useUpdateUserPermissions, type ModulePermission } from '../hooks/useUserPermissions'
import { isModuleVisible, useEnabledModules } from '../shared/moduleVisibility'

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

// vakthr fehlt im Typ des Hooks, das Backend akzeptiert es aber (ModulePermissionInput, oneof … vakthr).
type ModuleKey = ModulePermission['module']

interface PermissionRow {
  module: ModuleKey
  can_read: boolean
  can_write: boolean
}

// Reihenfolge der Zeilen; angezeigt wird davon nur, was der Server aktiviert hat (GET /modules).
const ALL_MODULES: ModuleKey[] = ['vaktcomply', 'vaktscan', 'vaktvault', 'vaktaware', 'vaktprivacy', 'vakthr']

const MODULE_LABELS: Record<ModuleKey, string> = {
  vaktscan:    'Vakt Scan',
  vaktcomply:  'Vakt Comply',
  vaktvault:   'Vakt Vault',
  vaktaware:   'Vakt Aware',
  vaktprivacy: 'Vakt Privacy',
  vakthr:      'Vakt HR',
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/** visibleRows baut je aktiviertem Modul eine Zeile — gespeicherter Wert oder „kein Zugriff". */
function visibleRows(loaded: readonly PermissionRow[]): PermissionRow[] {
  return ALL_MODULES.filter((m) => isModuleVisible(m)).map((module) => {
    const match = loaded.find((p) => p.module === module)
    return match ? { module, can_read: match.can_read, can_write: match.can_write } : { module, can_read: false, can_write: false }
  })
}

/**
 * buildPayload ergänzt die sichtbaren Zeilen um die gespeicherten Rechte abgeschalteter Module.
 * PUT /admin/users/:id/permissions ersetzt alle Zeilen des Nutzers (DELETE + INSERT). Ohne diese
 * Ergänzung würde das Speichern Rechte für ein abgeschaltetes Modul still löschen — beim
 * Wiedereinschalten hätte der Nutzer sie nicht mehr.
 */
function buildPayload(rows: readonly PermissionRow[], loaded: readonly PermissionRow[]): PermissionRow[] {
  const shown = new Set(rows.map((r) => r.module))
  return [...rows, ...loaded.filter((p) => !shown.has(p.module))]
}

// ---------------------------------------------------------------------------
// Inner editor (rendered only when permissions loaded successfully)
// ---------------------------------------------------------------------------

interface EditorBodyProps {
  userId: string
  initial: ModulePermission[]
}

function EditorBody({ userId, initial }: EditorBodyProps) {
  // Abonniert die Modulliste, damit sich die Zeilen nach GET /modules neu aufbauen.
  const enabled = useEnabledModules((s) => s.modules)
  const [rows, setRows] = useState<PermissionRow[]>(() => visibleRows(initial))
  const update = useUpdateUserPermissions(userId)

  // Keep local state in sync if data reloads (e.g. after save)
  useEffect(() => {
    setRows(visibleRows(initial))
  }, [initial, enabled])

  function setRead(module: ModuleKey, value: boolean) {
    setRows((prev) =>
      prev.map((row) => {
        if (row.module !== module) return row
        // Turning off read also turns off write
        return { ...row, can_read: value, can_write: value ? row.can_write : false }
      }),
    )
  }

  function setWrite(module: ModuleKey, value: boolean) {
    setRows((prev) =>
      prev.map((row) => {
        if (row.module !== module) return row
        // Turning on write implies read
        return { ...row, can_write: value, can_read: value ? true : row.can_read }
      }),
    )
  }

  function handleSave() {
    update.mutate(buildPayload(rows, initial))
  }

  return (
    <div className="space-y-4">
      <table className="w-full text-sm">
        <thead>
          <tr className="text-xs font-semibold uppercase tracking-wide text-secondary border-b border-border">
            <th className="text-left pb-2 pr-4 w-full">Modul</th>
            <th className="text-center pb-2 px-4 whitespace-nowrap">Lesen</th>
            <th className="text-center pb-2 px-4 whitespace-nowrap">Schreiben</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => (
            <tr key={row.module} className="border-b border-border/50 last:border-0">
              <td className="py-3 pr-4 font-medium text-primary">
                {MODULE_LABELS[row.module]}
              </td>
              <td className="py-3 px-4 text-center">
                <Switch
                  checked={row.can_read}
                  onCheckedChange={(v) => { setRead(row.module, v); }}
                  aria-label={`${MODULE_LABELS[row.module]} Lesen`}
                />
              </td>
              <td className="py-3 px-4 text-center">
                <Switch
                  checked={row.can_write}
                  onCheckedChange={(v) => { setWrite(row.module, v); }}
                  aria-label={`${MODULE_LABELS[row.module]} Schreiben`}
                />
              </td>
            </tr>
          ))}
        </tbody>
      </table>

      {update.error && (
        <p className="text-sm text-destructive">{update.error.message}</p>
      )}

      <div className="flex justify-end pt-1">
        <Button onClick={handleSave} disabled={update.isPending} size="sm">
          {update.isPending ? 'Speichern...' : 'Berechtigungen speichern'}
        </Button>
      </div>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Public component
// ---------------------------------------------------------------------------

export interface UserPermissionsEditorProps {
  userId: string
}

export function UserPermissionsEditor({ userId }: UserPermissionsEditorProps) {
  const { data, isLoading, error } = useUserPermissions(userId)

  // Separate FeatureLockedError from other errors so ProGate can handle it
  const proError = error instanceof FeatureLockedError ? error : null
  const otherError = error && !(error instanceof FeatureLockedError) ? error : null

  return (
    <ProGate error={proError}>
      {isLoading && (
        <p className="text-sm text-secondary py-4 text-center">Lade Berechtigungen...</p>
      )}
      {otherError && (
        <p className="text-sm text-destructive py-4">{otherError.message}</p>
      )}
      {!isLoading && !error && data && (
        <EditorBody userId={userId} initial={data} />
      )}
    </ProGate>
  )
}
