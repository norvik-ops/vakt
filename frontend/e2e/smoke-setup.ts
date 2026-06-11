import { test as setup } from '@playwright/test'
import fs from 'fs'
import path from 'path'
import { AUTH_FILE } from '../playwright.config'

setup.setTimeout(120_000)

setup('demo-login', async ({ page, baseURL }) => {
  const base = baseURL ?? 'http://localhost:5173'

  // Retry demo/start up to 3 times — the API may still be starting up
  // after the nightly reset cron (03:00 UTC) when smoke tests fire at ~03:30.
  let res: Response | undefined
  for (let attempt = 1; attempt <= 3; attempt++) {
    res = await fetch(`${base}/api/v1/demo/start`, {
      method: 'POST',
      signal: AbortSignal.timeout(30_000),
    })
    if (res.ok) break
    if (attempt < 3) await new Promise(r => setTimeout(r, 10_000))
  }
  if (!res || !res.ok) throw new Error(`demo/start schlug fehl: ${res?.status}`)
  const { admin_email, admin_password } = await res.json() as {
    admin_email: string
    admin_password: string
  }

  await page.goto('/login', { waitUntil: 'load', timeout: 30_000 })
  await page.locator('#email').fill(admin_email)
  await page.locator('#password').fill(admin_password)
  await page.getByRole('button', { name: /anmelden|sign in/i }).click()
  await page.waitForURL('/', { timeout: 15_000 })

  fs.mkdirSync(path.dirname(AUTH_FILE), { recursive: true })
  await page.context().storageState({ path: AUTH_FILE })
})
