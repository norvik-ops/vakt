import { describe, it, expect } from 'vitest'
import { showAutoRenewalBadge } from './licenseDisplay'

// UX-04: eine Community-Instanz ohne Schlüssel zeigte „Community · Auto-Renewal aktiv",
// weil die Plakette nur am Config-Schalter hing.
describe('showAutoRenewalBadge', () => {
  const pro = { is_pro: true, demo: false, auto_renewal_enabled: true, renewal_token_set: true }

  it('zeigt die Plakette bei Pro-Schlüssel mit Token und eingeschaltetem Auto-Renewal', () => {
    expect(showAutoRenewalBadge(pro)).toBe(true)
  })

  it('verbirgt sie auf Community ohne Schlüssel, obwohl der Schalter an ist', () => {
    expect(showAutoRenewalBadge({ is_pro: false, demo: false, auto_renewal_enabled: true, renewal_token_set: false })).toBe(false)
  })

  it('verbirgt sie bei Pro ohne Token', () => {
    expect(showAutoRenewalBadge({ ...pro, renewal_token_set: false })).toBe(false)
  })

  it('verbirgt sie, wenn ein älteres Backend das Feld nicht liefert', () => {
    expect(showAutoRenewalBadge({ is_pro: true, demo: false, auto_renewal_enabled: true })).toBe(false)
  })

  it('verbirgt sie bei abgeschaltetem Auto-Renewal und bei Demo-Lizenzen', () => {
    expect(showAutoRenewalBadge({ ...pro, auto_renewal_enabled: false })).toBe(false)
    expect(showAutoRenewalBadge({ ...pro, demo: true })).toBe(false)
  })

  it('verbirgt sie, solange die Lizenz noch lädt', () => {
    expect(showAutoRenewalBadge(undefined)).toBe(false)
  })
})
