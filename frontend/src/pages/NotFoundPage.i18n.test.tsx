/**
 * R1-KW-41: Fünf Schlüssel fehlten in allen Katalogen; die Aufrufe trugen einen
 * deutschen Text-Default, also zeigten die 404-Seite und das Feld „Datenpannen-ID"
 * in en/fr/nl deutschen Text. Jetzt stehen sie in jedem Katalog.
 */
import { describe, it, expect, afterEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import i18n from '../i18n'
import NotFoundPage from './NotFoundPage'

const KEYS = [
  'errors.notFound.title',
  'errors.notFound.description',
  'errors.notFound.back',
  'errors.notFound.searchHint',
  'vaktcomply.incidentsPage.labelBreachId',
]

afterEach(async () => { await i18n.changeLanguage('de') })

describe('R1-KW-41 — fehlende i18n-Schlüssel', () => {
  for (const lng of ['de', 'en', 'fr', 'nl']) {
    it(`stehen im Katalog ${lng}`, () => {
      // Direkt im Ressourcen-Bündel nachsehen — i18n.exists fiele auf de zurück.
      for (const key of KEYS) {
        expect(i18n.getResource(lng, 'translation', key), `${lng}: ${key}`).toEqual(expect.any(String))
      }
    })
  }

  it('die 404-Seite spricht Englisch, wenn Englisch eingestellt ist', async () => {
    await i18n.changeLanguage('en')
    render(<MemoryRouter><NotFoundPage /></MemoryRouter>)
    expect(screen.getByRole('heading', { name: 'Page not found' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Back to the start page' })).toBeInTheDocument()
  })
})
