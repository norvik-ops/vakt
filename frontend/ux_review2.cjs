const { chromium } = require('playwright');
const { mkdirSync } = require('fs');

const BASE = 'http://localhost:5173';
const SHOTS = '/tmp/ux_shots';
mkdirSync(SHOTS, { recursive: true });

async function run() {
  const browser = await chromium.launch({ 
    headless: true, 
    executablePath: '/home/stefan/.cache/ms-playwright/chromium-1223/chrome-linux64/chrome' 
  });
  const ctx = await browser.newContext({ viewport: { width: 1280, height: 900 } });
  const page = await ctx.newPage();

  const shot = async (name) => {
    await page.screenshot({ path: `${SHOTS}/${name}.png`, fullPage: false });
    console.log(`[shot] ${name}`);
  };

  // 1. Login page - inspect the DOM
  await page.goto(BASE);
  await page.waitForLoadState('networkidle');
  await shot('01_login');
  
  const dom = await page.evaluate(() => document.body.innerHTML);
  const forms = await page.evaluate(() => {
    const inputs = Array.from(document.querySelectorAll('input'));
    const buttons = Array.from(document.querySelectorAll('button'));
    return {
      inputs: inputs.map(i => ({ type: i.type, name: i.name, id: i.id, placeholder: i.placeholder })),
      buttons: buttons.map(b => ({ type: b.type, text: b.textContent?.trim(), id: b.id })),
    };
  });
  console.log('[form]', JSON.stringify(forms));
  
  await browser.close();
}

run().catch(e => { console.error('[FATAL]', e.message); process.exit(1); });
