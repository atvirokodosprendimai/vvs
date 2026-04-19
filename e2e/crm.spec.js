/**
 * CRM module tests — Tickets, Deals, Tasks (standalone list pages).
 * Contacts live inside the Customer detail page (covered in customer-detail.spec.js).
 */
const { test, expect } = require('@playwright/test');

// ── Tickets ────────────────────────────────────────────────────────────────

test.describe('Tickets', () => {
  test('list page loads', async ({ page }) => {
    await page.goto('/tickets');
    await expect(page.getByRole('heading', { name: 'Tickets' })).toBeVisible();
    await expect(page.locator('input[placeholder="Search tickets..."]')).toBeVisible();
  });

  test('ticket list loads via SSE', async ({ page }) => {
    await page.goto('/tickets');
    await page.waitForSelector('#all-tickets', { timeout: 8_000 });
    const bodyText = await page.locator('body').innerText();
    expect(bodyText).not.toContain('panic:');
  });

  test('search input is functional', async ({ page }) => {
    await page.goto('/tickets');
    await expect(page.locator('input[placeholder="Search tickets..."]')).toBeVisible();
    await page.fill('input[placeholder="Search tickets..."]', 'test');
    // No crash on search
    const bodyText = await page.locator('body').innerText();
    expect(bodyText).not.toContain('panic:');
  });
});

// ── Deals ──────────────────────────────────────────────────────────────────

test.describe('Deals', () => {
  test('list page loads', async ({ page }) => {
    await page.goto('/deals');
    await expect(page.getByRole('heading', { name: 'Deals' })).toBeVisible();
  });

  test('deals list loads via SSE', async ({ page }) => {
    await page.goto('/deals');
    await page.waitForSelector('#deals-page-content', { timeout: 8_000 });
    const bodyText = await page.locator('body').innerText();
    expect(bodyText).not.toContain('panic:');
  });

  test('deals page has no server errors', async ({ page }) => {
    await page.goto('/deals');
    await page.waitForSelector('#deals-page-content', { timeout: 8_000 });
    const bodyText = await page.locator('body').innerText();
    expect(bodyText).not.toContain('500');
  });
});

// ── Tasks ──────────────────────────────────────────────────────────────────

test.describe('Tasks', () => {
  test('list page loads', async ({ page }) => {
    await page.goto('/tasks');
    await expect(page.getByRole('heading', { name: 'Tasks' })).toBeVisible();
    await expect(page.locator('input[placeholder="Search tasks..."]')).toBeVisible();
  });

  test('tasks list loads via SSE', async ({ page }) => {
    await page.goto('/tasks');
    await page.waitForSelector('#tasks-content', { timeout: 8_000 });
    const bodyText = await page.locator('body').innerText();
    expect(bodyText).not.toContain('panic:');
  });

  test('tasks page has no server errors', async ({ page }) => {
    await page.goto('/tasks');
    await page.waitForSelector('#tasks-content', { timeout: 8_000 });
    const bodyText = await page.locator('body').innerText();
    expect(bodyText).not.toContain('500');
  });
});

// ── CRM Overview ───────────────────────────────────────────────────────────

test.describe('CRM Overview', () => {
  test('CRM dashboard loads', async ({ page }) => {
    await page.goto('/crm');
    await expect(page.getByRole('heading', { name: 'CRM' })).toBeVisible();
  });

  test('CRM stats load without errors', async ({ page }) => {
    await page.goto('/crm');
    await page.waitForTimeout(2_000); // SSE stats cards
    const bodyText = await page.locator('body').innerText();
    expect(bodyText).not.toContain('panic:');
    expect(bodyText).not.toContain('500 Internal Server Error');
  });
});
