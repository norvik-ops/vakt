import { test, expect } from './fixtures'

function mockStoreAuth(page: Parameters<typeof test>[1]['page']) {
  return page.addInitScript(() => {
    localStorage.setItem(
      'vakt_user',
      JSON.stringify({ id: 'u1', email: 'admin@example.com', role: 'admin', display_name: 'Admin', roles: ['admin'] }),
    )
  })
}

function mockAgentStream(page: Parameters<typeof test>[1]['page']) {
  return page.route('**/api/v1/**', async (route) => {
    const url = route.request().url()
    if (url.includes('/ai/agent')) {
      // Use "arguments" (not "args") to match the AgentEvent interface so that
      // hasDetails is true and the expand/collapse toggle renders.
      const events = [
        'data: {"type":"plan","step":1,"message":"Analysiere Controls…"}\n\n',
        'data: {"type":"tool_call","step":2,"tool":"list_controls","arguments":{"framework":"NIS2"},"result":"OK"}\n\n',
        'data: {"type":"final","step":3,"message":"Ich habe 12 offene Controls gefunden."}\n\n',
        'data: [DONE]\n\n',
      ].join('')
      return route.fulfill({
        status: 200,
        contentType: 'text/event-stream',
        body: events,
      })
    }
    return route.fulfill({ status: 200, contentType: 'application/json', body: '{}' })
  })
}

test.describe('AgentRunPanel', () => {
  test('renders input textarea and start button', async ({ page }) => {
    await mockStoreAuth(page)
    await mockAgentStream(page)
    await page.goto('/secvitals/ai/agent')
    await expect(page.locator('textarea').or(page.locator('[placeholder*="Erstelle"]'))).toBeVisible({ timeout: 8000 })
    await expect(page.locator('button', { hasText: /starten|start|agent/i }).first()).toBeVisible({ timeout: 8000 })
  })

  test('shows plan and tool-call cards after starting agent run', async ({ page }) => {
    await mockStoreAuth(page)
    await mockAgentStream(page)
    await page.goto('/secvitals/ai/agent')

    const textarea = page.locator('textarea').first()
    await textarea.waitFor({ state: 'visible', timeout: 8000 })
    await textarea.fill('Zeige offene NIS2 Controls')

    const startBtn = page.locator('button', { hasText: /starten|start/i }).first()
    await startBtn.click()

    // Expect a plan card or tool-call card to appear.
    await expect(
      page.locator('text=Analysiere').or(page.locator('[data-type="plan"]')).or(page.locator('text=Plan')),
    ).toBeVisible({ timeout: 10000 })
  })

  test('tool_call card expand/collapse toggle works', async ({ page }) => {
    await mockStoreAuth(page)
    await mockAgentStream(page)
    await page.goto('/secvitals/ai/agent')

    const textarea = page.locator('textarea').first()
    await textarea.waitFor({ state: 'visible', timeout: 8000 })
    await textarea.fill('Zeige offene NIS2 Controls')

    const startBtn = page.locator('button', { hasText: /starten|start/i }).first()
    await startBtn.click()

    // Wait for the tool_call card to appear — it has a "JSON" toggle button
    // because arguments is set (hasDetails = true).
    const jsonToggle = page.locator('button', { hasText: /JSON/i }).first()
    await jsonToggle.waitFor({ state: 'visible', timeout: 10000 })

    // Initially collapsed: toggle shows "JSON"
    await expect(jsonToggle).toBeVisible()

    // Expand by clicking the toggle.
    await jsonToggle.click()

    // After expansion the toggle label changes to "einklappen" and the
    // Arguments section becomes visible.
    await expect(
      page.locator('button', { hasText: /einklappen/i }).first()
        .or(page.locator('text=Arguments:')),
    ).toBeVisible({ timeout: 5000 })
  })

  test('stop button is visible while agent is running', async ({ page }) => {
    await mockStoreAuth(page)

    // Use a never-ending stream to keep isRunning=true.
    await page.route('**/api/v1/**', async (route) => {
      const url = route.request().url()
      if (url.includes('/ai/agent')) {
        return route.fulfill({
          status: 200,
          contentType: 'text/event-stream',
          body: 'data: {"type":"plan","step":1,"text":"Running…"}\n\n',
        })
      }
      return route.fulfill({ status: 200, contentType: 'application/json', body: '{}' })
    })

    await page.goto('/secvitals/ai/agent')

    const textarea = page.locator('textarea').first()
    await textarea.waitFor({ state: 'visible', timeout: 8000 })
    await textarea.fill('Test')

    const startBtn = page.locator('button', { hasText: /starten|start/i }).first()
    await startBtn.click()

    await expect(page.locator('button', { hasText: /stopp|stop/i })).toBeVisible({ timeout: 8000 })
  })
})
