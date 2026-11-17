import { describe, it, expect } from 'vitest'
import { screen } from '@testing-library/react'
import { renderWithProviders } from '../../../test-utils'
import RiskHeatmap from './RiskHeatmap'
import { scaleLabels } from './riskHeatmapLabels'

// UX-25: die Achsen der Heatmap zeigten „v a k t c" — die Schlüssel
// likelihoodLabels/impactLabels fehlten, i18next gab den Schlüsselnamen als
// Text zurück, und der Code las daraus einzelne Buchstaben.

describe('RiskHeatmap Achsenbeschriftung (UX-25)', () => {
  it('zeigt echte Stufen auf beiden Achsen', () => {
    renderWithProviders(<RiskHeatmap risks={[]} />)
    for (const label of ['Sehr unwahrscheinlich', 'Unwahrscheinlich', 'Möglich', 'Wahrscheinlich', 'Sehr wahrscheinlich']) {
      expect(screen.getByText(label)).toBeInTheDocument()
    }
    for (const label of ['Vernachlässigbar', 'Gering', 'Spürbar', 'Erheblich', 'Kritisch']) {
      expect(screen.getByText(label)).toBeInTheDocument()
    }
  })

  it('keine Achse zeigt einzelne Buchstaben des Schlüsselnamens', () => {
    renderWithProviders(<RiskHeatmap risks={[]} />)
    for (const letter of ['v', 'a', 'k', 't', 'c']) {
      expect(screen.queryByText(letter, { exact: true })).not.toBeInTheDocument()
    }
  })
})

describe('scaleLabels', () => {
  it('übernimmt ein Array aus fünf Texten', () => {
    expect(scaleLabels(['a', 'b', 'c', 'd', 'e'])).toEqual(['a', 'b', 'c', 'd', 'e'])
  })

  it('fällt bei einem String (fehlender Schlüssel) auf die Zahlen 1–5 zurück', () => {
    expect(scaleLabels('vaktcomply.riskHeatmap.likelihoodLabels')).toEqual(['1', '2', '3', '4', '5'])
  })

  it('fällt bei falscher Länge oder leeren Einträgen zurück', () => {
    expect(scaleLabels(['a', 'b', 'c'])).toEqual(['1', '2', '3', '4', '5'])
    expect(scaleLabels(['a', '', 'c', 'd', 'e'])).toEqual(['1', '2', '3', '4', '5'])
    expect(scaleLabels(undefined)).toEqual(['1', '2', '3', '4', '5'])
  })
})
