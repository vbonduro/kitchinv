import { defineConfig, devices } from '@playwright/test';

const APP_PORT = process.env.APP_PORT || '9090';
const CI = !!process.env.CI;

export default defineConfig({
  testDir: './tests',
  globalSetup: './global-setup.ts',
  globalTeardown: './global-teardown.ts',

  timeout: 30_000,
  expect: { timeout: 10_000 },
  retries: 0,
  workers: CI ? 2 : undefined,

  reporter: CI ? 'github' : 'list',

  use: {
    baseURL: `http://localhost:${APP_PORT}`,
    headless: true,
    screenshot: 'only-on-failure',
    trace: 'on-first-retry',
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
