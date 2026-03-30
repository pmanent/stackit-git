import {devices, type PlaywrightTestConfig} from '@playwright/test';

const BASE_URL = process.env.GITEA_URL?.replace?.(/\/$/g, '') || 'http://localhost:3003';

/**
 * @see https://playwright.dev/docs/test-configuration
 * @type {import('@playwright/test').PlaywrightTestConfig}
 */
export default {
  testDir: './tests/e2e/',
  testMatch: /.*\.test\.e2e\.ts/, // Match any .test.e2e.ts files

  // you can adjust this value locally to match your machine's power,
  // or pass `--workers x` to playwright
  workers: 1,

  /* Maximum time one test can run for. */
  timeout: 60 * 1000,

  expect: {
    /**
     * Maximum time expect() should wait for the condition to be met.
     * For example in `await expect(locator).toHaveText();`
     */
    timeout: 10 * 1000,
  },

  /* Fail the build on CI if you accidentally left test.only in the source code. */
  forbidOnly: Boolean(process.env.CI),

  /* Retry on CI only */
  retries: process.env.CI ? 3 : 0,
  /* fail fast */
  maxFailures: process.env.CI ? 5 : 0,

  /* Reporter to use. See https://playwright.dev/docs/test-reporters */
  reporter: process.env.CI ? 'list' : [['list'], ['html', {outputFolder: 'tests/e2e/reports/', open: 'never'}]],

  /* Shared settings for all the projects below. See https://playwright.dev/docs/api/class-testoptions. */
  use: {
    headless: true,   // set to false to debug

    locale: 'en-US',

    /* Maximum time each action such as `click()` can take. Defaults to 0 (no limit). */
    actionTimeout: 10 * 1000,

    /* Maximum time allowed for navigation, such as `page.goto()`. */
    navigationTimeout: 30 * 1000,

    /* Base URL to use in actions like `await page.goto('/')`. */
    baseURL: BASE_URL,

    trace: 'off',

    screenshot: 'only-on-failure',
  },

  /* Configure projects for major browsers */
  projects: [
    {
      name: 'chromium',

      /* Project-specific settings. */
      use: {
        ...devices['Desktop Chrome'],
        permissions: ['clipboard-read', 'clipboard-write'],
      },
    },

  ],

  /* Folder for test artifacts created during test execution such as screenshots, traces, etc. */
  outputDir: 'tests/e2e/test-artifacts/',
  /* Folder for explicit snapshots for visual testing */
  snapshotDir: 'tests/e2e/test-snapshots/',
  snapshotPathTemplate: '{snapshotDir}/snapshots/{testFilePath}/{projectName}_{arg}{ext}',
} satisfies PlaywrightTestConfig;
