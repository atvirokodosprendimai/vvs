/**
 * System module tests — Users, Email, Cron, Audit Log.
 */
const { test, expect } = require('@playwright/test');

// ── Users ──────────────────────────────────────────────────────────────────

test.describe('Users', () => {
  test('list page loads', async ({ page }) => {
    await page.goto('/users');
    await expect(page.getByRole('heading', { name: 'Users' })).toBeVisible();
    await expect(page.locator('button:has-text("Add User")')).toBeVisible();
  });

  test('user table loads via SSE', async ({ page }) => {
    await page.goto('/users');
    await page.waitForSelector('#user-table', { timeout: 8_000 });
    const bodyText = await page.locator('body').innerText();
    expect(bodyText).not.toContain('panic:');
  });

  test('user table has no server errors', async ({ page }) => {
    await page.goto('/users');
    await page.waitForSelector('#user-table', { timeout: 8_000 });
    const bodyText = await page.locator('body').innerText();
    expect(bodyText).not.toContain('500');
  });

  test('Add User button opens modal', async ({ page }) => {
    await page.goto('/users');
    await page.waitForSelector('#user-table', { timeout: 8_000 });
    await page.click('button:has-text("Add User")');
    await expect(page.locator('h2:has-text("New User")')).toBeVisible({ timeout: 3_000 });
  });

  test('create user modal has required fields', async ({ page }) => {
    await page.goto('/users');
    await page.waitForSelector('#user-table', { timeout: 8_000 });
    await page.click('button:has-text("Add User")');
    await expect(page.locator('input[placeholder="Username"]')).toBeVisible();
    await expect(page.locator('input[placeholder="Password"]')).toBeVisible();
    // Role select inside create modal
    await expect(page.locator('[data-bind\\:new-role]')).toBeVisible();
  });

  test('user table shows Full Name and Division columns', async ({ page }) => {
    await page.goto('/users');
    await page.waitForSelector('#user-table', { timeout: 8_000 });
    const headers = await page.locator('#user-table th').allInnerTexts();
    expect(headers.join(' ')).toMatch(/Full Name/i);
    expect(headers.join(' ')).toMatch(/Division/i);
  });

  test('Edit button opens edit modal', async ({ page }) => {
    await page.goto('/users');
    await page.waitForSelector('#user-table', { timeout: 8_000 });
    await page.locator('#user-table button:has-text("Edit")').first().click();
    await expect(page.locator('h2:has-text("Edit User")')).toBeVisible({ timeout: 3_000 });
    await expect(page.locator('input[placeholder="Full name"]')).toBeVisible();
    await expect(page.locator('input[placeholder="Division / department"]')).toBeVisible();
  });
});

// ── Email ──────────────────────────────────────────────────────────────────

test.describe('Email', () => {
  test('inbox page loads', async ({ page }) => {
    await page.goto('/emails');
    await expect(page.getByRole('heading', { name: 'Email' })).toBeVisible();
  });

  test('email inbox container present', async ({ page }) => {
    await page.goto('/emails');
    await page.waitForSelector('#email-inbox', { timeout: 8_000 });
    const bodyText = await page.locator('body').innerText();
    expect(bodyText).not.toContain('panic:');
  });

  test('email page has no server errors', async ({ page }) => {
    await page.goto('/emails');
    await page.waitForSelector('#email-inbox', { timeout: 8_000 });
    const bodyText = await page.locator('body').innerText();
    expect(bodyText).not.toContain('500');
  });

  test('email settings page loads', async ({ page }) => {
    await page.goto('/emails/settings');
    const bodyText = await page.locator('body').innerText();
    expect(bodyText).not.toContain('panic:');
    expect(bodyText).not.toContain('500');
  });
});

// ── Cron ───────────────────────────────────────────────────────────────────

test.describe('Cron', () => {
  test('list page loads', async ({ page }) => {
    await page.goto('/cron');
    await expect(page.getByRole('heading', { name: 'Cron Jobs' })).toBeVisible();
  });

  test('cron table loads via SSE', async ({ page }) => {
    await page.goto('/cron');
    await page.waitForSelector('#cron-table', { timeout: 8_000 });
    const bodyText = await page.locator('body').innerText();
    expect(bodyText).not.toContain('panic:');
  });

  test('cron page has no server errors', async ({ page }) => {
    await page.goto('/cron');
    await page.waitForSelector('#cron-table', { timeout: 8_000 });
    const bodyText = await page.locator('body').innerText();
    expect(bodyText).not.toContain('500');
  });

  test('Add Job button opens modal', async ({ page }) => {
    await page.goto('/cron');
    await page.waitForSelector('#cron-table', { timeout: 8_000 });
    await page.click('button:has-text("Add Job")');
    await expect(page.locator('h3:has-text("Add Cron Job")')).toBeVisible({ timeout: 3_000 });
  });
});

// ── Audit Log ──────────────────────────────────────────────────────────────

test.describe('Audit Log', () => {
  test('list page loads', async ({ page }) => {
    await page.goto('/audit-logs');
    await expect(page.getByRole('heading', { name: 'Audit Log' })).toBeVisible();
  });

  test('audit log table loads via SSE', async ({ page }) => {
    await page.goto('/audit-logs');
    await page.waitForSelector('#audit-logs-page', { timeout: 8_000 });
    const bodyText = await page.locator('body').innerText();
    expect(bodyText).not.toContain('panic:');
  });

  test('audit log page has no server errors', async ({ page }) => {
    await page.goto('/audit-logs');
    await page.waitForSelector('#audit-logs-page', { timeout: 8_000 });
    const bodyText = await page.locator('body').innerText();
    expect(bodyText).not.toContain('500');
  });

  test('resource filter buttons present', async ({ page }) => {
    await page.goto('/audit-logs');
    await expect(page.locator('button:has-text("All")')).toBeVisible();
    await expect(page.locator('button:has-text("Customer")')).toBeVisible();
    await expect(page.locator('button:has-text("Invoice")')).toBeVisible();
  });

  test('filter by customer does not crash', async ({ page }) => {
    await page.goto('/audit-logs');
    await page.waitForSelector('#audit-logs-page', { timeout: 8_000 });
    await page.click('button:has-text("Customer")');
    await page.waitForTimeout(500);
    const bodyText = await page.locator('body').innerText();
    expect(bodyText).not.toContain('panic:');
  });
});
