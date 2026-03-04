import { defineConfig, devices } from '@playwright/test';

const CI = !!process.env.CI;

export default defineConfig({
  testDir: './tests',
  globalSetup: './global-setup.ts',
  globalTeardown: './global-teardown.ts',

  timeout: 30_000,
  expect: { timeout: 10_000 },
  retries: 0,
  workers: CI ? 2 : 2,

  reporter: CI ? 'github' : 'list',

  use: {
    // baseURL is provided per-worker by the appPort fixture in fixtures.ts.
    headless: true,
    screenshot: 'only-on-failure',
    trace: 'on-first-retry',
  },

  projects: [
    {
      // Main suite: all tests except navigation (which needs serial isolation).
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
      testIgnore: '**/navigation.spec.ts',
    },
    {
      // Navigation tests run after the main suite and get their own dedicated
      // worker so they don't interleave with parallel search/areas tests that
      // share the same per-worker server instance.
      name: 'navigation',
      use: { ...devices['Desktop Chrome'] },
      testMatch: '**/navigation.spec.ts',
      dependencies: ['chromium'],
    },
    {
      name: 'mobile-chrome',
      use: { ...devices['Pixel 5'] },
      testMatch: '**/regression.spec.ts',
      dependencies: ['chromium'],
    },
  ],
});
