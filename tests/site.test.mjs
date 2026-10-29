import assert from 'node:assert/strict';
import {before, after, test} from 'node:test';
import {createServer} from 'node:http';
import {readFile} from 'node:fs/promises';
import {fileURLToPath} from 'node:url';
import path from 'node:path';
import {chromium, firefox, webkit} from 'playwright';

const root = path.dirname(path.dirname(fileURLToPath(import.meta.url)));
const engines = {chromium, firefox, webkit};
const engine = engines[process.env.SITE_BROWSER || 'chromium'];
assert.ok(engine, 'SITE_BROWSER must be chromium, firefox or webkit');
let server, browser, url;

before(async () => {
  server = createServer(async (request, response) => {
    const pathname = new URL(request.url, 'http://localhost').pathname;
    const file = {'/':'index.html', '/wasm_exec.js':'wasm_exec.js', '/sentinel.wasm':'sentinel.wasm'}[pathname];
    if (!file) { response.writeHead(404); response.end(); return; }
    const data = await readFile(path.join(root, '_site', file));
    response.writeHead(200, {
      'Content-Type': file.endsWith('.wasm') ? 'application/wasm' : file.endsWith('.js') ? 'text/javascript' : 'text/html',
      'Cache-Control':'no-store',
    });
    response.end(data);
  });
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
  url = `http://127.0.0.1:${server.address().port}/`;
  browser = await engine.launch();
});

after(async () => {
  await browser?.close();
  await new Promise(resolve => server?.close(resolve));
});

for (const width of [375, 1440]) {
  test(`cold page stays static, accessible and within ${width}px`, async t => {
    const context = await browser.newContext({viewport:{width,height:900},reducedMotion:'reduce'});
    try {
      const page = await context.newPage(), requests = [], errors = [];
      page.on('request', request => { if (/wasm_exec|\.wasm/.test(request.url())) requests.push(request.url()); });
      page.on('pageerror', error => errors.push(String(error)));
      await page.goto(url, {waitUntil:'networkidle'});
      await page.locator('#try').scrollIntoViewIfNeeded();
      await page.waitForTimeout(300);
      assert.deepEqual(requests, [], 'the engine must wait for explicit activation');
      assert.ok(await page.locator('#run').isDisabled());
      assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false);
      await page.addScriptTag({path:path.join(root,'node_modules/axe-core/axe.min.js')});
      const violations = await page.evaluate(async () => (await axe.run(document, {
        runOnly:{type:'tag',values:['wcag2a','wcag2aa','wcag21aa']},
      })).violations.map(v => ({id:v.id,targets:v.nodes.map(n=>n.target)})));
      assert.deepEqual(violations, []);
      for (const block of await page.locator('pre').all()) {
        await block.focus();
        assert.equal(await block.evaluate(e=>document.activeElement===e), true);
      }
      assert.deepEqual(errors, []);
      t.diagnostic(JSON.stringify(await page.evaluate(() => ({width:innerWidth,
        navigationBytes:performance.getEntriesByType('navigation')[0].transferSize,
        engineRequests:performance.getEntriesByType('resource').filter(r=>/wasm_exec|\.wasm/.test(r.name)).length,
      }))));
    } finally { await context.close(); }
  });
}

async function startEngine(page) {
  await page.waitForFunction(() => window.__SENT_READY__ ||
    document.getElementById('boot')?.textContent.includes('could not load'));
  assert.equal(await page.evaluate(() => window.__SENT_READY__), true,
    `Real Go engine did not load: ${await page.locator('#out').innerText()}`);
}

test('keyboard activation runs the compiled Go evaluator and traffic scenarios', async () => {
  const context = await browser.newContext();
  try {
    const page = await context.newPage(), requests = [], errors = [];
    page.on('request', request => { if (/\.wasm/.test(request.url())) requests.push(request.url()); });
    page.on('pageerror', error => errors.push(String(error)));
    await page.goto(url, {waitUntil:'networkidle'});
    await page.locator('#start-demo').focus();
    await page.locator('#start-demo').press('Enter');
    await startEngine(page);
    assert.equal(requests.length, 1, 'the actual compiled WASM engine must be fetched');
    assert.equal(await page.locator('#out .vd b').innerText(), 'PASS');
    assert.equal(await page.evaluate(()=>document.activeElement.id), 's-err');
    await page.locator('[data-p="High CPU"]').click();
    assert.equal(await page.locator('#out .vd b').innerText(), 'WARN');
    await page.locator('[data-p="Error spike"]').click();
    assert.equal(await page.locator('#out .vd b').innerText(), 'BREACH');
    assert.ok((await page.locator('#out').innerText()).includes('Rollback triggered'));
    await page.locator('[data-p="Healthy"]').click();
    await page.locator('#ramp').click();
    assert.ok((await page.locator('#out').innerText()).includes('Reached 100%'));
    assert.deepEqual(errors, []);
  } finally { await context.close(); }
});

test('failed engine download offers local fallback and retry recovers', async () => {
  const context = await browser.newContext({viewport:{width:375,height:900}});
  try {
    const page = await context.newPage();
    await page.route('**/sentinel.wasm', route=>route.fulfill({status:503,body:'unavailable'}));
    await page.goto(url);
    await page.locator('#start-demo').click();
    await page.waitForFunction(()=>document.getElementById('start-demo').textContent.includes('Retry'));
    assert.ok((await page.locator('#boot').innerText()).includes('local commands below'));
    assert.equal(await page.locator('#out').getAttribute('aria-busy'), 'false');
    assert.ok(await page.locator('#run').isDisabled());
    await page.unroute('**/sentinel.wasm');
    await page.locator('#start-demo').click();
    await startEngine(page);
    assert.equal(await page.locator('#out .vd b').innerText(), 'PASS');
  } finally { await context.close(); }
});

test('JavaScript-disabled visitors get a clear local path', async () => {
  const context = await browser.newContext({javaScriptEnabled:false,viewport:{width:375,height:900}});
  try {
    const page = await context.newPage();
    await page.goto(url);
    assert.ok((await page.locator('noscript').innerText()).includes('Go evaluator locally'));
    assert.equal(await page.locator('#start-demo').isVisible(), false);
    assert.ok(await page.locator('#run').isDisabled());
    assert.ok((await page.locator('#install').innerText()).includes('--demo'));
  } finally { await context.close(); }
});
