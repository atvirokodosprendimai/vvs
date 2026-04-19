/**
 * Module permissions E2E tests — admin UI, role enforcement, nav hiding.
 *
 * Tests run in two contexts:
 *   - Admin context:  shared .auth.json session (set up by setup.js)
 *   - Viewer context: fresh browser context, created in beforeAll
 *
 * The viewer user is created via the admin UI at the start of the suite and
 * cleaned up automatically (no explicit teardown — test DB is ephemeral in CI).
 */
const { test, expect } = require('@playwright/test');
const path = require('path');

const AUTH_FILE = path.join(__dirname, '.auth.json');
const TS = Date.now();
const VIEWER_USER = `e2e-viewer-${TS}`;
const VIEWER_PASS = 'Viewerpass1!';

// ── Admin permissions UI ───────────────────────────────────────────────────

test.describe('Permissions page (admin)', () => {
  test('loads at /settings/permissions', async ({ page }) => {
    await page.goto('/settings/permissions');
    await expect(page.getByRole('heading', { name: 'Module Permissions' })).toBeVisible();
    await page.waitForSelector('#permissions-grid', { timeout: 8_000 });
    await expect(page.locator('#permissions-grid')).toBeVisible();
  });

  test('shows Operator and Viewer sections', async ({ page }) => {
    await page.goto('/settings/permissions');
    await page.waitForSelector('#permissions-grid', { timeout: 8_000 });
    await expect(page.locator('#permissions-grid h3:has-text("Operator")')).toBeVisible();
    await expect(page.locator('#permissions-grid h3:has-text("Viewer")')).toBeVisible();
  });

  test('grid contains all 13 modules', async ({ page }) => {
    await page.goto('/settings/permissions');
    await page.waitForSelector('#permissions-grid', { timeout: 8_000 });
    const modules = [
      'Customers', 'Tickets', 'Deals', 'Tasks', 'Contacts',
      'Invoices', 'Products', 'Payments', 'Network',
      'Email', 'Cron', 'Audit Log', 'Users',
    ];
    for (const mod of modules) {
      // Each module label appears twice (once in operator table, once in viewer table)
      await expect(page.locator(`td:has-text("${mod}")`).first()).toBeVisible();
    }
  });

  test('toggle saves — POST returns 204', async ({ page, request }) => {
    // Use Playwright request API to hit the endpoint directly
    const cookieHeader = (await page.context().cookies())
      .map(c => `${c.name}=${c.value}`)
      .join('; ');

    // Toggle operator/customers view (currently true → false → back to true)
    const resp1 = await request.post('/api/permissions/operator/customers?f=view&v=false', {
      headers: { Cookie: cookieHeader },
    });
    expect(resp1.status()).toBe(204);

    const resp2 = await request.post('/api/permissions/operator/customers?f=view&v=true', {
      headers: { Cookie: cookieHeader },
    });
    expect(resp2.status()).toBe(204);
  });

  test('savePermission rejects unknown role', async ({ page, request }) => {
    const cookieHeader = (await page.context().cookies())
      .map(c => `${c.name}=${c.value}`)
      .join('; ');
    const resp = await request.post('/api/permissions/admin/customers?f=view&v=true', {
      headers: { Cookie: cookieHeader },
    });
    expect(resp.status()).toBe(400);
  });

  test('savePermission rejects unknown module', async ({ page, request }) => {
    const cookieHeader = (await page.context().cookies())
      .map(c => `${c.name}=${c.value}`)
      .join('; ');
    const resp = await request.post('/api/permissions/operator/notamodule?f=view&v=true', {
      headers: { Cookie: cookieHeader },
    });
    expect(resp.status()).toBe(400);
  });

  test('savePermission rejects invalid value', async ({ page, request }) => {
    const cookieHeader = (await page.context().cookies())
      .map(c => `${c.name}=${c.value}`)
      .join('; ');
    const resp = await request.post('/api/permissions/operator/customers?f=view&v=yes', {
      headers: { Cookie: cookieHeader },
    });
    expect(resp.status()).toBe(400);
  });

  test('permissions link visible in sidebar for admin', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('a[href="/settings/permissions"]')).toBeVisible();
  });
});

// ── Viewer role enforcement ────────────────────────────────────────────────

test.describe('Viewer role access', () => {
  /** Shared viewer browser context — created once, reused across all tests here. */
  let viewerCtx;
  let viewerPage;

  test.beforeAll(async ({ browser }) => {
    // Step 1: Create the viewer user via admin UI
    const adminCtx = await browser.newContext({ storageState: AUTH_FILE });
    const adminPage = await adminCtx.newPage();
    await adminPage.goto('/users');
    await adminPage.waitForSelector('#user-table', { timeout: 8_000 });

    // Open the New User modal
    await adminPage.click('button:has-text("Add User")');
    await adminPage.waitForSelector('h2:has-text("New User")', { timeout: 3_000 });

    // Fill create-user form (inputs use data-bind signals, select by placeholder)
    await adminPage.fill('input[placeholder="Username"]', VIEWER_USER);
    await adminPage.fill('input[placeholder="Password"]', VIEWER_PASS);
    await adminPage.selectOption('.fixed.inset-0 select', 'viewer');
    await adminPage.click('button:has-text("Create")');

    // Wait for table to refresh with the new user
    await adminPage.waitForTimeout(1_000);
    await adminCtx.close();

    // Step 2: Login as viewer in a fresh context
    viewerCtx = await browser.newContext();
    viewerPage = await viewerCtx.newPage();
    viewerPage.setDefaultTimeout(10_000);

    await viewerPage.goto('/login');
    await viewerPage.fill('input[type="text"]', VIEWER_USER);
    await viewerPage.fill('input[type="password"]', VIEWER_PASS);
    await viewerPage.click('button:has-text("Sign in")');
    await viewerPage.waitForURL('/', { timeout: 10_000 });
  });

  test.afterAll(async () => {
    await viewerCtx?.close();
  });

  // ── Page access ─────────────────────────────────────────────────────────

  test('viewer can access /customers (CanView=true by default)', async () => {
    await viewerPage.goto('/customers');
    await expect(viewerPage.getByRole('heading', { name: 'Customers' })).toBeVisible();
  });

  test('viewer cannot access /users — 403', async () => {
    await viewerPage.goto('/users');
    const body = await viewerPage.locator('body').innerText();
    expect(body).toMatch(/forbidden|403/i);
  });

  test('viewer can access /profile — self-service exempt', async () => {
    await viewerPage.goto('/profile');
    await expect(viewerPage.locator('h1, h2').filter({ hasText: 'Profile' })).toBeVisible();
  });

  test('viewer cannot access /settings/permissions — admin-only', async () => {
    await viewerPage.goto('/settings/permissions');
    const body = await viewerPage.locator('body').innerText();
    expect(body).toMatch(/forbidden|403/i);
  });

  test('viewer read-only badge shown in sidebar', async () => {
    await viewerPage.goto('/');
    // Read-only mode indicator is shown via data-show="$_userRole === 'viewer'"
    // It may be hidden until signal fires; wait for SSE
    await viewerPage.waitForTimeout(1_500);
    const badge = viewerPage.locator('text=Read-only mode');
    await expect(badge).toBeVisible({ timeout: 5_000 });
  });

  // ── Nav hiding ──────────────────────────────────────────────────────────

  test('viewer sidebar hides Users link', async () => {
    await viewerPage.goto('/');
    await expect(viewerPage.locator('a[href="/users"]')).not.toBeVisible();
  });

  test('viewer sidebar hides Permissions link', async () => {
    await viewerPage.goto('/');
    await expect(viewerPage.locator('a[href="/settings/permissions"]')).not.toBeVisible();
  });

  test('viewer sidebar shows Customers link (view access)', async () => {
    await viewerPage.goto('/');
    // Open CRM group if collapsed
    const crmGroup = viewerPage.locator('button:has-text("CRM")');
    if (await crmGroup.isVisible()) {
      await crmGroup.click();
      await viewerPage.waitForTimeout(300);
    }
    await expect(viewerPage.locator('a[href="/customers"]')).toBeVisible();
  });

  // ── Mutation blocked ────────────────────────────────────────────────────

  test('viewer POST to /api/customers returns 403', async ({ request }) => {
    const cookieHeader = (await viewerPage.context().cookies())
      .map(c => `${c.name}=${c.value}`)
      .join('; ');
    const resp = await request.post('/api/customers', {
      headers: { Cookie: cookieHeader, 'Content-Type': 'application/json' },
      data: { companyName: 'Should be blocked' },
    });
    expect(resp.status()).toBe(403);
  });

  // ── Login page accessible without auth ──────────────────────────────────

  test('unauthenticated user can reach /login (not module-gated)', async ({ browser }) => {
    const ctx = await browser.newContext();
    const pg = await ctx.newPage();
    await pg.goto('/login');
    await expect(pg.locator('button:has-text("Sign in")')).toBeVisible();
    await ctx.close();
  });
});
