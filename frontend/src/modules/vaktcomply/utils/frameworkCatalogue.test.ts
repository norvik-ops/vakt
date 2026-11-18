import { describe, it, expect } from 'vitest'
import { FRAMEWORK_CATALOGUE, frameworkDisplayName } from './frameworkCatalogue'

// Das Backend führt Frameworks unter ihrem Schlüssel („ISO27001", „EUAIACT").
// Die Oberfläche zeigt den Anzeigenamen aus dem Katalog.
describe('frameworkDisplayName', () => {
  it.each(FRAMEWORK_CATALOGUE.map((fw) => [fw.key, fw.name]))(
    'Katalog-Schlüssel %s ergibt den Anzeigenamen %s',
    (key, name) => {
      expect(frameworkDisplayName(key)).toBe(name)
      expect(name.trim()).not.toBe('')
    },
  )

  it('übersetzt die Schlüssel, die der Durchgang roh gesehen hat', () => {
    expect(frameworkDisplayName('ISO27001')).toBe('ISO 27001')
    expect(frameworkDisplayName('EUAIACT')).toBe('EU AI Act')
    expect(frameworkDisplayName('NIS2')).toBe('NIS2')
    expect(frameworkDisplayName('BSI')).toBe('BSI IT-Grundschutz')
  })

  it('erkennt Schlüssel unabhängig von Schreibweise und Trennzeichen', () => {
    // Der Multi-Framework-Assistent liefert kleingeschriebene Schlüssel.
    expect(frameworkDisplayName('iso27001')).toBe('ISO 27001')
    expect(frameworkDisplayName('dsgvo_tom')).toBe(frameworkDisplayName('DSGVO-TOM'))
  })

  it('lässt einen bereits lesbaren Namen stehen', () => {
    expect(frameworkDisplayName('ISO 27001')).toBe('ISO 27001')
  })

  it('zeigt einen unbekannten Namen (eigenes Plugin-Framework) unverändert an', () => {
    expect(frameworkDisplayName('Mein Hausstandard')).toBe('Mein Hausstandard')
  })

  it('liefert für fehlende Werte einen leeren Text statt „undefined"', () => {
    expect(frameworkDisplayName(undefined)).toBe('')
    expect(frameworkDisplayName(null)).toBe('')
    expect(frameworkDisplayName('')).toBe('')
  })

  it('hat keine doppelten Schlüssel im Katalog', () => {
    const keys = FRAMEWORK_CATALOGUE.map((fw) => fw.key.toLowerCase().replace(/[^a-z0-9]/g, ''))
    expect(new Set(keys).size).toBe(keys.length)
  })
})
