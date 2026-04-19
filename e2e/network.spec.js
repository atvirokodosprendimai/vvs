/**
 * Network module tests — Prefixes, Devices (Routers covered in routers.spec.js).
 */
const { test, expect } = require('@playwright/test');

// ── IP Prefixes ────────────────────────────────────────────────────────────

test.describe('IP Prefixes', () => {
  test('list page loads', async ({ page }) => {
    await page.goto('/prefixes');
    await expect(page.getByRole('heading', { name: 'IP Prefixes' })).toBeVisible();
  });

  test('prefix table loads via SSE', async ({ page }) => {
    await page.goto('/prefixes');
    await page.waitForSelector('#prefix-table', { timeout: 8_000 });
    const bodyText = await page.locator('body').innerText();
    expect(bodyText).not.toContain('panic:');
  });

  test('prefix page has no server errors', async ({ page }) => {
    await page.goto('/prefixes');
    await page.waitForSelector('#prefix-table', { timeout: 8_000 });
    const bodyText = await page.locator('body').innerText();
    expect(bodyText).not.toContain('500');
  });
});

// ── Devices ────────────────────────────────────────────────────────────────

test.describe('Devices', () => {
  test('list page loads', async ({ page }) => {
    await page.goto('/devices');
    await expect(page.getByRole('heading', { name: 'Devices' })).toBeVisible();
  });

  test('device table loads via SSE', async ({ page }) => {
    await page.goto('/devices');
    await page.waitForSelector('#device-table', { timeout: 8_000 });
    const bodyText = await page.locator('body').innerText();
    expect(bodyText).not.toContain('panic:');
  });

  test('device page has no server errors', async ({ page }) => {
    await page.goto('/devices');
    await page.waitForSelector('#device-table', { timeout: 8_000 });
    const bodyText = await page.locator('body').innerText();
    expect(bodyText).not.toContain('500');
  });
});
