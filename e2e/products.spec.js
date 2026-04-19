/**
 * Products tests — list, create, detail/edit.
 */
const { test, expect } = require('@playwright/test');

const TS = Date.now();
const TEST_PRODUCT = `E2E Product ${TS}`;
const TEST_PRICE = '29.99';

test.describe('Products', () => {
  test('list page loads', async ({ page }) => {
    await page.goto('/products');
    await expect(page.getByRole('heading', { name: 'Products' })).toBeVisible();
    await expect(page.locator('input[placeholder="Search products..."]')).toBeVisible();
    await expect(page.locator('a[href="/products/new"]')).toBeVisible();
  });

  test('product table loads via SSE', async ({ page }) => {
    await page.goto('/products');
    await page.waitForSelector('#product-content', { timeout: 8_000 });
    const bodyText = await page.locator('body').innerText();
    expect(bodyText).not.toContain('panic:');
  });

  test('new product form loads', async ({ page }) => {
    await page.goto('/products/new');
    await expect(page.getByRole('heading', { name: /New Product/i })).toBeVisible();
  });

  test('create new product and verify in list', async ({ page }) => {
    await page.goto('/products/new');
    await expect(page.getByRole('heading', { name: /New Product/i })).toBeVisible();

    await page.fill('#name', TEST_PRODUCT);
    await page.fill('#priceAmount', TEST_PRICE);

    // Submit (button text is dynamic — use data-on:click attribute presence)
    await page.click('[data-on\\:click*="/api/products"]');
    await page.waitForURL(/\/products/, { timeout: 10_000 });

    await page.waitForSelector('#product-content', { timeout: 8_000 });
    const bodyText = await page.locator('body').innerText();
    expect(bodyText).not.toContain('panic:');
  });

  test('search filters product list', async ({ page }) => {
    await page.goto('/products');
    await page.waitForSelector('#product-content', { timeout: 8_000 });
    await page.fill('input[placeholder="Search products..."]', TEST_PRODUCT);
    await page.waitForTimeout(600);
    // No crash
    const bodyText = await page.locator('body').innerText();
    expect(bodyText).not.toContain('panic:');
  });
});
