/**
 * IPTV module tests — Channels, Packages, Subscriptions, STBs, Stacks.
 *
 * Tests run serially. Subscription tests depend on:
 *   - A customer existing (created by customers.spec.js which runs first alphabetically)
 *   - A package existing (created in the Packages describe below)
 *
 * STB registration depends on a customer existing.
 * Stack create tests are skipped — requires live Docker swarm infrastructure.
 */
const { test, expect } = require('@playwright/test');

const TS = Date.now();

// ── Helpers ───────────────────────────────────────────────────────────────────

async function waitForTable(page, tableId) {
  await page.waitForSelector(`#${tableId}`, { timeout: 8_000 });
  const text = await page.locator('body').innerText();
  expect(text).not.toContain('panic:');
  expect(text).not.toContain('500 Internal');
}

// ── Channels ──────────────────────────────────────────────────────────────────

test.describe('IPTV Channels', () => {
  const CH_NAME = `E2E Channel ${TS}`;

  test('channels page loads', async ({ page }) => {
    await page.goto('/iptv/channels');
    await expect(page.getByRole('heading', { name: 'IPTV — Channels' })).toBeVisible();
    await expect(page.locator('button:has-text("+ Channel")')).toBeVisible();
  });

  test('channel table loads via SSE', async ({ page }) => {
    await page.goto('/iptv/channels');
    await waitForTable(page, 'iptv-channel-table');
  });

  test('create channel via modal', async ({ page }) => {
    await page.goto('/iptv/channels');
    await waitForTable(page, 'iptv-channel-table');

    await page.click('button:has-text("+ Channel")');
    await expect(page.locator('h2:has-text("New Channel")')).toBeVisible();

    await page.fill('[placeholder="Name"]', CH_NAME);
    await page.fill('[placeholder="Category (e.g. National)"]', 'E2E');
    await page.fill('[placeholder="Stream URL"]', 'http://e2e.test/stream.m3u8');

    await page.click('button:has-text("Create")');
    await page.waitForTimeout(1_200);

    await expect(page.locator('#iptv-channel-table')).toContainText(CH_NAME, { timeout: 5_000 });
  });

  test('channel detail (providers) page loads', async ({ page }) => {
    await page.goto('/iptv/channels');
    await waitForTable(page, 'iptv-channel-table');

    const link = page.locator('#iptv-channel-table a:has-text("Providers")').first();
    if (await link.count() === 0) {
      test.skip(true, 'No channels in database');
    }
    await link.click();
    await page.waitForURL(/\/iptv\/channels\/.+/, { timeout: 5_000 });
    const text = await page.locator('body').innerText();
    expect(text).not.toContain('panic:');
  });

  test('delete channel', async ({ page }) => {
    await page.goto('/iptv/channels');
    await waitForTable(page, 'iptv-channel-table');

    const row = page.locator(`#iptv-channel-table tr:has-text("${CH_NAME}")`);
    if (await row.count() === 0) {
      test.skip(true, 'E2E channel not found — create test may have failed');
    }
    await row.locator('button:has-text("Remove")').click();
    await page.waitForTimeout(1_200);
    await expect(page.locator('#iptv-channel-table')).not.toContainText(CH_NAME, { timeout: 5_000 });
  });
});

// ── Packages ──────────────────────────────────────────────────────────────────

test.describe('IPTV Packages', () => {
  const PKG_NAME = `E2E Package ${TS}`;

  test('packages page loads', async ({ page }) => {
    await page.goto('/iptv/packages');
    await expect(page.getByRole('heading', { name: 'IPTV — Packages' })).toBeVisible();
    await expect(page.locator('button:has-text("+ Package")')).toBeVisible();
  });

  test('package table loads via SSE', async ({ page }) => {
    await page.goto('/iptv/packages');
    await waitForTable(page, 'iptv-package-table');
  });

  test('create package via modal', async ({ page }) => {
    await page.goto('/iptv/packages');
    await waitForTable(page, 'iptv-package-table');

    await page.click('button:has-text("+ Package")');
    await expect(page.locator('h2:has-text("New Package")')).toBeVisible();

    await page.fill('[placeholder="Name"]', PKG_NAME);
    await page.fill('[placeholder="Price (e.g. 9.99)"]', '9.99');

    await page.click('button:has-text("Create")');
    await page.waitForTimeout(1_200);

    await expect(page.locator('#iptv-package-table')).toContainText(PKG_NAME, { timeout: 5_000 });
  });

  test('delete package', async ({ page }) => {
    await page.goto('/iptv/packages');
    await waitForTable(page, 'iptv-package-table');

    const row = page.locator(`#iptv-package-table tr:has-text("${PKG_NAME}")`);
    if (await row.count() === 0) {
      test.skip(true, 'E2E package not found — create test may have failed');
    }
    await row.locator('button:has-text("Remove")').click();
    await page.waitForTimeout(1_200);
    await expect(page.locator('#iptv-package-table')).not.toContainText(PKG_NAME, { timeout: 5_000 });
  });
});

// ── Subscriptions ─────────────────────────────────────────────────────────────

test.describe('IPTV Subscriptions', () => {
  test('subscriptions page loads', async ({ page }) => {
    await page.goto('/iptv/subscriptions');
    await expect(page.getByRole('heading', { name: 'IPTV — Subscriptions' })).toBeVisible();
    await expect(page.locator('button:has-text("+ Subscribe")')).toBeVisible();
  });

  test('subscription table loads via SSE', async ({ page }) => {
    await page.goto('/iptv/subscriptions');
    await waitForTable(page, 'iptv-sub-table');
  });

  test('open subscribe modal loads cascading selects', async ({ page }) => {
    await page.goto('/iptv/subscriptions');
    await waitForTable(page, 'iptv-sub-table');

    await page.click('button:has-text("+ Subscribe")');
    await expect(page.locator('h2:has-text("New Subscription")')).toBeVisible();

    // SSE fires and patches customer + package selects
    await page.waitForSelector('#iptv-sel-sub-customer select:not([disabled])', { timeout: 8_000 });
    await page.waitForSelector('#iptv-sel-sub-package select:not([disabled])', { timeout: 8_000 });
  });

  test('create subscription via cascading selects', async ({ page }) => {
    await page.goto('/iptv/subscriptions');
    await waitForTable(page, 'iptv-sub-table');

    await page.click('button:has-text("+ Subscribe")');
    await page.waitForSelector('#iptv-sel-sub-customer select:not([disabled])', { timeout: 8_000 });
    await page.waitForSelector('#iptv-sel-sub-package select:not([disabled])', { timeout: 8_000 });

    const custOpts = await page.locator('#iptv-sel-sub-customer select option').count();
    const pkgOpts  = await page.locator('#iptv-sel-sub-package select option').count();

    if (custOpts <= 1 || pkgOpts <= 1) {
      test.skip(true, 'No customers or packages available — run customers/packages tests first');
    }

    await page.selectOption('#iptv-sel-sub-customer select', { index: 1 });
    await page.selectOption('#iptv-sel-sub-package select', { index: 1 });

    await page.click('button:has-text("Subscribe")');
    await page.waitForTimeout(1_500);

    await waitForTable(page, 'iptv-sub-table');
    // At least one row should be visible — no empty-state placeholder
    const text = await page.locator('#iptv-sub-table').innerText();
    expect(text).not.toContain('No active subscriptions');
  });

  test('suspend active subscription', async ({ page }) => {
    await page.goto('/iptv/subscriptions');
    await waitForTable(page, 'iptv-sub-table');

    const btn = page.locator('#iptv-sub-table button:has-text("Suspend")').first();
    if (await btn.count() === 0) {
      test.skip(true, 'No active subscriptions to suspend');
    }
    await btn.click();
    await page.waitForTimeout(1_000);
    await expect(page.locator('#iptv-sub-table button:has-text("Reactivate")').first())
      .toBeVisible({ timeout: 5_000 });
  });

  test('reactivate suspended subscription', async ({ page }) => {
    await page.goto('/iptv/subscriptions');
    await waitForTable(page, 'iptv-sub-table');

    const btn = page.locator('#iptv-sub-table button:has-text("Reactivate")').first();
    if (await btn.count() === 0) {
      test.skip(true, 'No suspended subscriptions to reactivate');
    }
    await btn.click();
    await page.waitForTimeout(1_000);
    await expect(page.locator('#iptv-sub-table button:has-text("Suspend")').first())
      .toBeVisible({ timeout: 5_000 });
  });

  test('cancel subscription', async ({ page }) => {
    await page.goto('/iptv/subscriptions');
    await waitForTable(page, 'iptv-sub-table');

    const btn = page.locator('#iptv-sub-table button:has-text("Cancel")').first();
    if (await btn.count() === 0) {
      test.skip(true, 'No subscriptions to cancel');
    }
    await btn.click();
    await page.waitForTimeout(1_000);
    // Table refreshes — just check no crash
    const text = await page.locator('body').innerText();
    expect(text).not.toContain('panic:');
  });
});

// ── STBs ──────────────────────────────────────────────────────────────────────

test.describe('IPTV STBs', () => {
  // Use last 2 hex digits of timestamp for a unique MAC suffix
  const HEX = String(TS % 256).padStart(2, '0').toUpperCase();
  const MAC  = `AA:BB:CC:DD:EE:${HEX}`;
  const MODEL = 'MAG 522W3';

  test('STBs page loads', async ({ page }) => {
    await page.goto('/iptv/stbs');
    await expect(page.getByRole('heading', { name: 'IPTV — Set-Top Boxes' })).toBeVisible();
    await expect(page.locator('button:has-text("+ Register STB")')).toBeVisible();
  });

  test('STB table loads via SSE', async ({ page }) => {
    await page.goto('/iptv/stbs');
    await waitForTable(page, 'iptv-stb-table');
  });

  test('open register STB modal loads customer select', async ({ page }) => {
    await page.goto('/iptv/stbs');
    await waitForTable(page, 'iptv-stb-table');

    await page.click('button:has-text("+ Register STB")');
    await expect(page.locator('h2:has-text("Register STB")')).toBeVisible();

    // SSE fires and patches customer select
    await page.waitForSelector('#iptv-sel-stb-customer select:not([disabled])', { timeout: 8_000 });
  });

  test('register STB via modal', async ({ page }) => {
    await page.goto('/iptv/stbs');
    await waitForTable(page, 'iptv-stb-table');

    await page.click('button:has-text("+ Register STB")');
    await page.waitForSelector('#iptv-sel-stb-customer select:not([disabled])', { timeout: 8_000 });

    const custOpts = await page.locator('#iptv-sel-stb-customer select option').count();
    if (custOpts <= 1) {
      test.skip(true, 'No customers available — run customers.spec.js first');
    }

    await page.fill('[placeholder="MAC (00:1A:2B:3C:4D:5E)"]', MAC);
    await page.fill('[placeholder="Model (e.g. MAG 522W3)"]', MODEL);
    await page.selectOption('#iptv-sel-stb-customer select', { index: 1 });

    await page.click('button:has-text("Register")');
    await page.waitForTimeout(1_200);

    await expect(page.locator('#iptv-stb-table')).toContainText(MAC, { timeout: 5_000 });
  });

  test('delete STB', async ({ page }) => {
    await page.goto('/iptv/stbs');
    await waitForTable(page, 'iptv-stb-table');

    const row = page.locator(`#iptv-stb-table tr:has-text("${MAC}")`);
    if (await row.count() === 0) {
      test.skip(true, 'E2E STB not found — register test may have failed');
    }
    await row.locator('button:has-text("Remove")').click();
    await page.waitForTimeout(1_200);
    await expect(page.locator('#iptv-stb-table')).not.toContainText(MAC, { timeout: 5_000 });
  });
});

// ── Stacks ────────────────────────────────────────────────────────────────────

test.describe('IPTV Stacks', () => {
  test('stacks page loads', async ({ page }) => {
    await page.goto('/iptv/stacks');
    await expect(page.getByRole('heading', { name: 'IPTV — Stacks' })).toBeVisible();
    await expect(page.locator('button:has-text("+ Stack")')).toBeVisible();
  });

  test('stack table loads via SSE', async ({ page }) => {
    await page.goto('/iptv/stacks');
    await waitForTable(page, 'iptv-stack-table');
  });

  test('open stack modal loads cluster select', async ({ page }) => {
    await page.goto('/iptv/stacks');
    await waitForTable(page, 'iptv-stack-table');

    await page.click('button:has-text("+ Stack")');
    await expect(page.locator('h2:has-text("New IPTV Stack")')).toBeVisible();

    // SSE fires to load clusters
    await page.waitForSelector('#iptv-sel-cluster select', { timeout: 8_000 });
    const text = await page.locator('body').innerText();
    expect(text).not.toContain('panic:');
  });
});
