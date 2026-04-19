/**
 * Auth tests — login / logout / bad credentials.
 * These clear cookies before each test to start unauthenticated.
 */
const { test, expect } = require('@playwright/test');

test.describe('Authentication', () => {
  test('login page renders correctly', async ({ page }) => {
    await page.context().clearCookies();
    await page.goto('/login');
    await expect(page.locator('text=VVS ISP')).toBeVisible();
    await expect(page.locator('input[type="text"]')).toBeVisible();
    await expect(page.locator('input[type="password"]')).toBeVisible();
    await expect(page.locator('button:has-text("Sign in")')).toBeVisible();
  });

  test('bad credentials shows error', async ({ page }) => {
    await page.context().clearCookies();
    await page.goto('/login');
    await page.fill('input[type="text"]', 'admin');
    await page.fill('input[type="password"]', 'wrong');
    await page.click('button:has-text("Sign in")');

    await expect(page.locator('#login-error')).toContainText('Invalid', { timeout: 5_000 });
    expect(page.url()).toContain('/login');
  });

  test('valid credentials redirect to dashboard', async ({ page }) => {
    await page.context().clearCookies();
    await page.goto('/login');
    await page.fill('input[type="text"]', process.env.VVS_ADMIN_USER || 'admin');
    await page.fill('input[type="password"]', process.env.VVS_ADMIN_PASSWORD || 'secret');
    await page.click('button:has-text("Sign in")');

    // Datastar SSE redirect triggers window.location assignment
    await page.waitForURL('/', { timeout: 10_000 });
    await expect(page.locator('h2:has-text("Today")')).toBeVisible();
  });

  test('unauthenticated / redirects to /login', async ({ page }) => {
    await page.context().clearCookies();
    // goto follows 302 redirect automatically
    await page.goto('/');
    // After clearing cookies, middleware does 302 → /login
    expect(page.url()).toContain('/login');
    await expect(page.locator('button:has-text("Sign in")')).toBeVisible();
  });

  test('logout clears session', async ({ page }) => {
    // Create a fresh login session (don't use the shared .auth.json session
    // so we don't invalidate it for subsequent test files)
    await page.context().clearCookies();
    await page.goto('/login');
    await page.fill('input[type="text"]', process.env.VVS_ADMIN_USER || 'admin');
    await page.fill('input[type="password"]', process.env.VVS_ADMIN_PASSWORD || 'secret');
    await page.click('button:has-text("Sign in")');
    await page.waitForURL('/', { timeout: 10_000 });

    // Logout button uses data-on:click="@post('/api/logout')" with text "Sign out"
    await page.click('button:has-text("Sign out")');
    await page.waitForURL('**/login', { timeout: 10_000 });
    await expect(page.locator('button:has-text("Sign in")')).toBeVisible();
  });
});
