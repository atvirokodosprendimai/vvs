// @ts-check
const { defineConfig, devices } = require('@playwright/test');
const path = require('path');

const AUTH_FILE = path.join(__dirname, '.auth.json');

module.exports = defineConfig({
  testDir: '.',
  timeout: 30_000,
  expect: { timeout: 5_000 },
  fullyParallel: false, // SSE + shared DB — run serially
  retries: 0,
  workers: 1,
  reporter: 'list',

  use: {
    baseURL: process.env.VVS_URL || 'http://localhost:8080',
    actionTimeout: 10_000,
    headless: true,
  },

  projects: [
    // Setup: login once, save cookie
    {
      name: 'setup',
      testMatch: /setup\.js/,
    },
    {
      name: 'chromium',
      use: {
        ...devices['Desktop Chrome'],
        // storageState only injected here — not in global use, so setup doesn't choke
        storageState: AUTH_FILE,
      },
      dependencies: ['setup'],
    },
  ],
});
