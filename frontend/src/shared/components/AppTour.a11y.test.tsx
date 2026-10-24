import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, screen, fireEvent, act, waitFor } from '@testing-library/react'

import { AppTour } from './AppTour'
import { useEnabledModules } from '../moduleVisibility'

vi.mock('../hooks/useWhatsNew', () => ({
  useWhatsNew: () => ({ isNew: false, currentVersion: '0.6.0', dismiss: vi.fn() }),
}))

const TOUR_COMPLETED_KEY = 'vakt_tour_completed'

describe('AppTour', () => {
  beforeEach(() => {
    localStorage.removeItem(TOUR_COMPLETED_KEY)
  })

  it('opens 800ms after mount when not previously completed', async () => {
    render(<AppTour />)
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    await act(async () => {
      await new Promise((r) => setTimeout(r, 850))
    })
    expect(screen.getByRole('dialog')).toBeInTheDocument()
  })

  it('does not open if tour was previously completed', async () => {
    localStorage.setItem(TOUR_COMPLETED_KEY, '1')
    render(<AppTour />)
    await act(async () => {
      await new Promise((r) => setTimeout(r, 850))
    })
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('closes when Escape is pressed and marks completed', async () => {
    render(<AppTour />)
    await act(async () => {
      await new Promise((r) => setTimeout(r, 850))
    })
    expect(screen.getByRole('dialog')).toBeInTheDocument()
    fireEvent.keyDown(document, { key: 'Escape' })
    await waitFor(() => {
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    })
    expect(localStorage.getItem(TOUR_COMPLETED_KEY)).toBe('1')
  })

  it('moves keyboard focus to the tooltip when opened', async () => {
    render(<AppTour />)
    await act(async () => {
      await new Promise((r) => setTimeout(r, 850))
    })
    await waitFor(() => {
      expect(document.activeElement).toBe(screen.getByRole('dialog'))
    })
  })
})

describe('AppTour mit abgeschalteten Modulen', () => {
  beforeEach(() => {
    localStorage.removeItem(TOUR_COMPLETED_KEY)
  })

  afterEach(() => {
    useEnabledModules.setState({ modules: null })
  })

  async function stepCount(): Promise<string> {
    render(<AppTour />)
    await act(async () => {
      await new Promise((r) => setTimeout(r, 850))
    })
    return screen.getByRole('dialog').textContent ?? ''
  }

  it('hat keinen Findings-Schritt mehr (Vakt Scan ist archiviert)', async () => {
    useEnabledModules.setState({ modules: new Set(['vaktcomply']) })
    const text = await stepCount()
    // tour.step-Schlüssel wird im Test ohne Übersetzung gerendert; die Anzahl der Fortschrittsbalken zählt.
    expect(document.querySelectorAll('[role="dialog"] .flex.gap-1 > span')).toHaveLength(4)
    expect(text).not.toContain('tour.steps.findings.title')
  })
})
