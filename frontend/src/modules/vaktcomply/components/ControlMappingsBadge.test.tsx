/**
 * UX-18: „Auch abgedeckt in: NIS2 NIS2-A.1" erschien doppelt, die Farben grün/gelb
 * waren nicht erklärt, der Tooltip englisch („(equivalent)").
 */
import { describe, it, expect, vi } from 'vitest'
import { screen } from '@testing-library/react'
import { renderWithProviders } from '../../../test-utils'
import { ControlMappingsBadge } from './ControlMappingsBadge'
import { dedupeMappings, mappingChipLabel } from '../utils/controlMappings'
import type { ControlMapping } from '../hooks/useControlMappings'

const base: ControlMapping = {
  id: 'm1', source_framework: 'ISO27001', source_control_code: 'A.5.1',
  target_framework: 'NIS2', target_control_code: 'NIS2-A.1', mapping_type: 'equivalent',
  target_control_id: 'c-nis2-a1', target_control_title: 'Richtlinien zur Risikoanalyse', target_framework_name: 'NIS2',
}

let mappings: ControlMapping[] = []
vi.mock('../../../api/client', () => ({
  apiFetch: () => Promise.resolve({ mappings }),
  registerUnauthorizedHandler: vi.fn(),
}))

describe('dedupeMappings', () => {
  it('eine Zielkontrolle = ein Eintrag, die stärkere Art gewinnt', () => {
    const out = dedupeMappings([
      { ...base, id: 'm1', mapping_type: 'partial' },
      { ...base, id: 'm2', mapping_type: 'equivalent' },
      { ...base, id: 'm3', target_control_code: 'NIS2-A.2', mapping_type: 'partial' },
    ])
    expect(out).toHaveLength(2)
    expect(out.find((m) => m.target_control_code === 'NIS2-A.1')?.mapping_type).toBe('equivalent')
  })
})

describe('mappingChipLabel', () => {
  it('wiederholt das Framework nicht, wenn der Code es schon trägt', () => {
    expect(mappingChipLabel(base)).toBe('NIS2-A.1')
  })
  it('stellt das Framework voran, wenn der Code es nicht trägt', () => {
    expect(mappingChipLabel({ target_framework: 'ISO27001', target_control_code: 'A.5.1' })).toMatch(/^ISO.*27001 A\.5\.1$/)
  })
})

describe('ControlMappingsBadge', () => {
  it('zeigt doppelte Zuordnungen einmal und erklärt die Farben', async () => {
    mappings = [{ ...base, id: 'm1' }, { ...base, id: 'm2' }, { ...base, id: 'm3', target_control_code: 'NIS2-B.4', mapping_type: 'partial' }]
    renderWithProviders(<ControlMappingsBadge controlId="ctl-1" />)
    const chips = await screen.findAllByTestId('control-mapping-chip')
    expect(chips.map((c) => c.textContent)).toEqual(['NIS2-A.1', 'NIS2-B.4'])
    expect(chips[0]).toHaveAttribute('title', expect.stringContaining('(Gleichwertig)'))
    expect(chips[0].getAttribute('title')).not.toMatch(/equivalent/)
    const legend = screen.getByTestId('control-mappings-legend')
    expect(legend).toHaveTextContent('Gleichwertig')
    expect(legend).toHaveTextContent('Teilweise')
    // Nur Arten, die vorkommen, stehen in der Legende.
    expect(legend).not.toHaveTextContent('Informativ')
  })
})
