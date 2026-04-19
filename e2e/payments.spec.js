/**
 * Payment Import tests — upload form, preview page.
 */
const { test, expect } = require('@playwright/test');
const path = require('path');
const fs = require('fs');
const os = require('os');

// Minimal SEPA CSV fixture written to a temp file
function createTestCSV() {
  const content = [
    'Date;Amount;Reference;Description',
    '2026-01-15;100.00;INV-TEST-001;Test payment',
    '2026-01-16;250.50;INV-TEST-002;Another payment',
  ].join('\n');
  const tmpFile = path.join(os.tmpdir(), `sepa-test-${Date.now()}.csv`);
  fs.writeFileSync(tmpFile, content, 'utf8');
  return tmpFile;
}

test.describe('Payment Import', () => {
  test('import page loads', async ({ page }) => {
    await page.goto('/payments/import');
    await expect(page.getByRole('heading', { name: 'Payment Import' })).toBeVisible();
  });

  test('import page has no server errors', async ({ page }) => {
    await page.goto('/payments/import');
    const bodyText = await page.locator('body').innerText();
    expect(bodyText).not.toContain('panic:');
    expect(bodyText).not.toContain('500');
  });

  test('CSV file input present', async ({ page }) => {
    await page.goto('/payments/import');
    await expect(page.locator('#csv_file')).toBeVisible();
  });

  test('Preview Matches button present', async ({ page }) => {
    await page.goto('/payments/import');
    await expect(page.locator('button:has-text("Preview Matches")')).toBeVisible();
  });

  test('upload CSV shows preview page', async ({ page }) => {
    const csvFile = createTestCSV();
    try {
      await page.goto('/payments/import');
      await page.locator('#csv_file').setInputFiles(csvFile);
      await page.click('button:has-text("Preview Matches")');

      // After POST to /payments/import/preview, should still be on import page
      await page.waitForURL(/\/payments\/import/, { timeout: 10_000 });

      const bodyText = await page.locator('body').innerText();
      expect(bodyText).not.toContain('panic:');
      expect(bodyText).not.toContain('500');
    } finally {
      fs.unlinkSync(csvFile);
    }
  });

  test('preview page has Upload Another File form', async ({ page }) => {
    const csvFile = createTestCSV();
    try {
      await page.goto('/payments/import');
      await page.locator('#csv_file').setInputFiles(csvFile);
      await page.click('button:has-text("Preview Matches")');
      await page.waitForURL(/\/payments\/import/, { timeout: 10_000 });

      // Preview page shows the upload-again form
      await expect(page.locator('#csv_file')).toBeVisible({ timeout: 5_000 });
    } finally {
      fs.unlinkSync(csvFile);
    }
  });

  test('payments nav link accessible', async ({ page }) => {
    await page.goto('/');
    // Finance group — find the Payments link
    const paymentsLink = page.locator('a[href="/payments/import"]');
    // May be in a collapsed nav group; just confirm it exists in DOM
    await expect(paymentsLink).toHaveCount(1);
  });
});
