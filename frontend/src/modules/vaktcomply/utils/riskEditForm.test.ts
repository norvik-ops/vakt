import { describe, it, expect } from 'vitest'
import { riskToEditForm, editFormToRequests, residualScoreOf } from './riskEditForm'
import type { Risk } from '../types'

// UX-26: die Risiko-Detailseite fragte Strategie, Verantwortlichen, Maßnahmen und
// Restrisiko je zweimal ab — je Angabe zwei Spalten aus zwei Ausbaustufen. Die Seite
// zeigt jetzt jedes Feld einmal und schreibt beide Spalten gleich.

const BASE: Risk = {
  id: 'r-1', org_id: 'o-1', title: 'Ransomware', description: 'd', category: 'Cyber',
  likelihood: 4, impact: 5, risk_score: 20, owner: '', status: 'open', treatment: 'mitigate',
  treatment_notes: '', treatment_option: undefined, treatment_plan: '', treatment_owner: '',
  treatment_due_date: null, treatment_status: 'pending',
  residual_likelihood: null, residual_impact: null,
  created_at: '2026-11-17T10:00:00Z', updated_at: '2026-11-17T10:00:00Z',
}

describe('riskToEditForm — eine Angabe aus zwei Spalten', () => {
  it('die neuere Spalte gewinnt', () => {
    const f = riskToEditForm({
      ...BASE,
      treatment: 'mitigate', treatment_option: 'transfer',
      treatment_notes: 'alt', treatment_plan: 'neu',
      owner: 'Risikoeigner', treatment_owner: 'Andere Person',
    })
    expect(f.strategy).toBe('transfer')
    expect(f.plan).toBe('neu')
    // owner ist das Feld der Grunddaten und Pflicht der Anlage — es gewinnt.
    expect(f.owner).toBe('Risikoeigner')
  })

  it('Altdaten: die ältere Spalte füllt die Lücke', () => {
    const f = riskToEditForm({
      ...BASE,
      treatment: 'accept', treatment_option: undefined,
      treatment_notes: 'Maßnahmen aus dem Anlegen-Dialog', treatment_plan: '',
      owner: '', treatment_owner: 'Nur im Behandlungsplan',
    })
    expect(f.strategy).toBe('accept')
    expect(f.plan).toBe('Maßnahmen aus dem Anlegen-Dialog')
    expect(f.owner).toBe('Nur im Behandlungsplan')
  })

  it('Zieldatum wird auf JJJJ-MM-TT gekürzt, fehlendes als leerer Text', () => {
    expect(riskToEditForm({ ...BASE, treatment_due_date: '2026-12-31T00:00:00Z' }).dueDate).toBe('2026-12-31')
    expect(riskToEditForm(BASE).dueDate).toBe('')
  })
})

describe('editFormToRequests — beide Spalten gleich geschrieben', () => {
  const form = {
    ...riskToEditForm(BASE),
    strategy: 'avoid' as const,
    plan: 'Fileserver abschalten',
    owner: 'CISO',
    residualLikelihood: 2,
    residualImpact: 2,
    dueDate: '',
  }
  const req = editFormToRequests(form)

  it('Strategie, Maßnahmen und Verantwortlicher landen in beiden Spalten', () => {
    expect(req.update.treatment).toBe('avoid')
    expect(req.treatment.treatment_option).toBe('avoid')
    expect(req.update.treatment_notes).toBe('Fileserver abschalten')
    expect(req.treatment.treatment_plan).toBe('Fileserver abschalten')
    expect(req.update.owner).toBe('CISO')
    expect(req.treatment.treatment_owner).toBe('CISO')
  })

  it('Ausgangsbewertung = Bruttorisiko, Restrisiko in beiden Endpunkten gleich', () => {
    expect(req.residual.inherent_likelihood).toBe(req.update.likelihood)
    expect(req.residual.inherent_impact).toBe(req.update.impact)
    expect(req.residual.residual_likelihood).toBe(2)
    expect(req.treatment.residual_likelihood).toBe(2)
    expect(req.residual.residual_impact).toBe(2)
    expect(req.treatment.residual_impact).toBe(2)
  })

  // Kernweg 8 (CI 2026-11-18): "" scheitert an der Validierung (omitempty prüft beim
  // *string nur den Zeiger, "" fällt durch `datetime`) → 422. Leer heißt deshalb null.
  it('ein leeres Zieldatum geht als null, nie als ""', () => {
    expect(req.treatment.treatment_due_date).toBeNull()
    expect(editFormToRequests({ ...form, dueDate: '2026-12-31' }).treatment.treatment_due_date).toBe('2026-12-31')
  })

  it('ohne Restrisiko-Bewertung schickt /residual keine Restwerte', () => {
    const r = editFormToRequests({ ...form, residualLikelihood: null, residualImpact: null })
    expect('residual_likelihood' in r.residual).toBe(false)
    expect('residual_impact' in r.residual).toBe(false)
  })

  it('Registerstatus und Umsetzungsstand bleiben getrennt (verschiedene Bedeutung)', () => {
    const r = editFormToRequests({ ...form, status: 'accepted', treatmentStatus: 'implemented' })
    expect(r.update.status).toBe('accepted')
    expect(r.treatment.treatment_status).toBe('implemented')
  })
})

describe('residualScoreOf', () => {
  it('nur mit beiden Werten', () => {
    expect(residualScoreOf(2, 2)).toBe(4)
    expect(residualScoreOf(2, null)).toBeNull()
    expect(residualScoreOf(undefined, 3)).toBeNull()
  })
})
