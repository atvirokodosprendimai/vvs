/**
 * Profile page tests — page load, change password flow, 2FA setup.
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
    const bodyText = await page.locator('body').innerText();
    expect(bodyText).toMatch(/Username|username/i);
    expect(bodyText).toMatch(/Role|role/i);
  });

  test('display name input present', async ({ page }) => {
    await page.goto('/profile');
    await expect(page.locator('input[placeholder="Your full name"]')).toBeVisible();
    await expect(page.locator('button:has-text("Save display name")')).toBeVisible();
  });

  // ── Two-Factor Authentication ──────────────────────────────────────────────

  test('profile page has 2FA link', async ({ page }) => {
    await page.goto('/profile');
    await expect(page.locator('a[href="/profile/2fa"]')).toBeVisible();
  });

  test('2FA setup page loads', async ({ page }) => {
    await page.goto('/profile/2fa');
    await expect(page.locator('h1, h2').filter({ hasText: /Two-Factor/i })).toBeVisible();
  });

  test('2FA setup page has no server errors', async ({ page }) => {
    await page.goto('/profile/2fa');
    const bodyText = await page.locator('body').innerText();
    expect(bodyText).not.toContain('panic:');
    expect(bodyText).not.toContain('500');
  });

  test('2FA setup page shows QR code and confirm input', async ({ page }) => {
    await page.goto('/profile/2fa');
    // QR code image
    await expect(page.locator('img[alt="TOTP QR code"]')).toBeVisible();
    // Manual key code block
    await expect(page.locator('code')).toBeVisible();
    // 6-digit confirm input
    await expect(page.locator('input[placeholder="000000"]')).toBeVisible();
    // Enable button
    await expect(page.locator('button:has-text("Enable Two-Factor Auth")')).toBeVisible();
  });

  test('2FA wrong code shows error', async ({ page }) => {
    await page.goto('/profile/2fa');
    await page.fill('input[placeholder="000000"]', '000000');
    await page.click('button:has-text("Enable Two-Factor Auth")');
    await expect(page.locator('#totp-setup-error')).toBeVisible({ timeout: 5_000 });
    const errText = await page.locator('#totp-setup-error').innerText();
    expect(errText.length).toBeGreaterThan(0);
  });
});
