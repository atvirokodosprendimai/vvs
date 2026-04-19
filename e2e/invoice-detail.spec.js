/**
 * Invoice detail page tests — detail view, status actions, line items.
 *
 * Requires at least one invoice. If none exist, tests that navigate
 * to the list only will still pass.
 */
const { test, expect } = require('@playwright/test');

test.describe('Invoice Detail', () => {
  test('invoice list page loads', async ({ page }) => {
    await page.goto('/invoices');
    await expect(page.getByRole('heading', { name: 'Invoices' })).toBeVisible();
  });

  test('invoice table loads via SSE', async ({ page }) => {
    await page.goto('/invoices');
    await page.waitForSelector('#invoice-table', { timeout: 8_000 });
    const bodyText = await page.locator('body').innerText();
    expect(bodyText).not.toContain('panic:');
  });

  test('status filter buttons present', async ({ page }) => {
    await page.goto('/invoices');
    await expect(page.locator('button:has-text("All")')).toBeVisible();
    await expect(page.locator('button:has-text("Draft")')).toBeVisible();
    await expect(page.locator('button:has-text("Finalized")')).toBeVisible();
    await expect(page.locator('button:has-text("Paid")')).toBeVisible();
  });

  test('filter by Draft does not crash', async ({ page }) => {
    await page.goto('/invoices');
    await page.waitForSelector('#invoice-table', { timeout: 8_000 });
    await page.click('button:has-text("Draft")');
    await page.waitForTimeout(500);
    const bodyText = await page.locator('body').innerText();
    expect(bodyText).not.toContain('panic:');
  });

  test('new invoice form loads', async ({ page }) => {
    await page.goto('/invoices/new');
    const bodyText = await page.locator('body').innerText();
    expect(bodyText).not.toContain('panic:');
    expect(bodyText).not.toContain('500');
  });

  test('invoice detail page loads when invoice exists', async ({ page }) => {
    await page.goto('/invoices');
    await page.waitForSelector('#invoice-table', { timeout: 8_000 });

    const firstLink = page.locator('#invoice-table a[href^="/invoices/"]').first();
    const linkCount = await firstLink.count();
    if (linkCount === 0) {
      // No invoices in DB — skip navigation test
      return;
    }

    await firstLink.click();
    await page.waitForURL(/\/invoices\/[^/]+$/, { timeout: 8_000 });

    const bodyText = await page.locator('body').innerText();
    expect(bodyText).not.toContain('panic:');
    expect(bodyText).not.toContain('500');
  });

  test('invoice detail content section loads via SSE', async ({ page }) => {
    await page.goto('/invoices');
    await page.waitForSelector('#invoice-table', { timeout: 8_000 });

    const firstLink = page.locator('#invoice-table a[href^="/invoices/"]').first();
    if (await firstLink.count() === 0) return;

    await firstLink.click();
    await page.waitForURL(/\/invoices\/[^/]+$/, { timeout: 8_000 });
    await page.waitForSelector('#invoice-detail-content', { timeout: 8_000 });

    const bodyText = await page.locator('body').innerText();
    expect(bodyText).not.toContain('panic:');
  });

  test('invoice status actions section present', async ({ page }) => {
    await page.goto('/invoices');
    await page.waitForSelector('#invoice-table', { timeout: 8_000 });

    const firstLink = page.locator('#invoice-table a[href^="/invoices/"]').first();
    if (await firstLink.count() === 0) return;

    await firstLink.click();
    await page.waitForURL(/\/invoices\/[^/]+$/, { timeout: 8_000 });
    await page.waitForSelector('#invoice-status-actions', { timeout: 8_000 });

    await expect(page.locator('#invoice-status-actions')).toBeVisible();
  });

  test('back link navigates to invoice list', async ({ page }) => {
    await page.goto('/invoices');
    await page.waitForSelector('#invoice-table', { timeout: 8_000 });

    const firstLink = page.locator('#invoice-table a[href^="/invoices/"]').first();
    if (await firstLink.count() === 0) return;

    await firstLink.click();
    await page.waitForURL(/\/invoices\/[^/]+$/, { timeout: 8_000 });

    // Back arrow link
    await page.locator('a[href="/invoices"]').first().click();
    await page.waitForURL('/invoices', { timeout: 6_000 });
    await expect(page.getByRole('heading', { name: 'Invoices' })).toBeVisible();
  });
});
