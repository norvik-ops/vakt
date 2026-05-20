import { useState } from 'react'

export interface FieldRules<T = unknown> {
  required?: boolean
  minLength?: number
  maxLength?: number
  pattern?: RegExp
  patternMessage?: string
  /**
   * Cross-field or custom validation. Receives all form values and the value
   * of the current field. Return a non-empty string to fail validation with
   * that message, or null/undefined to pass.
   *
   * Example (DPIA: high-risk impact requires DPIA-required = true):
   *
   *   impact_high: { custom: (vals) => vals.dpia_required ? null : 'DPIA erforderlich' }
   */
  custom?: (values: T, value: unknown) => string | null | undefined
}

export interface FormValidationOptions {
  /**
   * When true (default), the form scrolls to the first field with an error
   * after a failed validate() call. Disable for inline validators that don't
   * own a visible form (e.g. validating before opening a dialog).
   */
  scrollToError?: boolean
}

/**
 * Read why we don't use react-hook-form here:
 *
 * The Vakt frontend has ~89 custom hooks and consistent useState-driven forms
 * across all modules. A full react-hook-form migration would mean rewriting
 * every form — high risk for a quality-of-life improvement. Instead we extend
 * this hook to cover the gaps the external review flagged:
 *
 *   • cross-field validation     — added via the `custom` callback (this file)
 *   • scroll-to-first-error      — added below; opt-in via FormValidationOptions
 *   • dirty-state warning        — already provided by useUnsavedChanges
 *   • autosave                   — out of scope: most Vakt forms are
 *                                  user-explicit "Create" / "Update" actions
 *
 * If a future form is sufficiently complex (DPIA with conditional sections,
 * 20+ fields, deep nesting) that this hook becomes painful, that specific
 * form can adopt react-hook-form locally without affecting the rest.
 */
export function useFormValidation<T extends Record<string, unknown>>(
  fields: Record<keyof T, FieldRules<T>>,
  options: FormValidationOptions = {},
) {
  const { scrollToError = true } = options
  const [errors, setErrors] = useState<Partial<Record<keyof T, string>>>({})

  const validate = (values: T): boolean => {
    const newErrors: Partial<Record<keyof T, string>> = {}

    for (const key of Object.keys(fields) as Array<keyof T>) {
      const rules = fields[key]
      const raw = values[key]
      const value = typeof raw === 'string' ? raw : raw == null ? '' : String(raw)

      if (rules.required && value.trim().length === 0) {
        newErrors[key] = 'Dieses Feld ist erforderlich.'
        continue
      }

      if (value.trim().length > 0) {
        if (rules.minLength !== undefined && value.length < rules.minLength) {
          newErrors[key] = `Mindestens ${rules.minLength.toString()} Zeichen erforderlich.`
          continue
        }
        if (rules.maxLength !== undefined && value.length > rules.maxLength) {
          newErrors[key] = `Maximal ${rules.maxLength.toString()} Zeichen erlaubt.`
          continue
        }
        if (rules.pattern !== undefined && !rules.pattern.test(value)) {
          newErrors[key] = rules.patternMessage ?? 'Ungültiges Format.'
          continue
        }
      }

      if (rules.custom) {
        const customError = rules.custom(values, raw)
        if (customError) {
          newErrors[key] = customError
        }
      }
    }

    setErrors(newErrors)

    // Scroll-to-first-error: improves UX on long forms where the failing field
    // may be off-screen. Looks for [data-field="<name>"] anchor on the page.
    if (scrollToError && Object.keys(newErrors).length > 0) {
      const firstErrorField = Object.keys(newErrors)[0]
      // setTimeout pushes the scroll past React's commit phase so the error
      // styling is applied before the scroll lands on it.
      setTimeout(() => {
        const el = document.querySelector<HTMLElement>(`[data-field="${firstErrorField}"]`)
        if (el) {
          el.scrollIntoView({ behavior: 'smooth', block: 'center' })
          // Try to focus the input inside if possible.
          const focusable = el.querySelector<HTMLElement>('input, textarea, select')
          focusable?.focus({ preventScroll: true })
        }
      }, 0)
    }

    return Object.keys(newErrors).length === 0
  }

  const clearError = (field: keyof T) => {
    setErrors((prev) => {
      const next = { ...prev }
      delete next[field]
      return next
    })
  }

  const clearAll = () => setErrors({})

  return { errors, validate, clearError, clearAll }
}
