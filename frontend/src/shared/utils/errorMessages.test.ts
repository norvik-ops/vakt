import { describe, it, expect } from 'vitest'
import { describeMissingRole, humanizeError } from './errorMessages'

// The ESK-13 half that the user actually reads. The backend names the missing
// role in the 403 body; main.tsx's global mutation-error toast runs every error
// message through describeMissingRole, so this is the function that decides
// whether the operator learns which role is missing or sees "forbidden".
//
// Non-vacuity: every assertion here fails if describeMissingRole stops
// extracting the role (returning null, or a fixed sentence without the name).
describe('describeMissingRole', () => {
  it('names the missing role and where to get it', () => {
    const msg = describeMissingRole('forbidden: requires role InternalAuditor')

    expect(msg).toContain('InternalAuditor')
    expect(msg).toContain('Einstellungen → Team')
  })

  it('lists every accepted role when the route allows several', () => {
    const msg = describeMissingRole('forbidden: requires role Admin or SecurityAnalyst')

    expect(msg).toContain('Admin')
    expect(msg).toContain('SecurityAnalyst')
  })

  it('returns null for anything else, so other errors pass through untouched', () => {
    expect(describeMissingRole('forbidden')).toBeNull()
    expect(describeMissingRole('Der Eintrag wurde nicht gefunden.')).toBeNull()
    expect(describeMissingRole('')).toBeNull()
    // A bare "forbidden" is exactly the pre-fix body — it must not be dressed up
    // as a helpful message, because it carries no role to report.
    expect(describeMissingRole('forbidden: requires role ')).toBeNull()
  })

  it('is reached through humanizeError before the generic map swallows it', () => {
    // Asserted against the whole sentence, not just the role name: without the
    // early branch humanizeError falls through to "return as-is" and hands back
    // the raw API string, which also contains "InternalAuditor" — a substring
    // check would stay green for a change that removed the helpful text.
    expect(humanizeError(new Error('forbidden: requires role InternalAuditor'))).toBe(
      'Für diese Aktion fehlt dir die Rolle „InternalAuditor". ' +
        'Ein Administrator vergibt sie unter Einstellungen → Team.',
    )
  })

  it('leaves unrelated messages to the existing mapping', () => {
    expect(humanizeError(new Error('AUTH_INVALID_CREDENTIALS')))
      .toBe('E-Mail oder Passwort ist falsch.')
  })
})
