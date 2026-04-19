/**
 * Invoice tests — list, status filters, PDF view.
 */
const { test, expect } = require('@playwright/test');

test.describe('Invoices', () => {
  test('invoice list page loads with New Invoice button', async ({ page }) => {
    await page.goto('/invoices');
    await expect(page.getByRole('heading', { name: 'Invoices' })).toBeVisible();
    await expect(page.locator('a[href="/invoices/new"]')).toBeVisible();
  });

  test('invoice list has status filter buttons', async ({ page }) => {
    await page.goto('/invoices');
    // Status filters use data-on:click with statusFilter signal
    // Look for buttons that contain Draft/Finalized text
    await expect(
      page.locator('button:has-text("Draft")').or(page.locator('[data-on*="draft"]')).first()
    ).toBeVisible({ timeout: 5_000 });
  });

  test('invoice table loads via SSE', async ({ page }) => {
    await page.goto('/invoices');
    await page.waitForSelector('#invoice-table', { timeout: 8_000 });
    // No server errors
    const bodyText = await page.locator('body').innerText();
    expect(bodyText).not.toContain('panic:');
  });

  test('All filter shows invoices', async ({ page }) => {
    await page.goto('/invoices');
    await page.waitForSelector('#invoice-table', { timeout: 8_000 });

    // Click "All" filter — clears statusFilter signal
    const allBtn = page.locator('button:has-text("All")');
    if (await allBtn.isVisible()) {
      await allBtn.click();
      await page.waitForTimeout(500);
    }
    await expect(page.locator('#invoice-table')).toBeVisible();
  });

  test('new invoice page loads', async ({ page }) => {
    await page.goto('/invoices/new');
    // The new invoice form exists (redirects or renders form)
    const url = page.url();
    expect(url).toMatch(/\/(invoices\/new|customers|invoices)/);
    const bodyText = await page.locator('body').innerText();
    expect(bodyText).not.toContain('panic:');
  });

  test('invoice PDF endpoint returns HTML', async ({ page }) => {
    await page.goto('/invoices');
    await page.waitForSelector('#invoice-table', { timeout: 8_000 });

    const invoiceLink = page.locator('#invoice-table a[href*="/invoices/"]').first();
    if (await invoiceLink.count() > 0) {
      const href = await invoiceLink.getAttribute('href');
      const invoiceID = href?.match(/\/invoices\/([a-z0-9-]+)/)?.[1];

      if (invoiceID) {
        const response = await page.request.get(`/invoices/${invoiceID}/pdf`);
        expect(response.status()).toBe(200);
        expect(response.headers()['content-type']).toContain('text/html');

        await page.goto(`/invoices/${invoiceID}/pdf`);
        const bodyText = await page.locator('body').innerText();
        expect(bodyText).not.toContain('not found');
        expect(bodyText).not.toContain('panic:');
      }
    } else {
      test.skip(true, 'No invoices in database to test PDF');
    }
  });
});
