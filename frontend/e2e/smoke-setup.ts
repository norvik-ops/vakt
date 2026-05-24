import { test as setup } from '@playwright/test'
import fs from 'fs'
import path from 'path'
import { AUTH_FILE } from '../playwright.config'

setup('demo-login', async ({ page, baseURL }) => {
  const base = baseURL ?? 'http://localhost:5173'

  const res = await fetch(`${base}/api/v1/demo/start`, { method: 'POST' })
  if (!res.ok) throw new Error(`demo/start schlug fehl: ${res.status}`)
  const { admin_email, admin_password } = await res.json() as {
    admin_email: string
    admin_password: string
  }

  await page.goto('/login', { waitUntil: 'domcontentloaded', timeout: 30_000 })

  // Diagnostik: was sieht Playwright wirklich?
  console.log('URL nach goto:', page.url())
  console.log('Title:', await page.title())
  const html = await page.content()
  console.log('Hat #email:', html.includes('id="email"'))
  console.log('Hat id="root":', html.includes('id="root"'))
  console.log('HTML-Snippet (erste 600 Zeichen):', html.slice(0, 600))

  // Screenshot für Artifact
  const screenshotDir = path.join(path.dirname(AUTH_FILE), 'screenshots')
  fs.mkdirSync(screenshotDir, { recursive: true })
  await page.screenshot({ path: path.join(screenshotDir, 'smoke-login.png'), fullPage: true })

  // SW-autoUpdate kann einen Page-Reload triggern — kurz auf Networkidle warten
  await page.waitForLoadState('networkidle', { timeout: 15_000 }).catch(() => {
    console.log('networkidle-Timeout — fahre trotzdem fort')
  })

  await page.locator('#email').fill(admin_email)
  await page.locator('#password').fill(admin_password)
  await page.getByRole('button', { name: /anmelden|sign in/i }).click()
  await page.waitForURL('/', { timeout: 15_000 })

  fs.mkdirSync(path.dirname(AUTH_FILE), { recursive: true })
  await page.context().storageState({ path: AUTH_FILE })
})
