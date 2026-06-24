import { defineConfig, devices } from '@playwright/test'

// Where the app is served. Defaults to the Vite dev server (which this config
// auto-starts). In CI/Docker, set E2E_BASE_URL to a running instance (e.g.
// http://localhost:3000) and the webServer is skipped.
const baseURL = process.env.E2E_BASE_URL || 'http://localhost:5173'
const useExternal = Boolean(process.env.E2E_BASE_URL)

export default defineConfig({
  testDir: './e2e',
  // The journey is stateful (one user walks through every feature), so run the
  // steps in order, not in parallel.
  fullyParallel: false,
  workers: 1,
  timeout: 30_000,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [['list'], ['html', { open: 'never' }]] : 'list',

  use: {
    baseURL,
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
  },

  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],

  // Auto-start the frontend dev server unless we're pointed at an external one.
  webServer: useExternal
    ? undefined
    : {
        command: 'npm run dev',
        url: baseURL,
        reuseExistingServer: !process.env.CI,
        timeout: 60_000,
      },
})
