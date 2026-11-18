// One label per platform role, shared by the team page (roleBadge, selects,
// legend), the public invite page and the missing-role error message, so an
// invitee reads the same role name the admin chose.
//
// The backend role values (Admin, SecurityAnalyst, Viewer, AuditorReadOnly,
// InternalAuditor) stay as they are — they are what roles.name stores and what
// a 403 names. Only the display changes: "AuditorReadOnly" and "Viewer" are
// internal words, not something an operator should have to translate.
//
// The lowercase names are the legacy invitation vocabulary; an older backend
// may still report them on GET /invite/info.
const ROLE_LABEL_KEYS: Record<string, string> = {
  Admin: 'settings.team.roleAdmin',
  SecurityAnalyst: 'settings.team.roleEditor',
  Viewer: 'settings.team.roleViewer',
  AuditorReadOnly: 'settings.team.roleAuditorReadOnly',
  InternalAuditor: 'settings.team.roleInternalAuditor',
  admin: 'settings.team.roleAdmin',
  editor: 'settings.team.roleEditor',
  viewer: 'settings.team.roleViewer',
}

// What each role may do, one short sentence, for the invite dialog and the
// legend below the member table.
const ROLE_DESCRIPTION_KEYS: Record<string, string> = {
  Admin: 'teamSettingsPage.roleAdmin',
  SecurityAnalyst: 'teamSettingsPage.roleEditor',
  Viewer: 'teamSettingsPage.roleViewer',
  AuditorReadOnly: 'teamSettingsPage.roleAuditorReadOnly',
  InternalAuditor: 'teamSettingsPage.roleInternalAuditor',
  admin: 'teamSettingsPage.roleAdmin',
  editor: 'teamSettingsPage.roleEditor',
  viewer: 'teamSettingsPage.roleViewer',
}

/** Translated label for a role name; unknown names are shown as they are. */
export function roleLabel(role: string, t: (key: string) => string): string {
  const key = ROLE_LABEL_KEYS[role]
  return key ? t(key) : role
}

/** Translated one-sentence description of a role; empty for unknown names. */
export function roleDescription(role: string, t: (key: string) => string): string {
  const key = ROLE_DESCRIPTION_KEYS[role]
  return key ? t(key) : ''
}
