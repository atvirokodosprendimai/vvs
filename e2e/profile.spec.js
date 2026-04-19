/**
 * Profile page tests — page load, change password flow.
 */
const { test, expect } = require('@playwright/test');

test.describe('Profile', () => {
  test('profile page loads', async ({ page }) => {
    await page.goto('/profile');
    await expect(page.locator('h1, h2').filter({ hasText: 'Profile' })).toBeVisible();
  });

  test('profile page has no server errors', async ({ page }) => {
    await page.goto('/profile');
    const bodyText = await page.locator('body').innerText();
    expect(bodyText).not.toContain('panic:');
    expect(bodyText).not.toContain('500');
  });

  test('change password fields present', async ({ page }) => {
    await page.goto('/profile');
    // Inputs use data-bind:current-password / data-bind:new-password
    await expect(page.locator('input[placeholder="Current password"]')).toBeVisible();
    await expect(page.locator('input[placeholder="New password"]')).toBeVisible();
    await expect(page.locator('button:has-text("Update password")')).toBeVisible();
  });

  test('wrong current password shows error', async ({ page }) => {
    await page.goto('/profile');
    await page.fill('input[placeholder="Current password"]', 'wrongpassword');
    await page.fill('input[placeholder="New password"]', 'NewPass123!');
    await page.click('button:has-text("Update password")');
    // Error element should appear
    await expect(page.locator('#change-pw-error')).toBeVisible({ timeout: 5_000 });
    const errText = await page.locator('#change-pw-error').innerText();
    expect(errText.length).toBeGreaterThan(0);
  });

  test('username and role displayed', async ({ page }) => {
    await page.goto('/profile');
    // Username and role sections should be visible
    const bodyText = await page.locator('body').innerText();
    expect(bodyText).toMatch(/Username|username/i);
    expect(bodyText).toMatch(/Role|role/i);
  });
});
