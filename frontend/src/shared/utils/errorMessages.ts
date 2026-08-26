const ERROR_MAP: Record<string, string> = {
  // Auth
  AUTH_INVALID_CREDENTIALS: 'E-Mail oder Passwort ist falsch.',
  ACCOUNT_LOCKED: 'Konto vorübergehend gesperrt. Bitte warte 15 Minuten.',
  AUTH_TOKEN_REVOKED: 'Deine Sitzung wurde beendet. Bitte melde dich erneut an.',
  // Generic backend patterns (substring matching — keep specific phrases above general ones)
  'not found': 'Der Eintrag wurde nicht gefunden.',
  'already exists': 'Ein Eintrag mit diesen Daten existiert bereits.',
  unauthorized: 'Du hast keine Berechtigung für diese Aktion.',
  'invalid request': 'Die Eingabe ist ungültig. Bitte überprüfe deine Angaben.',
  'failed to create': 'Der Eintrag konnte nicht erstellt werden. Bitte versuche es erneut.',
  'failed to update': 'Die Änderungen konnten nicht gespeichert werden. Bitte versuche es erneut.',
  'failed to delete': 'Der Eintrag konnte nicht gelöscht werden.',
  'failed to list': 'Die Daten konnten nicht geladen werden.',
  'connection refused': 'Verbindung zum Server fehlgeschlagen.',
  timeout: 'Die Anfrage hat zu lange gedauert. Bitte versuche es erneut.',
  'HTTP 500': 'Serverfehler. Bitte versuche es in einem Moment erneut.',
  'HTTP 503': 'Der Dienst ist vorübergehend nicht verfügbar.',
}

// Matches the 403 body that auth.RequireRole and usermgmt.requireAdmin produce:
// `forbidden: requires role InternalAuditor` / `... Admin or SecurityAnalyst`.
// The backend names the role there on purpose — a bare "forbidden" left the
// operator with no idea which role was missing, and the obvious self-help was an
// UPDATE against the database (ESK-13).
const MISSING_ROLE_RE = /^forbidden: requires role (.+)$/

/**
 * describeMissingRole turns the backend's insufficient-role 403 into a sentence
 * that says which role is missing and where to grant it. Returns null for every
 * other message, so callers can fall back to their normal handling.
 */
export function describeMissingRole(message: string): string | null {
  const m = MISSING_ROLE_RE.exec(message.trim())
  if (!m) return null
  const roles = m[1].split(' or ').map(r => r.trim()).filter(Boolean)
  if (roles.length === 0) return null
  const list = roles.length === 1 ? `„${roles[0]}"` : roles.map(r => `„${r}"`).join(' oder ')
  return (
    `Für diese Aktion fehlt dir die Rolle ${list}. ` +
    'Ein Administrator vergibt sie unter Einstellungen → Team.'
  )
}

export function humanizeError(error: unknown): string {
  const msg = error instanceof Error ? error.message : String(error)

  // Role-specific 403 before the generic map: the generic entries would swallow
  // the role name that makes this message useful.
  const missingRole = describeMissingRole(msg)
  if (missingRole) return missingRole

  // Exact match first
  if (ERROR_MAP[msg]) return ERROR_MAP[msg]

  // Substring match (case-insensitive)
  const lower = msg.toLowerCase()
  for (const [key, value] of Object.entries(ERROR_MAP)) {
    if (lower.includes(key.toLowerCase())) return value
  }

  // If it looks like a technical code (all caps with underscores), return generic
  if (/^[A-Z_]+$/.test(msg)) return 'Ein unerwarteter Fehler ist aufgetreten.'

  // Otherwise return as-is (might be already user-friendly)
  return msg
}

export function handleApiError(error: unknown, fallback = 'Ein Fehler ist aufgetreten.'): string {
  return humanizeError(error) || fallback
}
