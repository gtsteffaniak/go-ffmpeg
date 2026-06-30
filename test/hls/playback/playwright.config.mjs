/** @type {import('@playwright/test').PlaywrightTestConfig} */
export default {
  testDir: '.',
  timeout: 120_000,
  use: {
    headless: true,
  },
  projects: [{ name: 'chromium', use: { browserName: 'chromium' } }],
};
