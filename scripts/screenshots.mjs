// README screenshots and a browser smoke test against a demo instance:
//
//   make build && DATA_DIR=/tmp/capsule-demo DEMO_DATA=true AUTH_MODE=none HOST=127.0.0.1 PORT=18268 ./pocket-capsule &
//   CDP=http://127.0.0.1:9338 BASE=http://127.0.0.1:18268 node scripts/screenshots.mjs   # any Chrome with remote debugging
//   SHOTS=0 … node scripts/screenshots.mjs                                                # checks only
// playwright-core comes from web/node_modules, or set PW_FROM to a package.json next to an install of it.
import { createRequire } from 'node:module'
import assert from 'node:assert/strict'

const require = createRequire(process.env.PW_FROM || new URL('../web/package.json', import.meta.url))
let pw
try { pw = require('playwright') } catch { pw = require('playwright-core') }
const { chromium } = pw
const base = process.env.BASE || 'http://127.0.0.1:18268'
const shots = process.env.SHOTS !== '0'
const only = process.env.ONLY || ''
const dir = process.env.OUT || new URL('../docs/screenshots/', import.meta.url).pathname
const browser = process.env.CDP ? await chromium.connectOverCDP(process.env.CDP) : await chromium.launch()
const errors = []

async function session({ phone = false, dark = false } = {}) {
  const ctx = await browser.newContext({
    viewport: phone ? { width: 390, height: 844 } : { width: 1440, height: 900 },
    deviceScaleFactor: phone ? 3 : 2, isMobile: phone, hasTouch: phone, colorScheme: dark ? 'dark' : 'light',
  })
  const page = await ctx.newPage()
  page.on('pageerror', e => errors.push(`pageerror: ${e}`))
  page.on('console', m => { if (m.type() === 'error' && !/Failed to load resource/.test(m.text())) errors.push(m.text()) })
  return { ctx, page }
}
async function go(page, path, wait = 1200) {
  await page.goto(base + path, { waitUntil: 'domcontentloaded' })
  await page.addStyleTag({ content: '*, *::before, *::after { transition: none !important; animation: none !important; }' }).catch(() => {})
  await page.waitForTimeout(wait)
}
async function shot(page, name, full = false) { if (shots && (!only || only.split(',').includes(name))) await page.screenshot({ path: dir + name + '.png', fullPage: full }) }
const api = (page, path) => page.evaluate(p => fetch('/api/v1' + p).then(r => r.json()), path)
const idOf = async (page, title) => (await api(page, '/capsules')).capsules.find(r => r.capsule.title.startsWith(title)).capsule.id

{
  const { ctx, page } = await session()
  await go(page, '/', 2000)
  assert.ok((await page.content()).includes('is ready to open'), 'the ready capsule shows on the home page')
  await shot(page, 'desktop-home')
  const mila = await idOf(page, 'For Mila')
  await go(page, `/c/${mila}`)
  assert.ok((await page.content()).includes('Sealed'), 'a sealed capsule shows its countdown')
  assert.ok(!(await page.content()).includes('furious and perfect'), 'nothing inside a sealed capsule shows')
  await shot(page, 'desktop-sealed')
  const fam = await idOf(page, 'Family capsule')
  await go(page, `/c/${fam}`)
  await shot(page, 'desktop-filling')
  const ny = await idOf(page, 'New Year')
  await go(page, `/c/${ny}`)
  await shot(page, 'desktop-opened')
  await go(page, '/predictions')
  await shot(page, 'desktop-predictions')

  // make one, write a letter, seal it
  await go(page, '/new')
  await page.click('text=Letter to future me')
  await page.waitForTimeout(400)
  await page.fill('input[placeholder="Letter to future me"]', 'Smoke test capsule')
  await shot(page, 'desktop-new')
  await page.click('text=Make the capsule')
  await page.waitForURL(/\/c\/\d+$/)
  await page.waitForTimeout(800)
  await page.click('.kind-btn:has-text("Letter")')
  await page.fill('.letter-area', 'Dear future me, this is a test.')
  await page.click('.modal-foot >> text=Put it in')
  await page.waitForTimeout(800)
  assert.ok((await page.content()).includes('this is a test'), 'the letter is in')
  await page.click('.cap-status >> text=Seal it')
  await page.click('.modal-foot >> text=Seal it')
  await page.waitForTimeout(1200)
  assert.ok(!(await page.content()).includes('this is a test'), 'sealed: the letter is hidden')

  // the opening ceremony of the one that is ready
  const ready = (await api(page, '/capsules?state=ready')).capsules[0].capsule.id
  await go(page, `/c/${ready}/open`)
  await shot(page, 'desktop-ceremony')
  await page.click('text=Break the seal')
  await page.waitForTimeout(1800)
  assert.ok((await page.content()).includes(' of '), 'the things come one by one')
  await shot(page, 'desktop-reveal')
  await go(page, `/c/${ready}`)
  await page.click('.outcome >> text=Came true')
  await page.waitForTimeout(800)
  await ctx.close()
}
{
  const { ctx, page } = await session({ dark: true })
  await go(page, '/', 1500)
  await shot(page, 'desktop-home-dark')
  await ctx.close()
}
{
  const { ctx, page } = await session({ phone: true })
  await go(page, '/', 1500)
  await shot(page, 'phone-home')
  const mila = await idOf(page, 'For Mila')
  const view = await api(page, `/capsules/${mila}`)
  await go(page, `/c/${mila}`)
  await shot(page, 'phone-sealed')
  const rec = view.recipients[0].url.split('/o/')[1]
  await go(page, `/o/${rec}`)
  await shot(page, 'phone-recipient')
  const fam = await idOf(page, 'Family capsule')
  const give = (await api(page, `/capsules/${fam}`)).give_url.split('/give/')[1]
  await go(page, `/give/${give}`)
  await shot(page, 'phone-give')
  await ctx.close()
}
await browser.close().catch(() => {})
if (errors.length) { console.error(errors.join('\n')); process.exit(1) }
console.log('ok')
