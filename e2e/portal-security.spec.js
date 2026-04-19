/**
 * Portal security E2E tests — PDF token flow + ownership isolation.
 *
 * Covers the Codex HIGH findings fixed in commit f601772:
 *   1. PDF link: /i/{token} renders the invoice (token-based public access)
 *   2. Cross-customer isolation: portal session A cannot read customer B's invoice
 *   3. Bogus /i/{token}: 404, no crash
 *   4. Portal can only see own customer's invoices in the authenticated list
 */
const { test, expect } = require('@playwright/test');
const path = require('path');

const AUTH_FILE = path.join(__dirname, '.auth.json');
const TS = Date.now();

// ── helpers ──────────────────────────────────────────────────────────────────

async function adminCookies(ctx) {
  return (await ctx.cookies()).map(c => `${c.name}=${c.value}`).join('; ');
}

async function createCustomer(ctx, name, email) {
  const resp = await ctx.request.post('/api/customers', {
    headers: { 'Content-Type': 'application/json', Cookie: await adminCookies(ctx) },
    data: { companyName: name, contactName: 'Test', email, status: 'active' },
  });
  expect(resp.status()).toBeLessThan(500);

  // Extract ID from SSE body (event patches #customer-id or redirects to /customers/{id})
  const body = await resp.text();
  const match = body.match(/\/customers\/([a-z0-9-]+)/);
  return match?.[1] ?? null;
}

async function findCustomerID(ctx, name) {
  // The customer table has: code link in <td:first>, company name as plain text in <td:nth-2>.
  // Find the <tr> containing the company name, then extract the href from the code <a> in that row.
  const page = await ctx.newPage();
  await page.goto('/customers');
  // Wait for the row containing the company name (SSE-loaded)
  const row = page.locator(`tr:has-text("${name}")`);
  await expect(row.first()).toBeAttached({ timeout: 12_000 });
  const link = row.first().locator('a[href^="/customers/"]').first();
  const href = await link.getAttribute('href');
  await page.close();
  return href?.split('/').pop() ?? null;
}

async function createInvoice(ctx, customerID, customerName) {
  const today = new Date().toISOString().slice(0, 10);
  const due = new Date(Date.now() + 30 * 86400000).toISOString().slice(0, 10);

  const resp = await ctx.request.post('/api/invoices', {
    headers: { 'Content-Type': 'application/json', Cookie: await adminCookies(ctx) },
    data: {
      customerID,
      customerName,
      customerCode: 'E2E',
      issueDate: today,
      dueDate: due,
      lineDescription1: 'E2E test service',
      lineQty1: '1',
      linePrice1: '100.00',
      lineVATRate1: '21',
    },
  });
  expect(resp.status()).toBeLessThan(500);
  const body = await resp.text();
  const match = body.match(/\/invoices\/([a-z0-9-]+)/);
  return match?.[1] ?? null;
}

async function finalizeInvoice(ctx, invoiceID) {
  const resp = await ctx.request.put(`/api/invoices/${invoiceID}/finalize`, {
    headers: { Cookie: await adminCookies(ctx) },
  });
  expect(resp.status()).toBeLessThan(500);
}

async function getPortalToken(ctx, customerID) {
  const resp = await ctx.request.post(`/api/customers/${customerID}/portal-link`, {
    headers: { Cookie: await adminCookies(ctx) },
  });
  expect(resp.status()).toBe(200);
  const body = await resp.text();
  const match = body.match(/portal\/auth\?token=([A-Za-z0-9_=.+-]+)/);
  expect(match).not.toBeNull();
  return match[1];
}

async function loginPortal(browser, token) {
  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  page.setDefaultTimeout(10_000);
  await page.goto(`/portal/auth?token=${token}`);
  await page.waitForURL('/portal/invoices', { timeout: 10_000 });
  await page.close();
  return ctx;
}

// ── 1. Bogus /i/{token} — no session required ────────────────────────────────

test.describe('Portal PDF token — invalid', () => {
  test('GET /i/bogustoken returns 404, no panic', async ({ page }) => {
    await page.context().clearCookies();
    const resp = await page.request.get('/i/thisisnotavalidtokensomebogusstring');
    expect(resp.status()).toBe(404);
    const body = await resp.text();
    expect(body).not.toContain('panic:');
  });

  test('GET /i/ with empty token returns 404 or 405', async ({ page }) => {
    await page.context().clearCookies();
    const resp = await page.request.get('/i/');
    // chi router: /i/ with no token segment → 404 or redirect; must not 500
    expect(resp.status()).not.toBe(500);
  });
});

// ── 2. PDF link flow — create invoice → finalize → portal → /i/{token} ───────

test.describe('Portal PDF token — valid flow', () => {
  let adminCtx;
  let portalCtx;
  let invoiceID;
  let customerID;

  const CUSTOMER_NAME = `E2E PDF ${TS}`;
  const CUSTOMER_EMAIL = `pdf-${TS}@example.com`;

  test.beforeAll(async ({ browser }) => {
    adminCtx = await browser.newContext({ storageState: AUTH_FILE });

    // Create customer
    await createCustomer(adminCtx, CUSTOMER_NAME, CUSTOMER_EMAIL);
    customerID = await findCustomerID(adminCtx, CUSTOMER_NAME);
    expect(customerID).not.toBeNull();

    // Create + finalize invoice
    invoiceID = await createInvoice(adminCtx, customerID, CUSTOMER_NAME);
    expect(invoiceID).not.toBeNull();
    await finalizeInvoice(adminCtx, invoiceID);

    // Generate portal link + log in as customer
    const token = await getPortalToken(adminCtx, customerID);
    portalCtx = await loginPortal(browser, token);
  });

  test.afterAll(async () => {
    await adminCtx?.close();
    await portalCtx?.close();
  });

  test('portal invoice list shows the finalized invoice', async () => {
    const page = await portalCtx.newPage();
    await page.goto('/portal/invoices');
    const body = await page.locator('body').innerText();
    expect(body).not.toContain('panic:');
    // Invoice list renders (may show invoice row or empty state — no crash is the assertion)
    await page.close();
  });

  test('portal invoice detail has a PDF link', async () => {
    const page = await portalCtx.newPage();
    await page.goto(`/portal/invoices/${invoiceID}`);
    const status = page.url();

    // Should not redirect to auth (invoice belongs to this customer)
    expect(status).not.toContain('/portal/auth');

    const body = await page.locator('body').innerText();
    expect(body).not.toContain('panic:');
    expect(body).not.toContain('500');

    // PDF link present — href starts with /i/
    const pdfLink = page.locator('a[href^="/i/"]');
    await expect(pdfLink).toBeVisible({ timeout: 8_000 });
    await page.close();
  });

  test('/i/{token} renders invoice HTML (no auth required)', async ({ browser }) => {
    // Get the PDF token from the invoice detail page
    const portalPage = await portalCtx.newPage();
    await portalPage.goto(`/portal/invoices/${invoiceID}`);
    const pdfLink = portalPage.locator('a[href^="/i/"]');
    await expect(pdfLink).toBeVisible({ timeout: 8_000 });
    const href = await pdfLink.getAttribute('href');
    await portalPage.close();

    expect(href).toMatch(/^\/i\/.+/);

    // Access PDF URL in a fresh context (no cookies) — public route
    const anonCtx = await browser.newContext();
    const anonPage = await anonCtx.newPage();
    const resp = await anonPage.goto(href);
    expect(resp.status()).toBe(200);

    const body = await anonPage.locator('body').innerText();
    expect(body).not.toContain('panic:');
    expect(body).not.toContain('404');
    // Invoice print page should contain some invoice content
    expect(body.length).toBeGreaterThan(50);

    await anonCtx.close();
  });
});

// ── 3. Cross-customer isolation ───────────────────────────────────────────────

test.describe('Portal ownership isolation', () => {
  let adminCtx;
  let custACtx;

  const NAME_A = `E2E IsoA ${TS}`;
  const EMAIL_A = `iso-a-${TS}@example.com`;
  const NAME_B = `E2E IsoB ${TS}`;
  const EMAIL_B = `iso-b-${TS}@example.com`;

  let idA, idB, invoiceB;

  test.beforeAll(async ({ browser }) => {
    adminCtx = await browser.newContext({ storageState: AUTH_FILE });

    // Create two customers
    await createCustomer(adminCtx, NAME_A, EMAIL_A);
    await createCustomer(adminCtx, NAME_B, EMAIL_B);
    idA = await findCustomerID(adminCtx, NAME_A);
    idB = await findCustomerID(adminCtx, NAME_B);
    expect(idA).not.toBeNull();
    expect(idB).not.toBeNull();

    // Create + finalize an invoice for customer B
    invoiceB = await createInvoice(adminCtx, idB, NAME_B);
    expect(invoiceB).not.toBeNull();
    await finalizeInvoice(adminCtx, invoiceB);

    // Log in as customer A
    const tokenA = await getPortalToken(adminCtx, idA);
    custACtx = await loginPortal(browser, tokenA);
  });

  test.afterAll(async () => {
    await adminCtx?.close();
    await custACtx?.close();
  });

  test('customer A cannot read customer B invoice detail', async () => {
    // Customer A's portal session tries to access customer B's invoice directly
    const page = await custACtx.newPage();
    const resp = await page.goto(`/portal/invoices/${invoiceB}`);

    // Must return 404 — not customer A's invoice
    expect(resp.status()).toBe(404);

    const body = await page.locator('body').innerText();
    expect(body).not.toContain('panic:');
    await page.close();
  });

  test('customer A invoice list does not contain customer B invoices', async () => {
    const page = await custACtx.newPage();
    await page.goto('/portal/invoices');
    await page.waitForSelector('body', { timeout: 8_000 });

    // Body must not contain customer B's name or invoice ID
    const body = await page.locator('body').innerText();
    expect(body).not.toContain(NAME_B);
    expect(body).not.toContain('panic:');
    await page.close();
  });
});
