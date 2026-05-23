import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { useFormValidation } from './useFormValidation'

type TestForm = { name: string; email: string; note: string }

const NO_SCROLL = { scrollToError: false }

describe('useFormValidation — required', () => {
  it('fails and sets error when required field is empty', () => {
    const { result } = renderHook(() =>
      useFormValidation<TestForm>({ name: { required: true }, email: {}, note: {} }, NO_SCROLL),
    )
    let valid = true
    act(() => { valid = result.current.validate({ name: '', email: '', note: '' }) })
    expect(valid).toBe(false)
    expect(result.current.errors.name).toBe('Dieses Feld ist erforderlich.')
  })

  it('passes when required field has a non-empty value', () => {
    const { result } = renderHook(() =>
      useFormValidation<TestForm>({ name: { required: true }, email: {}, note: {} }, NO_SCROLL),
    )
    let valid = false
    act(() => { valid = result.current.validate({ name: 'Acme GmbH', email: '', note: '' }) })
    expect(valid).toBe(true)
    expect(result.current.errors.name).toBeUndefined()
  })

  it('trims whitespace — blank string fails required check', () => {
    const { result } = renderHook(() =>
      useFormValidation<TestForm>({ name: { required: true }, email: {}, note: {} }, NO_SCROLL),
    )
    let valid = true
    act(() => { valid = result.current.validate({ name: '   ', email: '', note: '' }) })
    expect(valid).toBe(false)
  })
})

describe('useFormValidation — minLength', () => {
  it('fails when value is shorter than minLength', () => {
    const { result } = renderHook(() =>
      useFormValidation<TestForm>({ name: { minLength: 3 }, email: {}, note: {} }, NO_SCROLL),
    )
    let valid = true
    act(() => { valid = result.current.validate({ name: 'AB', email: '', note: '' }) })
    expect(valid).toBe(false)
    expect(result.current.errors.name).toMatch(/3/)
  })

  it('passes when value meets minLength exactly', () => {
    const { result } = renderHook(() =>
      useFormValidation<TestForm>({ name: { minLength: 3 }, email: {}, note: {} }, NO_SCROLL),
    )
    let valid = false
    act(() => { valid = result.current.validate({ name: 'ABC', email: '', note: '' }) })
    expect(valid).toBe(true)
  })
})

describe('useFormValidation — maxLength', () => {
  it('fails when value exceeds maxLength', () => {
    const { result } = renderHook(() =>
      useFormValidation<TestForm>({ name: { maxLength: 5 }, email: {}, note: {} }, NO_SCROLL),
    )
    let valid = true
    act(() => { valid = result.current.validate({ name: 'ABCDEF', email: '', note: '' }) })
    expect(valid).toBe(false)
    expect(result.current.errors.name).toMatch(/5/)
  })

  it('passes when value is exactly at maxLength', () => {
    const { result } = renderHook(() =>
      useFormValidation<TestForm>({ name: { maxLength: 5 }, email: {}, note: {} }, NO_SCROLL),
    )
    let valid = false
    act(() => { valid = result.current.validate({ name: 'ABCDE', email: '', note: '' }) })
    expect(valid).toBe(true)
  })
})

describe('useFormValidation — pattern', () => {
  const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

  it('fails with default message when pattern does not match', () => {
    const { result } = renderHook(() =>
      useFormValidation<TestForm>({ name: {}, email: { pattern: EMAIL_PATTERN }, note: {} }, NO_SCROLL),
    )
    let valid = true
    act(() => { valid = result.current.validate({ name: '', email: 'not-an-email', note: '' }) })
    expect(valid).toBe(false)
    expect(result.current.errors.email).toBe('Ungültiges Format.')
  })

  it('uses patternMessage when provided', () => {
    const { result } = renderHook(() =>
      useFormValidation<TestForm>(
        { name: {}, email: { pattern: EMAIL_PATTERN, patternMessage: 'Keine gültige E-Mail-Adresse.' }, note: {} },
        NO_SCROLL,
      ),
    )
    let valid = true
    act(() => { valid = result.current.validate({ name: '', email: 'bad', note: '' }) })
    expect(valid).toBe(false)
    expect(result.current.errors.email).toBe('Keine gültige E-Mail-Adresse.')
  })

  it('passes when value matches pattern', () => {
    const { result } = renderHook(() =>
      useFormValidation<TestForm>({ name: {}, email: { pattern: EMAIL_PATTERN }, note: {} }, NO_SCROLL),
    )
    let valid = false
    act(() => { valid = result.current.validate({ name: '', email: 'admin@example.com', note: '' }) })
    expect(valid).toBe(true)
    expect(result.current.errors.email).toBeUndefined()
  })
})

describe('useFormValidation — custom cross-field validation', () => {
  type RiskForm = { risk_level: string; mitigation: string }

  it('fails when custom validator returns an error string', () => {
    const { result } = renderHook(() =>
      useFormValidation<RiskForm>({
        risk_level: {},
        mitigation: {
          custom: (values) =>
            values.risk_level === 'high' && !values.mitigation
              ? 'Pflichtfeld bei hohem Risiko.'
              : null,
        },
      }, NO_SCROLL),
    )
    let valid = true
    act(() => { valid = result.current.validate({ risk_level: 'high', mitigation: '' }) })
    expect(valid).toBe(false)
    expect(result.current.errors.mitigation).toBe('Pflichtfeld bei hohem Risiko.')
  })

  it('passes when custom validator returns null', () => {
    const { result } = renderHook(() =>
      useFormValidation<RiskForm>({
        risk_level: {},
        mitigation: {
          custom: (values) =>
            values.risk_level === 'high' && !values.mitigation
              ? 'Pflichtfeld bei hohem Risiko.'
              : null,
        },
      }, NO_SCROLL),
    )
    let valid = false
    act(() => { valid = result.current.validate({ risk_level: 'low', mitigation: '' }) })
    expect(valid).toBe(true)
  })
})

describe('useFormValidation — clearError', () => {
  it('removes a single field error without affecting other fields', () => {
    const { result } = renderHook(() =>
      useFormValidation<TestForm>(
        { name: { required: true }, email: { required: true }, note: {} },
        NO_SCROLL,
      ),
    )
    act(() => { result.current.validate({ name: '', email: '', note: '' }) })
    expect(result.current.errors.name).toBeDefined()
    expect(result.current.errors.email).toBeDefined()

    act(() => { result.current.clearError('name') })
    expect(result.current.errors.name).toBeUndefined()
    expect(result.current.errors.email).toBeDefined()
  })
})

describe('useFormValidation — clearAll', () => {
  it('removes all errors at once', () => {
    const { result } = renderHook(() =>
      useFormValidation<TestForm>(
        { name: { required: true }, email: { required: true }, note: {} },
        NO_SCROLL,
      ),
    )
    act(() => { result.current.validate({ name: '', email: '', note: '' }) })
    expect(Object.keys(result.current.errors).length).toBeGreaterThan(0)

    act(() => { result.current.clearAll() })
    expect(result.current.errors).toEqual({})
  })
})

describe('useFormValidation — scrollToError', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    Element.prototype.scrollIntoView = vi.fn()
  })
  afterEach(() => {
    vi.restoreAllMocks()
    vi.useRealTimers()
  })

  it('queries and scrolls to the first error field by default', () => {
    const el = document.createElement('div')
    const input = document.createElement('input')
    el.appendChild(input)
    const spy = vi.spyOn(document, 'querySelector').mockReturnValue(el)

    const { result } = renderHook(() =>
      useFormValidation<TestForm>({ name: { required: true }, email: {}, note: {} }),
    )
    act(() => { result.current.validate({ name: '', email: '', note: '' }) })
    act(() => { vi.runAllTimers() })

    expect(spy).toHaveBeenCalledWith('[data-field="name"]')
    expect(el.scrollIntoView).toHaveBeenCalled()
  })

  it('does not scroll when scrollToError is false', () => {
    const spy = vi.spyOn(document, 'querySelector')

    const { result } = renderHook(() =>
      useFormValidation<TestForm>({ name: { required: true }, email: {}, note: {} }, NO_SCROLL),
    )
    act(() => { result.current.validate({ name: '', email: '', note: '' }) })
    act(() => { vi.runAllTimers() })

    expect(spy).not.toHaveBeenCalled()
  })

  it('does not scroll when validation passes', () => {
    const spy = vi.spyOn(document, 'querySelector')

    const { result } = renderHook(() =>
      useFormValidation<TestForm>({ name: { required: true }, email: {}, note: {} }),
    )
    act(() => { result.current.validate({ name: 'valid', email: '', note: '' }) })
    act(() => { vi.runAllTimers() })

    expect(spy).not.toHaveBeenCalled()
  })
})
