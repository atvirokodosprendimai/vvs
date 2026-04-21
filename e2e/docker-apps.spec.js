/**
 * Docker Apps (git-source deploy) tests.
 *
 * Tests verify page structure, form fields, and navigation.
 * Build/deploy tests require a real Docker socket and are skipped in CI.
 */
const { test, expect } = require('@playwright/test');

test.describe('Docker Apps', () => {
  test('apps page loads', async ({ page }) => {
    await page.goto('/docker/apps');
    await expect(page.getByRole('heading', { name: 'Docker Apps' })).toBeVisible();
    const text = await page.locator('body').innerText();
    expect(text).not.toContain('panic:');
    expect(text).not.toContain('500 Internal');
  });

  test('apps table loads via SSE', async ({ page }) => {
    await page.goto('/docker/apps');
    await page.waitForSelector('#docker-apps-table', { timeout: 8_000 });
    const text = await page.locator('body').innerText();
    expect(text).not.toContain('panic:');
  });

  test('"+ New App" link is visible', async ({ page }) => {
    await page.goto('/docker/apps');
    await expect(page.locator('a:has-text("+ New App")')).toBeVisible();
  });

  test('new app form has required fields', async ({ page }) => {
    await page.goto('/docker/apps/new');
    const text = await page.locator('body').innerText();
    expect(text).not.toContain('panic:');
    expect(text).not.toContain('500 Internal');

    // Source card
    await expect(page.locator('input[name="repo_url"]')).toBeVisible();
    await expect(page.locator('input[name="branch"]')).toBeVisible();
    await expect(page.locator('input[name="name"]')).toBeVisible();
    await expect(page.locator('input[name="reg_user"]')).toBeVisible();
    await expect(page.locator('input[name="reg_pass"]')).toBeVisible();

    // Restart policy dropdown
    await expect(page.locator('select[name="restart_policy"]')).toBeVisible();

    // Submit button
    await expect(page.locator('button[type="submit"]:has-text("Create App")')).toBeVisible();
  });

  test('new app form has build args section', async ({ page }) => {
    await page.goto('/docker/apps/new');
    await expect(page.locator('text=Build Args')).toBeVisible();
    await expect(page.locator('text=+ Add build arg')).toBeVisible();
  });

  test('new app form has runtime section', async ({ page }) => {
    await page.goto('/docker/apps/new');
    await expect(page.locator('text=Runtime')).toBeVisible();
    await expect(page.locator('text=+ Add env var')).toBeVisible();
    await expect(page.locator('text=+ Add port')).toBeVisible();
    await expect(page.locator('text=+ Add volume')).toBeVisible();
  });

  test('cancel returns to apps list', async ({ page }) => {
    await page.goto('/docker/apps/new');
    await page.locator('a:has-text("Cancel")').click();
    await page.waitForURL('/docker/apps');
    await expect(page.getByRole('heading', { name: 'Docker Apps' })).toBeVisible();
  });
});
