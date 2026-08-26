import { describe, it, expect } from 'vitest'
import {
  findingSeverityClass,
  findingSeverityOrder,
  findingSeverityVariant,
  findingSeverityWeight,
  type FindingSeverity,
} from './statusMapping'

// R1-W6C-N1 (Oberflächenteil): `unknown` ist seit Migration 265 ein eigener
// Schweregrad und heißt „nicht bewertet", nicht „harmlos". Diese Tests halten die
// beiden Entscheidungen fest, die man sonst beim nächsten Aufräumen versehentlich
// zurückdreht — die Sortierposition und die eigenständige Darstellung.

describe('Sortierposition von „unbewertet"', () => {
  it('steht unter critical und über high', () => {
    // Die eigentliche Entscheidung. Begründung ausführlich in statusMapping.ts:
    // Ein unbewerteter Fund KANN kritisch sein, gehört also vor alles, von dem
    // wir WISSEN, dass es nur hoch ist — aber hinter einen bestätigt kritischen,
    // weil Gewissheit bei knapper Zeit den Ausschlag gibt.
    expect(findingSeverityOrder.unknown).toBeLessThan(findingSeverityOrder.critical)
    expect(findingSeverityOrder.unknown).toBeGreaterThan(findingSeverityOrder.high)
  })

  it('steht NICHT am Listenende', () => {
    // Die bequeme Antwort wäre gewesen, ihn ganz unten einzusortieren — und
    // genau das war über den alten `|| 0`-Fallback das faktische Verhalten. In
    // einer langen Liste sieht ihn dort niemand, und die Sicherheitsfolge, die
    // das Backend beseitigt hat, wäre über die Sortierung wieder da.
    const gewichte = Object.values(findingSeverityOrder)
    expect(findingSeverityOrder.unknown).toBeGreaterThan(Math.min(...gewichte))
    expect(findingSeverityOrder.unknown).toBeGreaterThan(findingSeverityOrder.info)
    expect(findingSeverityOrder.unknown).toBeGreaterThan(findingSeverityOrder.low)
  })

  it('sortiert eine gemischte Liste in der erwarteten Reihenfolge', () => {
    const liste: FindingSeverity[] = ['info', 'critical', 'low', 'unknown', 'high', 'medium']
    const sortiert = [...liste].sort((a, b) => findingSeverityOrder[b] - findingSeverityOrder[a])
    expect(sortiert).toEqual(['critical', 'unknown', 'high', 'medium', 'low', 'info'])
  })
})

describe('findingSeverityWeight', () => {
  it('gibt für bekannte Grade das Tabellengewicht zurück', () => {
    expect(findingSeverityWeight('critical')).toBe(findingSeverityOrder.critical)
    expect(findingSeverityWeight('info')).toBe(findingSeverityOrder.info)
  })

  it('behandelt einen unbekannten Grad wie „unbewertet", nicht wie 0', () => {
    // Ein Grad, den die Tabelle nicht kennt, IST ein unbewerteter Fund.
    expect(findingSeverityWeight('catastrophic')).toBe(findingSeverityOrder.unknown)
    expect(findingSeverityWeight('')).toBe(findingSeverityOrder.unknown)
    expect(findingSeverityWeight('catastrophic')).not.toBe(0)
  })

  it('fällt nicht auf Prototyp-Schlüssel herein', () => {
    // Mit `in` statt hasOwnProperty wäre `toString` ein Treffer gewesen und die
    // Funktion hätte eine Funktion statt einer Zahl zurückgegeben.
    expect(findingSeverityWeight('toString')).toBe(findingSeverityOrder.unknown)
    expect(typeof findingSeverityWeight('toString')).toBe('number')
  })
})

describe('Darstellung von „unbewertet"', () => {
  it('sieht nicht aus wie `info`', () => {
    // Sonst wäre die Verwechslung, die das Backend gerade verhindert, in der
    // Oberfläche wieder da: „nicht bewertet" darf nicht aussehen wie „geprüft
    // und harmlos".
    expect(findingSeverityClass.unknown).not.toBe(findingSeverityClass.info)
    expect(findingSeverityVariant.unknown).not.toBe(findingSeverityVariant.info)
  })

  it('ist von jedem anderen Schweregrad unterscheidbar', () => {
    const andere = (['info', 'low', 'medium', 'high', 'critical'] as const)
      .map((s) => findingSeverityClass[s])
    expect(andere).not.toContain(findingSeverityClass.unknown)
  })

  it('trägt keine Schweregrad-Farbe, sondern eine gestrichelte Umrandung', () => {
    // Eine Farbe aus der Schweregrad-Skala wäre eine Einstufung — und genau die
    // fehlt ja. Die gestrichelte Umrandung liest sich als „noch offen".
    expect(findingSeverityClass.unknown).toContain('border-dashed')
    expect(findingSeverityClass.unknown).not.toContain('bg-severity-')
  })
})

describe('Vollständigkeit der Tabellen', () => {
  it('jede Tabelle kennt jeden Grad', () => {
    // Fehlt ein Eintrag, rendert das Chip unstyled bzw. sortiert falsch — ohne
    // dass irgendetwas fehlschlägt.
    const grade = Object.keys(findingSeverityOrder) as FindingSeverity[]
    expect(grade).toContain('unknown')
    for (const g of grade) {
      expect(findingSeverityClass[g], `Klasse für ${g}`).toBeTruthy()
      expect(findingSeverityVariant[g], `Variante für ${g}`).toBeTruthy()
    }
  })
})
