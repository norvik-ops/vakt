// One label per platform role, shared by the team page (roleBadge) and the
// public invite page, so an invitee reads the same role name the admin chose.
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

/** Translated label for a role name; unknown names are shown as they are. */
export function roleLabel(role: string, t: (key: string) => string): string {
  const key = ROLE_LABEL_KEYS[role]
  return key ? t(key) : role
}
