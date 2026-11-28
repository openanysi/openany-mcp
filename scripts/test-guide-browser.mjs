import assert from 'node:assert/strict';
import { createRequire } from 'node:module';
import { mkdir, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

const require = createRequire(import.meta.url);
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const base = process.env.GUIDE_PREVIEW_URL || 'http://127.0.0.1:5290';
const output = process.env.GUIDE_QA_DIR || join(tmpdir(), 'openany-mcp-guide-qa');
await mkdir(output, { recursive: true });
const browser = await chromium.launch({ headless: true, channel: 'chrome' });
const checks = [];
try {
  for (const lang of ['en', 'zh']) {
    for (const viewport of [{ width: 1440, height: 1000 }, { width: 375, height: 812 }, { width: 1440, height: 600 }]) {
      const context = await browser.newContext({ viewport });
      await context.grantPermissions(['clipboard-read', 'clipboard-write'], { origin: base });
      const page = await context.newPage();
      const requests = [];
      const errors = [];
      page.on('pageerror', error => errors.push(error.message));
      await page.route('**/*', route => {
        const request = route.request();
        requests.push({ method: request.method(), url: request.url() });
        return request.method() === 'GET' && new URL(request.url()).origin === new URL(base).origin ? route.continue() : route.abort();
      });
      await page.goto(`${base}/?lang=${lang}`, { waitUntil: 'networkidle' });
      const disclosures = page.locator('details');
      assert.equal(await disclosures.count(), 2);
      assert.equal(await disclosures.nth(0).getAttribute('open'), '');
      assert.equal(await disclosures.nth(1).getAttribute('open'), null);
      const noOverflow = async () => {
        const size = await page.evaluate(() => ({ page: document.documentElement.scrollWidth, viewport: innerWidth }));
        assert.ok(size.page <= size.viewport, JSON.stringify({ lang, viewport, size }));
      };
      await noOverflow();
      assert.ok(await page.locator('a[href="https://openany.si/mcp/"]').count() > 0);
      assert.ok(await page.locator('a[href="https://openany.si/dashboard?page=api-keys"]').count() > 0);
      assert.ok(!/168\+|4,400\+|3,200\+|any MCP client/.test(await page.locator('main').innerText()));
      assert.equal(await page.locator('.table-scroll').nth(0).locator('tbody tr').count(), 7);
      assert.equal(await page.locator('.table-scroll').nth(1).locator('tbody tr').count(), 4);

      const copy = page.locator('[data-copy-endpoint]');
      await page.bringToFront();
      await copy.focus();
      assert.equal(await copy.evaluate(element => getComputedStyle(element).outlineStyle), 'solid');
      await page.keyboard.press('Enter');
      await page.waitForFunction(() => document.querySelector('[data-copy-status]').textContent.length > 0);
      assert.equal(await page.evaluate(() => Promise.race([
        navigator.clipboard.readText(),
        new Promise((_, reject) => setTimeout(() => reject(new Error('Clipboard read timed out')), 5000)),
      ])), 'https://mcp.openany.si');
      assert.match(await page.locator('[data-copy-status]').innerText(), /copied|已复制/);

      await page.evaluate(() => Object.defineProperty(navigator, 'clipboard', {
        configurable: true,
        value: { writeText: async () => { throw new Error('denied'); } },
      }));
      await copy.click();
      await page.waitForFunction(() => /failed|失败/.test(document.querySelector('[data-copy-status]').textContent));
      assert.equal(await copy.isEnabled(), true);
      await disclosures.nth(1).locator('summary').click();
      await noOverflow();
      await disclosures.nth(1).locator('summary').click();
      await page.evaluate(() => { document.querySelector('[data-copy-status]').textContent = ''; scrollTo(0, 0); });
      const screenshot = `${lang}-${viewport.width}x${viewport.height}.png`;
      await page.screenshot({ path: join(output, screenshot) });
      if (viewport.width === 1440 && viewport.height === 1000) {
        await page.locator('h3').filter({ hasText: lang === 'zh' ? 'API 类别' : 'API categories' }).evaluate(element => element.scrollIntoView({ block: 'start' }));
        await page.screenshot({ path: join(output, `${lang}-catalog.png`) });
      }
      if (viewport.width === 375) {
        const table = page.locator('.table-scroll').first();
        await table.focus();
        assert.ok(await table.evaluate(element => element.scrollWidth > element.clientWidth));
        await page.keyboard.press('ArrowRight');
        await page.waitForFunction(() => document.querySelector('.table-scroll').scrollLeft > 0);
        await page.screenshot({ path: join(output, `${lang}-mobile-table.png`) });
      }
      assert.deepEqual(errors, []);
      assert.equal(requests.some(request => request.method !== 'GET'), false);
      checks.push({ lang, viewport, screenshot, overflow: false, copy: 'passed', copyFailure: 'passed', businessRequests: 0 });
      await context.close();
    }
  }
  const noJS = await browser.newContext({ javaScriptEnabled: false, viewport: { width: 375, height: 812 } });
  const page = await noJS.newPage();
  await page.goto(base);
  assert.equal(await page.locator('[data-copy-endpoint]').isVisible(), false);
  assert.equal(await page.locator('details').first().getAttribute('open'), '');
  assert.ok(await page.locator('main').innerText().then(text => text.includes('https://mcp.openany.si')));
  await noJS.close();
  await writeFile(join(output, 'results.json'), JSON.stringify({ checks, noJS: 'passed' }, null, 2));
  console.log(JSON.stringify({ output, viewsPassed: checks.length, noJS: 'passed', businessRequests: 0 }));
} finally {
  await browser.close();
}
