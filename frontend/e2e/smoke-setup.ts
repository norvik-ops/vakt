import { test as setup } from '@playwright/test'
import fs from 'fs'
import path from 'path'
import { AUTH_FILE } from '../playwright.config'

setup.setTimeout(60_000)

setup('demo-login', async ({ page, baseURL }) => {
  const base = baseURL ?? 'http://localhost:5173'

  const res = await fetch(`${base}/api/v1/demo/start`, { method: 'POST' })
  if (!res.ok) throw new Error(`demo/start schlug fehl: ${res.status}`)
  const { admin_email, admin_password } = await res.json() as {
    admin_email: string
    admin_password: string
  }

  // waitUntil:'load' statt 'domcontentloaded' — erst nach 'load' haben alle
  // Scripts ausgeführt und React hat synchron gerendert (#email im DOM).
  await page.goto('/login', { waitUntil: 'load', timeout: 30_000 })

  console.log('URL nach goto:', page.url())
  const html = await page.content()
  console.log('Hat #email nach load:', html.includes('id="email"'))
  console.log('Hat id="root":', html.includes('id="root"'))

  await page.locator('#email').fill(admin_email)
  await page.locator('#password').fill(admin_password)
  await page.getByRole('button', { name: /anmelden|sign in/i }).click()
  await page.waitForURL('/', { timeout: 15_000 })

  fs.mkdirSync(path.dirname(AUTH_FILE), { recursive: true })
  await page.context().storageState({ path: AUTH_FILE })
})
