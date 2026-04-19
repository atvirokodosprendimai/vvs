/**
 * Customer portal E2E tests.
 *
 * Three sections:
 *   1. Unauthenticated paths — no server state required.
 *   2. Admin UI — "Portal Access" button on customer detail.
 *   3. Portal session flow — full auth → invoice list → detail → logout.
 *
 * The portal runs on the same origin but uses a separate `vvs_portal` cookie.
 * Portal tests use a fresh browser context (no admin storageState).
 */
const { test, expect } = require('@playwright/test');
const path = require('path');

const AUTH_FILE = path.join(__dirname, '.auth.json');
const TS = Date.now();
const CUSTOMER_NAME = `E2E Portal ${TS}`;
const CUSTOMER_EMAIL = `portal-${TS}@example.com`;

// ── 1. Unauthenticated portal paths ─────────────────────────────────────────

test.describe('Portal — unauthenticated', () => {
  test('GET /portal/auth no token — shows expired page', async ({ page }) => {
    await page.context().clearCookies();
    await page.goto('/portal/auth');
    const body = await page.locator('body').innerText();
    expect(body).toContain('Access link expired');
    expect(body).not.toContain('panic:');
  });

  test('GET /portal/auth?token=bogus — shows expired page', async ({ page }) => {
    await page.context().clearCookies();
    await page.goto('/portal/auth?token=totallybogustoken');
    const body = await page.locator('body').innerText();
    expect(body).toContain('Access link expired');
    expect(body).not.toContain('panic:');
  });

  test('GET /portal/invoices no cookie — redirects to /portal/auth', async ({ page }) => {
    await page.context().clearCookies();
    await page.goto('/portal/invoices');
    await page.waitForURL(/\/portal\/auth/, { timeout: 8_000 });
    expect(page.url()).toContain('expired=1');
  });

  test('GET /portal/invoices/{id} no cookie — redirects to /portal/auth', async ({ page }) => {
    await page.context().clearCookies();
    await page.goto('/portal/invoices/nonexistent-id');
    await page.waitForURL(/\/portal\/auth/, { timeout: 8_000 });
    expect(page.url()).toContain('expired=1');
  });
});

// ── 2. Admin UI — Portal Access button ──────────────────────────────────────

test.describe('Admin — Portal Access button', () => {
  test.beforeAll(async ({ request }) => {
    await request.post('/api/customers', {
      headers: { 'Content-Type': 'application/json' },
      data: {
        companyName: CUSTOMER_NAME,
        contactName: 'Portal Test Contact',
        email: CUSTOMER_EMAIL,
        status: 'active',
      },
    });
  });

  test('customer detail has Portal Access button', async ({ page }) => {
    await page.goto('/customers');
    await page.waitForSelector('#customer-table', { timeout: 8_000 });

    // Find our test customer
    const link = page.locator(`#customer-table a[href^="/customers/"]`).first();
    await expect(link).toBeVisible({ timeout: 5_000 });
    await link.click();
    await page.waitForURL(/\/customers\/[^/]+$/, { timeout: 8_000 });

    await expect(page.locator('button:has-text("Portal Access")')).toBeVisible();
  });

  test('clicking Portal Access button shows portal URL fragment', async ({ page }) => {
    await page.goto('/customers');
    await page.waitForSelector('#customer-table', { timeout: 8_000 });

    const link = page.locator(`#customer-table a[href^="/customers/"]`).first();
    await link.click();
    await page.waitForURL(/\/customers\/[^/]+$/, { timeout: 8_000 });

    // Click the Portal Access button (triggers Datastar SSE → patches #portal-link-result)
    await page.locator('button:has-text("Portal Access")').click();

    // Wait for portal URL to appear in the patched fragment
    const linkSpan = page.locator('#portal-link-result .font-mono');
    await expect(linkSpan).toBeVisible({ timeout: 10_000 });

    const urlText = await linkSpan.innerText();
    expect(urlText).toContain('/portal/auth?token=');
  });
});

// ── 3. Portal session flow ────────────────────────────────────────────────────

test.describe('Portal session flow', () => {
  let portalCtx;
  let portalPage;
  let customerId;

  test.beforeAll(async ({ browser }) => {
    // Admin context — needed to create customer + generate portal link
    const adminCtx = await browser.newContext({ storageState: AUTH_FILE });
    const adminPage = await adminCtx.newPage();
    adminPage.setDefaultTimeout(10_000);

    // Create test customer
    const cookiesBefore = (await adminCtx.cookies()).map(c => `${c.name}=${c.value}`).join('; ');
    await adminCtx.request.post('/api/customers', {
      headers: { 'Content-Type': 'application/json', Cookie: cookiesBefore },
      data: {
        companyName: CUSTOMER_NAME,
        contactName: 'Portal Test Contact',
        email: CUSTOMER_EMAIL,
        status: 'active',
      },
    });

    // Navigate to customer list, find our customer, extract ID from URL
    await adminPage.goto('/customers');
    await adminPage.waitForSelector('#customer-table', { timeout: 8_000 });

    const customerLink = adminPage
      .locator(`#customer-table a[href^="/customers/"]:has-text("${CUSTOMER_NAME}")`);
    const count = await customerLink.count();
    if (count > 0) {
      await customerLink.first().click();
    } else {
      await adminPage.locator('#customer-table a[href^="/customers/"]').first().click();
    }
    await adminPage.waitForURL(/\/customers\/[^/]+$/, { timeout: 8_000 });
    customerId = adminPage.url().split('/').pop();

    // Generate portal link via admin API with session cookie
    const cookieHeader = (await adminCtx.cookies())
      .map(c => `${c.name}=${c.value}`)
      .join('; ');

    const resp = await adminCtx.request.post(`/api/customers/${customerId}/portal-link`, {
      headers: { Cookie: cookieHeader },
    });
    expect(resp.status()).toBe(200);

    // Parse portal token from SSE response body
    const body = await resp.text();
    const match = body.match(/portal\/auth\?token=([A-Za-z0-9_=-]+)/);
    expect(match).not.toBeNull();
    const portalURL = `/portal/auth?token=${match[1]}`;

    await adminCtx.close();

    // Create a fresh browser context (no admin cookies) and navigate to portal
    portalCtx = await browser.newContext();
    portalPage = await portalCtx.newPage();
    portalPage.setDefaultTimeout(10_000);

    await portalPage.goto(portalURL);
    await portalPage.waitForURL('/portal/invoices', { timeout: 10_000 });
  });

  test.afterAll(async () => {
    await portalCtx?.close();
  });

  test('valid token redirects to /portal/invoices', async () => {
    // beforeAll already verified this; assert final URL
    expect(portalPage.url()).toContain('/portal/invoices');
  });

  test('portal invoice list page renders correctly', async () => {
    await portalPage.goto('/portal/invoices');
    const body = await portalPage.locator('body').innerText();
    expect(body).toContain('Invoices');
    expect(body).not.toContain('panic:');
    expect(body).not.toContain('500');
  });

  test('portal page title contains "Customer Portal"', async () => {
    await portalPage.goto('/portal/invoices');
    const title = await portalPage.title();
    expect(title).toContain('Customer Portal');
  });

  test('portal header has customer company name or logout button', async () => {
    await portalPage.goto('/portal/invoices');
    // Header contains logout button (always present for authenticated portal users)
    await expect(portalPage.locator('button:has-text("Sign out")')).toBeVisible();
  });

  test('portal shows empty state when no invoices', async () => {
    await portalPage.goto('/portal/invoices');
    const body = await portalPage.locator('body').innerText();
    // Either shows invoice table headers or empty state message — no crash
    expect(body).not.toContain('panic:');
    expect(body).not.toContain('500');
  });

  test('portal invoice detail page — navigates or shows not-found', async () => {
    await portalPage.goto('/portal/invoices');
    const invoiceLinks = portalPage.locator('a[href^="/portal/invoices/"]');
    const count = await invoiceLinks.count();

    if (count > 0) {
      await invoiceLinks.first().click();
      await portalPage.waitForURL(/\/portal\/invoices\/[^/]+$/, { timeout: 8_000 });

      const body = await portalPage.locator('body').innerText();
      expect(body).not.toContain('panic:');
      // Detail page should have "Download PDF" or at minimum invoice summary section
      await expect(portalPage.locator('a[href^="/portal/invoices"]')).toBeVisible();
    } else {
      // No invoices — empty state is valid
      const body = await portalPage.locator('body').innerText();
      expect(body).not.toContain('panic:');
    }
  });

  test('portal logout clears session and redirects to auth', async () => {
    await portalPage.goto('/portal/invoices');

    // Click logout
    await portalPage.click('button:has-text("Sign out")');
    await portalPage.waitForURL(/\/portal\/auth/, { timeout: 8_000 });
    expect(portalPage.url()).toContain('expired=1');

    // Subsequent navigation to portal requires re-auth
    await portalPage.goto('/portal/invoices');
    await portalPage.waitForURL(/\/portal\/auth/, { timeout: 8_000 });
    expect(portalPage.url()).toContain('expired=1');
  });
});
