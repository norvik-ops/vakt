import { useState } from 'react'

export interface FieldRules {
  required?: boolean
  minLength?: number
  maxLength?: number
  pattern?: RegExp
  patternMessage?: string
}

export function useFormValidation<T extends Record<string, unknown>>(
  fields: Record<keyof T, FieldRules>,
) {
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

      if (value.trim().length === 0) continue

      if (rules.minLength !== undefined && value.length < rules.minLength) {
        newErrors[key] = `Mindestens ${rules.minLength} Zeichen erforderlich.`
        continue
      }

      if (rules.maxLength !== undefined && value.length > rules.maxLength) {
        newErrors[key] = `Maximal ${rules.maxLength} Zeichen erlaubt.`
        continue
      }

      if (rules.pattern !== undefined && !rules.pattern.test(value)) {
        newErrors[key] = rules.patternMessage ?? 'Ungültiges Format.'
        continue
      }
    }

    setErrors(newErrors)
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
