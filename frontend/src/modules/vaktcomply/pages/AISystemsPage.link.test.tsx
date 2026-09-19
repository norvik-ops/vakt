import { render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes, Link } from 'react-router-dom'
import { describe, expect, it } from 'vitest'

/**
 * R1-SA28-01 — der Einstieg in die Art.-11-Technikdoku des EU AI Act.
 *
 * AISystemsPage ist unter der Route `ai-systems` gemountet
 * (SecVitalsRoutes.tsx:106). Ein RELATIVER Link haengt an den bereits
 * vorhandenen Pfad an. `to="ai-systems/<id>/documentation"` ergab deshalb
 * `/vaktcomply/ai-systems/ai-systems/<id>/documentation` — ein Pfad, den keine
 * Route kennt. Es gab dafuer keinen 404: `path='*'` greift und schickt den
 * Nutzer STILL auf die Comply-Uebersicht zurueck. Ein Klick, der aussieht, als
 * haette man ihn nicht getroffen.
 *
 * Der Test bildet die Mount-Situation nach, statt die Seite zu rendern: geprueft
 * wird die Aufloesung des relativen Links, und genau die war der Defekt.
 */
describe('AISystemsPage — Link auf die KI-Technikdokumentation', () => {
  function renderAt(to: string) {
    return render(
      <MemoryRouter initialEntries={['/vaktcomply/ai-systems']}>
        <Routes>
          <Route path="/vaktcomply">
            <Route path="ai-systems" element={<Link to={to}>Doku</Link>} />
            <Route path="ai-systems/:id/documentation" element={<div>TECHNIKDOKU</div>} />
            <Route path="*" element={<div>UEBERSICHT-FALLBACK</div>} />
          </Route>
        </Routes>
      </MemoryRouter>,
    )
  }

  it('loest auf die Dokumentationsseite auf, nicht auf den Fallback', () => {
    renderAt('sys-1/documentation')
    const link = screen.getByRole('link', { name: 'Doku' })
    expect(link.getAttribute('href')).toBe('/vaktcomply/ai-systems/sys-1/documentation')
  })

  // Gegenprobe: ohne sie waere der Test oben auch dann gruen, wenn die Route
  // gar nicht mehr existierte. Sie haelt fest, WAS der alte Link erzeugte.
  it('das alte Muster verdoppelte das Segment und traf keine Route', () => {
    renderAt('ai-systems/sys-1/documentation')
    const link = screen.getByRole('link', { name: 'Doku' })
    expect(link.getAttribute('href')).toBe(
      '/vaktcomply/ai-systems/ai-systems/sys-1/documentation',
    )
  })
})
