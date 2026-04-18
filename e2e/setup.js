/**
 * Auth setup — runs once before all tests.
 * Logs in with admin/secret and saves the session cookie to .auth.json.
 * Datastar login uses SSE redirect (window.location), not a full page navigation.
 */
const { test: setup, expect } = require('@playwright/test');
const path = require('path');

const AUTH_FILE = path.join(__dirname, '.auth.json'); // absolute — same as playwright.config.js

setup('authenticate', async ({ page }) => {
  await page.goto('/login');
  await expect(page.locator('text=VVS ISP')).toBeVisible();

  await page.fill('input[type="text"]', process.env.VVS_ADMIN_USER || 'admin');
  await page.fill('input[type="password"]', process.env.VVS_ADMIN_PASSWORD || 'secret');

  // Click login; Datastar fires @post('/api/login') which SSE-redirects to /
  await page.click('button:has-text("Sign in")');

  // Wait for Datastar to process the SSE redirect event → window.location = '/'
  await page.waitForURL('/', { timeout: 10_000 });
  await expect(page.locator('h2:has-text("Today")')).toBeVisible();

  await page.context().storageState({ path: AUTH_FILE });
});
