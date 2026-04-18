/**
 * Router (network) tests — list and new form.
 */
const { test, expect } = require('@playwright/test');

test.describe('Routers', () => {
  test('router list page loads', async ({ page }) => {
    await page.goto('/routers');
    await expect(page.getByRole('heading', { name: 'Routers' })).toBeVisible();
    await expect(page.locator('a[href="/routers/new"]')).toBeVisible();
  });

  test('router table loads via SSE', async ({ page }) => {
    await page.goto('/routers');
    await page.waitForSelector('#router-table', { timeout: 8_000 });
    // Either a table or empty state is rendered
    const hasContent = await page.locator('#router-table').count();
    expect(hasContent).toBeGreaterThan(0);
    // No server errors
    const bodyText = await page.locator('body').innerText();
    expect(bodyText).not.toContain('panic:');
  });

  test('new router form loads', async ({ page }) => {
    await page.goto('/routers/new');
    // Page title is "New Router" per routerFormTitle()
    await expect(page.getByRole('heading', { name: 'New Router' })).toBeVisible({ timeout: 5_000 });
    // Must have a name field
    await expect(page.locator('#name')).toBeVisible();
  });

  test('new router form has required fields', async ({ page }) => {
    await page.goto('/routers/new');
    await expect(page.getByRole('heading', { name: 'New Router' })).toBeVisible();

    // Name, Host, RouterType fields should be present
    await expect(page.locator('#name')).toBeVisible();
    await expect(page.locator('#host')).toBeVisible();
    // Submit button
    await expect(page.locator('button:has-text("Create Router")')).toBeVisible();
  });
});
