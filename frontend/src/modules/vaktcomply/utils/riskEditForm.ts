import type { Risk, TreatmentStatus, UpdateRiskInput, UpdateRiskTreatmentInput } from '../types'

/**
 * One edit form for the risk detail page (UX-26).
 *
 * The page used to ask strategy, owner, measures and residual risk twice, because
 * ck_risks carries each of them in two columns from two eras:
 *
 *   treatment        ↔ treatment_option     (strategy: avoid | mitigate | transfer | accept)
 *   treatment_notes  ↔ treatment_plan       (measures / treatment plan)
 *   owner            ↔ treatment_owner      (responsible person)
 *   likelihood/impact ↔ inherent_likelihood/inherent_impact (gross risk, S61-4)
 *   residual_* via PATCH /treatment ↔ residual_* via PATCH /residual
 *
 * Readers are split as well (KPI and AI read `treatment`, the CSV export and the
 * heatmap read `treatment_option`), so the form shows each value once and writes it
 * to both columns. Both columns stay; merging them needs a migration.
 *
 * `status` (register status: open | mitigated | accepted | closed) and
 * `treatment_status` (progress of the plan: pending … verified) are NOT duplicates —
 * different value sets, different meaning — and stay two fields.
 */
export interface RiskEditForm {
  title: string
  description: string
  category: string
  likelihood: number
  impact: number
  owner: string
  status: Risk['status']
  strategy: Risk['treatment']
  plan: string
  dueDate: string
  treatmentStatus: TreatmentStatus
  residualLikelihood: number | null
  residualImpact: number | null
}

/** Reads a risk into the form. The newer column wins; the older one fills a gap. */
export function riskToEditForm(risk: Risk): RiskEditForm {
  return {
    title: risk.title,
    description: risk.description ?? '',
    category: risk.category ?? '',
    likelihood: risk.likelihood,
    impact: risk.impact,
    owner: risk.owner || risk.treatment_owner || '',
    status: risk.status,
    strategy: risk.treatment_option || risk.treatment,
    plan: risk.treatment_plan || risk.treatment_notes || '',
    dueDate: risk.treatment_due_date ? risk.treatment_due_date.slice(0, 10) : '',
    treatmentStatus: risk.treatment_status ?? 'pending',
    residualLikelihood: risk.residual_likelihood ?? null,
    residualImpact: risk.residual_impact ?? null,
  }
}

export interface RiskSaveRequests {
  update: UpdateRiskInput
  treatment: UpdateRiskTreatmentInput
  residual: {
    inherent_likelihood: number
    inherent_impact: number
    residual_likelihood?: number
    residual_impact?: number
  }
}

/** Splits the form into the three existing endpoints, each pair of columns written alike. */
export function editFormToRequests(form: RiskEditForm): RiskSaveRequests {
  return {
    update: {
      title: form.title,
      description: form.description,
      category: form.category,
      likelihood: form.likelihood,
      impact: form.impact,
      owner: form.owner,
      status: form.status,
      treatment: form.strategy,
      treatment_notes: form.plan,
    },
    treatment: {
      treatment_option: form.strategy,
      treatment_plan: form.plan,
      treatment_owner: form.owner,
      // An empty date goes as null (backend: nil = keep the stored date). '' would be
      // rejected: the validator's `omitempty` checks only the pointer, so "" fails
      // `datetime` with 422 (Kernweg 8, 2026-11-18). Clearing a date is therefore not
      // possible yet — same as before the merge of the forms (R1-KW-68).
      treatment_due_date: form.dueDate || null,
      treatment_status: form.treatmentStatus,
      residual_likelihood: form.residualLikelihood,
      residual_impact: form.residualImpact,
    },
    residual: {
      inherent_likelihood: form.likelihood,
      inherent_impact: form.impact,
      ...(form.residualLikelihood != null ? { residual_likelihood: form.residualLikelihood } : {}),
      ...(form.residualImpact != null ? { residual_impact: form.residualImpact } : {}),
    },
  }
}

/** Residual score of the form, or null while residual likelihood/impact are not both set. */
export function residualScoreOf(likelihood: number | null | undefined, impact: number | null | undefined): number | null {
  return likelihood != null && impact != null ? likelihood * impact : null
}
