/**
 * Customer detail page tests — CRM tabs, notes, status actions.
 *
 * Requires at least one customer in the DB (created via API in beforeAll).
 */
const { test, expect } = require('@playwright/test');

const TS = Date.now();
const TEST_CUSTOMER = `E2E Detail ${TS}`;

let customerId;

test.describe('Customer Detail', () => {
  test.beforeAll(async ({ request }) => {
    // Create a test customer directly via API
    const resp = await request.post('/api/customers', {
      data: {
        companyName: TEST_CUSTOMER,
        contactName: 'Test Contact',
        email: `detail-${TS}@example.com`,
        status: 'active',
      },
    });
    expect(resp.status()).toBe(200);
    const body = await resp.json();
    // The API may return the customer inside a wrapper; extract ID
    customerId = body.id || (body.customer && body.customer.id);
    // Fallback: navigate to customer list and grab ID from URL
    if (!customerId) {
      // We'll discover the customer by listing — not ideal but workable
      const listResp = await request.get('/api/customers?search=' + encodeURIComponent(TEST_CUSTOMER));
      if (listResp.status() === 200) {
        const listBody = await listResp.json();
        customerId = listBody?.customers?.[0]?.id;
      }
    }
  });

  test('customer list page loads and has link', async ({ page }) => {
    await page.goto('/customers');
    await page.waitForSelector('#customer-table', { timeout: 8_000 });
    const bodyText = await page.locator('body').innerText();
    expect(bodyText).not.toContain('panic:');
  });

  test('customer detail page loads', async ({ page }) => {
    // Navigate via list to get a valid customer ID
    await page.goto('/customers');
    await page.waitForSelector('#customer-table', { timeout: 8_000 });

    // Click the first customer row link
    const firstLink = page.locator('#customer-table a[href^="/customers/"]').first();
    await expect(firstLink).toBeVisible({ timeout: 5_000 });
    await firstLink.click();

    // Should land on detail page
    await page.waitForURL(/\/customers\/[^/]+$/, { timeout: 8_000 });
    const bodyText = await page.locator('body').innerText();
    expect(bodyText).not.toContain('panic:');
    expect(bodyText).not.toContain('500');
  });

  test('CRM tab bar renders', async ({ page }) => {
    await page.goto('/customers');
    await page.waitForSelector('#customer-table', { timeout: 8_000 });
    await page.locator('#customer-table a[href^="/customers/"]').first().click();
    await page.waitForURL(/\/customers\/[^/]+$/, { timeout: 8_000 });

    await expect(page.locator('#crm-tab-bar')).toBeVisible({ timeout: 5_000 });
  });

  test('Tickets tab is default active tab', async ({ page }) => {
    await page.goto('/customers');
    await page.waitForSelector('#customer-table', { timeout: 8_000 });
    await page.locator('#customer-table a[href^="/customers/"]').first().click();
    await page.waitForURL(/\/customers\/[^/]+$/, { timeout: 8_000 });

    await page.waitForSelector('#crm-tab-bar', { timeout: 5_000 });
    // Default tab is 'tickets'
    const ticketsTab = page.locator('#crm-tab-bar button:has-text("Tickets")');
    await expect(ticketsTab).toBeVisible();
    await expect(ticketsTab).toHaveClass(/text-amber-500/);
  });

  test('switching to Services tab works', async ({ page }) => {
    await page.goto('/customers');
    await page.waitForSelector('#customer-table', { timeout: 8_000 });
    await page.locator('#customer-table a[href^="/customers/"]').first().click();
    await page.waitForURL(/\/customers\/[^/]+$/, { timeout: 8_000 });

    await page.waitForSelector('#crm-tab-bar', { timeout: 5_000 });
    await page.locator('#crm-tab-bar button:has-text("Services")').click();

    // No crash
    const bodyText = await page.locator('body').innerText();
    expect(bodyText).not.toContain('panic:');
  });

  test('switching to Contacts tab works', async ({ page }) => {
    await page.goto('/customers');
    await page.waitForSelector('#customer-table', { timeout: 8_000 });
    await page.locator('#customer-table a[href^="/customers/"]').first().click();
    await page.waitForURL(/\/customers\/[^/]+$/, { timeout: 8_000 });

    await page.waitForSelector('#crm-tab-bar', { timeout: 5_000 });
    await page.locator('#crm-tab-bar button:has-text("Contacts")').click();

    const bodyText = await page.locator('body').innerText();
    expect(bodyText).not.toContain('panic:');
  });

  test('switching to Invoices tab works', async ({ page }) => {
    await page.goto('/customers');
    await page.waitForSelector('#customer-table', { timeout: 8_000 });
    await page.locator('#customer-table a[href^="/customers/"]').first().click();
    await page.waitForURL(/\/customers\/[^/]+$/, { timeout: 8_000 });

    await page.waitForSelector('#crm-tab-bar', { timeout: 5_000 });
    await page.locator('#crm-tab-bar button:has-text("Invoices")').click();

    const bodyText = await page.locator('body').innerText();
    expect(bodyText).not.toContain('panic:');
  });

  test('switching to Audit Log tab works', async ({ page }) => {
    await page.goto('/customers');
    await page.waitForSelector('#customer-table', { timeout: 8_000 });
    await page.locator('#customer-table a[href^="/customers/"]').first().click();
    await page.waitForURL(/\/customers\/[^/]+$/, { timeout: 8_000 });

    await page.waitForSelector('#crm-tab-bar', { timeout: 5_000 });
    await page.locator('#crm-tab-bar button:has-text("Audit Log")').click();

    await page.waitForTimeout(500);
    const bodyText = await page.locator('body').innerText();
    expect(bodyText).not.toContain('panic:');
  });

  test('customer new form loads', async ({ page }) => {
    await page.goto('/customers/new');
    await expect(page.getByRole('heading', { name: /New Customer/i })).toBeVisible();
    await expect(page.locator('button:has-text("Create Customer")')).toBeVisible();
  });

  test('customer edit form loads', async ({ page }) => {
    await page.goto('/customers');
    await page.waitForSelector('#customer-table', { timeout: 8_000 });
    const editLink = page.locator('#customer-table a[href$="/edit"]').first();
    await expect(editLink).toBeVisible({ timeout: 5_000 });
    await editLink.click();
    await page.waitForURL(/\/customers\/[^/]+\/edit/, { timeout: 8_000 });
    const bodyText = await page.locator('body').innerText();
    expect(bodyText).not.toContain('panic:');
    expect(bodyText).not.toContain('500');
  });
});
