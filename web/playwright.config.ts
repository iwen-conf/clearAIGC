import { env } from 'node:process'
import { defineConfig } from '@playwright/test'

const baseURL = env.PLAYWRIGHT_BASE_URL ?? 'http://127.0.0.1:8080'

export default defineConfig({
  testDir: './e2e',
  timeout: 180_000,
  expect: {
    timeout: 60_000,
  },
  fullyParallel: false,
  forbidOnly: Boolean(env.CI),
  retries: env.CI ? 1 : 0,
  reporter: 'list',
  use: {
    baseURL,
    headless: true,
    screenshot: 'only-on-failure',
    trace: 'retain-on-failure',
  },
  projects: [
    {
      name: 'chromium',
      use: {
        browserName: 'chromium',
      },
    },
  ],
})
