import { describe, it, expect } from 'vitest'
import deLocale from '../../../i18n/locales/de.json'
import enLocale from '../../../i18n/locales/en.json'
import frLocale from '../../../i18n/locales/fr.json'
import nlLocale from '../../../i18n/locales/nl.json'

// Sprint-59 i18n contract test for AccessReviewsPage. The page references
// these keys via the `t()` helper; if any key is removed from one locale
// the runtime fallback would silently swap in the key path as visible UI
// text. We pin the contract here so the regression bites in CI, not in
// production.
const requiredPaths = [
  ['vaktcomply', 'accessReviews', 'title'],
  ['vaktcomply', 'accessReviews', 'description'],
  ['vaktcomply', 'accessReviews', 'emptyTitle'],
  ['vaktcomply', 'accessReviews', 'addCampaign'],
  ['vaktcomply', 'accessReviews', 'editCampaign'],
  ['vaktcomply', 'accessReviews', 'fields', 'title'],
  ['vaktcomply', 'accessReviews', 'fields', 'description'],
  ['vaktcomply', 'accessReviews', 'fields', 'reviewerEmail'],
  ['vaktcomply', 'accessReviews', 'fields', 'scope'],
  ['vaktcomply', 'accessReviews', 'fields', 'dueDate'],
  ['vaktcomply', 'accessReviews', 'fields', 'status'],
  ['vaktcomply', 'accessReviews', 'fields', 'user'],
  ['vaktcomply', 'accessReviews', 'fields', 'userEmail'],
  ['vaktcomply', 'accessReviews', 'fields', 'role'],
  ['vaktcomply', 'accessReviews', 'fields', 'decision'],
  ['vaktcomply', 'accessReviews', 'fields', 'comment'],
  ['vaktcomply', 'accessReviews', 'fields', 'actions'],
  ['vaktcomply', 'accessReviews', 'placeholders', 'userEmail'],
  ['vaktcomply', 'accessReviews', 'placeholders', 'role'],
  ['vaktcomply', 'accessReviews', 'placeholders', 'title'],
  ['vaktcomply', 'accessReviews', 'placeholders', 'description'],
  ['vaktcomply', 'accessReviews', 'placeholders', 'reviewerEmail'],
  ['vaktcomply', 'accessReviews', 'placeholders', 'scope'],
  ['vaktcomply', 'accessReviews', 'status', 'draft'],
  ['vaktcomply', 'accessReviews', 'status', 'active'],
  ['vaktcomply', 'accessReviews', 'status', 'completed'],
  ['vaktcomply', 'accessReviews', 'status', 'cancelled'],
  ['vaktcomply', 'accessReviews', 'decision', 'pending'],
  ['vaktcomply', 'accessReviews', 'decision', 'approved'],
  ['vaktcomply', 'accessReviews', 'decision', 'revoked'],
]

const requiredAISystemsPaths = [
  ['vaktcomply', 'aiSystems', 'title'],
  ['vaktcomply', 'aiSystems', 'description'],
  ['vaktcomply', 'aiSystems', 'emptyTitle'],
  ['vaktcomply', 'aiSystems', 'add'],
  ['vaktcomply', 'aiSystems', 'edit'],
  ['vaktcomply', 'aiSystems', 'filterAll'],
  ['vaktcomply', 'aiSystems', 'actions', 'classify'],
  ['vaktcomply', 'aiSystems', 'actions', 'documentation'],
  ['vaktcomply', 'aiSystems', 'fields', 'name'],
  ['vaktcomply', 'aiSystems', 'fields', 'provider'],
  ['vaktcomply', 'aiSystems', 'fields', 'useCase'],
  ['vaktcomply', 'aiSystems', 'fields', 'description'],
  ['vaktcomply', 'aiSystems', 'fields', 'affectedGroups'],
  ['vaktcomply', 'aiSystems', 'fields', 'autonomy'],
  ['vaktcomply', 'aiSystems', 'fields', 'riskClass'],
  ['vaktcomply', 'aiSystems', 'fields', 'status'],
  ['vaktcomply', 'aiSystems', 'fields', 'classification'],
  ['vaktcomply', 'aiSystems', 'fields', 'classifiedBy'],
  ['vaktcomply', 'aiSystems', 'autonomyLevel', 'assistive'],
  ['vaktcomply', 'aiSystems', 'autonomyLevel', 'semiAutonomous'],
  ['vaktcomply', 'aiSystems', 'autonomyLevel', 'fullyAutonomous'],
  ['vaktcomply', 'aiSystems', 'riskClassLevel', 'minimal'],
  ['vaktcomply', 'aiSystems', 'riskClassLevel', 'limited'],
  ['vaktcomply', 'aiSystems', 'riskClassLevel', 'high'],
  ['vaktcomply', 'aiSystems', 'riskClassLevel', 'unacceptable'],
  ['vaktcomply', 'aiSystems', 'riskClassLevel', 'prohibited'],
  ['vaktcomply', 'aiSystems', 'statusLevel', 'classified'],
  ['vaktcomply', 'aiSystems', 'statusLevel', 'approved'],
  ['vaktcomply', 'aiSystems', 'statusLevel', 'compliant'],
  ['vaktcomply', 'aiSystems', 'statusLevel', 'decommissioned'],
]

function get(obj: unknown, path: string[]): string | undefined {
  let cur: unknown = obj
  for (const p of path) {
    if (cur && typeof cur === 'object' && p in (cur as Record<string, unknown>)) {
      cur = (cur as Record<string, unknown>)[p]
    } else {
      return undefined
    }
  }
  return typeof cur === 'string' ? cur : undefined
}

const locales: Record<string, unknown> = {
  de: deLocale,
  en: enLocale,
  fr: frLocale,
  nl: nlLocale,
}

describe('vaktcomply i18n contract — AccessReviewsPage', () => {
  for (const lang of Object.keys(locales)) {
    for (const path of requiredPaths) {
      it(`${lang}: ${path.join('.')} is a non-empty string`, () => {
        const value = get(locales[lang], path)
        expect(value).toBeDefined()
        expect(value).not.toBe('')
      })
    }
  }
})

describe('vaktcomply i18n contract — AISystemsPage', () => {
  for (const lang of Object.keys(locales)) {
    for (const path of requiredAISystemsPaths) {
      it(`${lang}: ${path.join('.')} is a non-empty string`, () => {
        const value = get(locales[lang], path)
        expect(value).toBeDefined()
        expect(value).not.toBe('')
      })
    }
  }
})
