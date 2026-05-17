// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { apiFetch } from '../../api/client'
import { useAuthStore } from '../stores/auth'
import { VAKT_LS_PORTAL_URL } from '../../lib/constants'

interface LicenseInfo {
  tier: string
  is_pro: boolean
  features: string[]
  org_name: string
  expires_at: string | null
  demo: boolean
  revoked: boolean
}

function useLicenseInfo() {
  return useQuery<LicenseInfo>({
    queryKey: ['license'],
    queryFn: () => apiFetch<LicenseInfo>('/license'),
    staleTime: 5 * 60 * 1000, // 5 minutes
    retry: false,
  })
}

function daysUntilExpiry(expiresAt: string): number {
  return Math.floor((new Date(expiresAt).getTime() - Date.now()) / 86400000)
}

/** localStorage key includes today's date so the banner reappears each new day. */
function dismissalKey(expiresAt: string): string {
  const today = new Date().toISOString().slice(0, 10)
  return `vakt_license_warning_dismissed_${today}`
}

function isAlreadyDismissed(expiresAt: string): boolean {
  return localStorage.getItem(dismissalKey(expiresAt)) === '1'
}

function persistDismissal(expiresAt: string) {
  localStorage.setItem(dismissalKey(expiresAt), '1')
}

export function LicenseExpiryBanner() {
  const { user } = useAuthStore()
  const { data: lic } = useLicenseInfo()
  const [dismissed, setDismissed] = useState(false)

  const isAdmin = user?.roles?.includes('admin') || user?.roles?.includes('owner')

  // Only admins, only Pro licenses with an expiry, not already dismissed this session/day
  if (!isAdmin || !lic?.is_pro || !lic.expires_at) {
    return null
  }

  const expiresAt = lic.expires_at
  const days = daysUntilExpiry(expiresAt)

  // Nothing to show if more than 30 days remain
  if (days > 30) {
    return null
  }

  // Dismissed in this render cycle or already stored in localStorage
  if (dismissed || isAlreadyDismissed(expiresAt)) {
    return null
  }

  const formattedDate = new Date(expiresAt).toLocaleDateString('de-DE')
  const isExpired = days < 0
  const isUrgent = days <= 7 // includes expired

  function handleDismiss() {
    persistDismissal(expiresAt)
    setDismissed(true)
  }

  if (isUrgent || isExpired) {
    return (
      <div className="bg-red-50 dark:bg-red-950/30 border-b border-red-200 dark:border-red-800 px-4 py-2 flex items-center justify-between text-sm shrink-0">
        <span className="text-red-800 dark:text-red-300">
          {isExpired
            ? 'Deine Pro-Lizenz ist abgelaufen. Features wurden deaktiviert.'
            : `Deine Pro-Lizenz läuft am ${formattedDate} ab — noch ${days} Tag${days === 1 ? '' : 'e'}.`}
          {' '}
          <a
            href={VAKT_LS_PORTAL_URL}
            target="_blank"
            rel="noopener noreferrer"
            className="underline font-medium hover:text-red-900 dark:hover:text-red-200"
          >
            Jetzt verlängern →
          </a>
        </span>
        <button
          onClick={handleDismiss}
          aria-label="Schließen"
          className="text-red-600 dark:text-red-400 hover:text-red-800 dark:hover:text-red-200 ml-4"
        >
          ✕
        </button>
      </div>
    )
  }

  // Yellow warning: 8–30 days remaining
  return (
    <div className="bg-amber-50 dark:bg-amber-950/30 border-b border-amber-200 dark:border-amber-800 px-4 py-2 flex items-center justify-between text-sm shrink-0">
      <span className="text-amber-800 dark:text-amber-300">
        Deine Pro-Lizenz läuft am {formattedDate} ab.{' '}
        <a
          href={VAKT_LS_PORTAL_URL}
          target="_blank"
          rel="noopener noreferrer"
          className="underline font-medium hover:text-amber-900 dark:hover:text-amber-200"
        >
          Jetzt verlängern →
        </a>
      </span>
      <button
        onClick={handleDismiss}
        aria-label="Schließen"
        className="text-amber-600 dark:text-amber-400 hover:text-amber-800 dark:hover:text-amber-200 ml-4"
      >
        ✕
      </button>
    </div>
  )
}
