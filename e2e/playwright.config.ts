import { defineConfig, devices } from '@playwright/test';

const CI = !!process.env.CI;
const DEBUG = !!process.env.E2E_DEBUG;

export default defineConfig({
  testDir: './tests',
  globalSetup: './global-setup.ts',
  globalTeardown: './global-teardown.ts',

  timeout: 30_000,
  expect: { timeout: 10_000 },
  retries: 0,
  workers: CI ? 2 : 2,
  fullyParallel: true,

  reporter: DEBUG
    ? [['html', { outputFolder: 'debug-results/html', open: 'never' }], ['list']]
    : CI ? 'github' : 'list',
  outputDir: DEBUG ? 'debug-results/artifacts' : 'test-results',

  use: {
    // baseURL is provided per-test by the appPort fixture in fixtures.ts.
    headless: true,
    screenshot: DEBUG ? 'on' : 'only-on-failure',
    video: DEBUG ? 'on' : 'off',
    trace: DEBUG ? 'on' : 'on-first-retry',
  },

  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
    {
      name: 'mobile-chrome',
      use: { ...devices['Pixel 5'] },
      testMatch: '**/regression.spec.ts',
    },
  ],
});
