/**
 * Dashboard tests — Today section, nav, stat cards.
 */
const { test, expect } = require('@playwright/test');

test.describe('Dashboard', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
  });

  test('loads dashboard with Today section', async ({ page }) => {
    await expect(page.locator('h2:has-text("Today")')).toBeVisible();
  });

  test('Today section has all 4 action cards', async ({ page }) => {
    // Cards are rendered inside the todaySection — text is in <p> tags
    const today = page.locator('h2:has-text("Today")').locator('..');
    await expect(today.locator('text=Overdue Invoices')).toBeVisible();
    await expect(today.locator('text=Services Due Billing')).toBeVisible();
    await expect(today.locator('text=Open Tickets')).toBeVisible();
    await expect(today.locator('text=Overdue Tasks')).toBeVisible();
  });

  test('sidebar navigation links are present', async ({ page }) => {
    await expect(page.locator('a[href="/customers"]')).toBeVisible();
    await expect(page.locator('a[href="/invoices"]')).toBeVisible();
    await expect(page.locator('a[href="/tickets"]')).toBeVisible();
    await expect(page.locator('a[href="/tasks"]')).toBeVisible();
  });

  test('dashboard renders without errors', async ({ page }) => {
    await expect(page.locator('h2:has-text("Today")')).toBeVisible();
    const bodyText = await page.locator('body').innerText();
    expect(bodyText).not.toContain('panic:');
    expect(bodyText).not.toContain('500 Internal Server Error');
  });
});
