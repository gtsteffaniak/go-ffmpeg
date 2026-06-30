import { test, expect } from '@playwright/test';

test('smooth playback — no playhead jumps', async ({ page }) => {
  const url = process.env.PLAYBACK_TEST_URL;
  if (!url) {
    test.skip(true, 'PLAYBACK_TEST_URL not set');
  }

  const consoleErrors = [];
  page.on('console', (msg) => {
    if (msg.type() === 'error') {
      consoleErrors.push(msg.text());
    }
  });

  await page.goto(url, { waitUntil: 'domcontentloaded' });
  await page.waitForFunction(() => window.__playbackDone === true, null, { timeout: 90_000 });

  const audit = await page.evaluate(() => window.__playbackAudit);
  expect(audit, 'audit object missing').toBeTruthy();
  expect(audit.jumps, `playhead jumps: ${JSON.stringify(audit.jumps)}`).toHaveLength(0);
  expect(audit.errors?.filter((e) => e.fatal)?.length ?? 0).toBe(0);
  expect(consoleErrors, `console errors: ${consoleErrors.join('; ')}`).toHaveLength(0);
});
