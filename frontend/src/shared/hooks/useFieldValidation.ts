import { useState, useEffect, useRef } from 'react'
import i18next from 'i18next'

export interface ValidationRule {
  test: (v: string) => boolean
  message: string
}

export interface FieldValidationResult {
  error: string | null
  isValid: boolean
}

export function useFieldValidation(
  value: string,
  rules: ValidationRule[],
): FieldValidationResult {
  const [error, setError] = useState<string | null>(null)
  const timerRef = useRef<ReturnType<typeof setTimeout>>()

  useEffect(() => {
    clearTimeout(timerRef.current)
    timerRef.current = setTimeout(() => {
      if (value === '') {
        setError(null)
        return
      }
      for (const rule of rules) {
        if (!rule.test(value)) {
          setError(rule.message)
          return
        }
      }
      setError(null)
    }, 300)

    return () => { clearTimeout(timerRef.current); }
  }, [value, rules])

  return { error, isValid: error === null && value.length > 0 }
}

// ─── Pre-built rules ──────────────────────────────────────────────────────────

export const required: ValidationRule = {
  test: (v) => v.trim().length > 0,
  get message() { return i18next.t('validation.required') },
}

export function minLength(n: number): ValidationRule {
  return {
    test: (v) => v.length >= n,
    get message() { return i18next.t('validation.minLength', { count: n }) },
  }
}

export const email: ValidationRule = {
  test: (v) => /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(v),
  get message() { return i18next.t('validation.email') },
}

export const passwordStrength: ValidationRule = {
  test: (v) =>
    v.length >= 10 &&
    /[A-Z]/.test(v) &&
    /[0-9]/.test(v) &&
    /[^A-Za-z0-9]/.test(v),
  get message() { return i18next.t('validation.passwordStrength') },
}

// ─── Password strength score (0-4) ───────────────────────────────────────────

export function getPasswordStrengthScore(password: string): number {
  if (!password) return 0
  let score = 0
  if (password.length >= 10) score++
  if (/[A-Z]/.test(password)) score++
  if (/[0-9]/.test(password)) score++
  if (/[^A-Za-z0-9]/.test(password)) score++
  return score
}
