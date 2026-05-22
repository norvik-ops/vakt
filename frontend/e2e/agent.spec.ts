import { test, expect } from './fixtures'

function mockStoreAuth(page: Parameters<typeof test>[1]['page']) {
  return page.addInitScript(() => {
    localStorage.setItem(
      'auth-storage',
      JSON.stringify({
        state: {
          token: 'v2.local.testtoken',
          user: { id: 'u1', email: 'admin@example.com', role: 'admin', org_id: 'org-1', mfa_enabled: false },
        },
        version: 0,
      }),
    )
  })
}

function mockAgentStream(page: Parameters<typeof test>[1]['page']) {
  return page.route('**/api/v1/**', async (route) => {
    const url = route.request().url()
    if (url.includes('/ai/agent')) {
      const events = [
        'data: {"type":"plan","step":1,"text":"Analysiere Controls…"}\n\n',
        'data: {"type":"tool_call","step":2,"tool":"list_controls","args":{"framework":"NIS2"},"result":"OK"}\n\n',
        'data: {"type":"result","step":3,"text":"Ich habe 12 offene Controls gefunden."}\n\n',
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
