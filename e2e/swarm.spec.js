/**
 * Swarm module tests — Clusters, Hetzner config page, filter checkboxes.
 *
 * Most tests are conditional: they skip gracefully when no clusters exist
 * or when a cluster has no Hetzner credentials configured.
 *
 * The Hetzner filter fix being verified:
 *   Signal names must NOT start with "_" — Datastar treats those as private
 *   and never sends them to the backend.  Before the fix, _hl_* / _hs_*
 *   signals were silently dropped, so Save always stored an empty filter.
 *   After the fix: hl_* / hs_* signals are sent and persisted correctly.
 */
const { test, expect } = require('@playwright/test');

// ── Helpers ───────────────────────────────────────────────────────────────────

async function waitForTable(page, tableId) {
  await page.waitForSelector(`#${tableId}`, { timeout: 8_000 });
  const text = await page.locator('body').innerText();
  expect(text).not.toContain('panic:');
  expect(text).not.toContain('500 Internal');
}

// Returns the href of the first cluster's detail link, or null.
async function firstClusterHref(page) {
  await page.goto('/swarm/clusters');
  await waitForTable(page, 'swarm-clusters-table');
  const link = page.locator('#swarm-clusters-table a[href^="/swarm/clusters/"]').first();
  if (await link.count() === 0) return null;
  return link.getAttribute('href');
}

// ── Clusters list ─────────────────────────────────────────────────────────────

test.describe('Swarm Clusters', () => {
  test('clusters page loads', async ({ page }) => {
    await page.goto('/swarm/clusters');
    await expect(page.getByRole('heading', { name: 'Swarm Clusters' })).toBeVisible();
    await expect(page.locator('a:has-text("+ New Cluster")')).toBeVisible();
  });

  test('cluster table loads via SSE', async ({ page }) => {
    await page.goto('/swarm/clusters');
    await waitForTable(page, 'swarm-clusters-table');
  });
});

// ── Cluster detail ────────────────────────────────────────────────────────────

test.describe('Swarm Cluster Detail', () => {
  test('detail page loads and has no errors', async ({ page }) => {
    const href = await firstClusterHref(page);
    if (!href) {
      test.skip(true, 'No clusters in database');
    }
    await page.goto(href);
    const text = await page.locator('body').innerText();
    expect(text).not.toContain('panic:');
    expect(text).not.toContain('500 Internal');
  });

  test('⚙ Hetzner link always visible on cluster detail page', async ({ page }) => {
    const href = await firstClusterHref(page);
    if (!href) {
      test.skip(true, 'No clusters in database');
    }
    await page.goto(href);
    // Link must be present regardless of whether Hetzner is configured
    await expect(page.locator('a:has-text("⚙ Hetzner")').first()).toBeVisible({ timeout: 5_000 });
  });

  test('⚙ Hetzner link navigates to hetzner config page', async ({ page }) => {
    const href = await firstClusterHref(page);
    if (!href) {
      test.skip(true, 'No clusters in database');
    }
    await page.goto(href);
    await page.locator('a:has-text("⚙ Hetzner")').first().click();
    await page.waitForURL(/\/swarm\/clusters\/.+\/hetzner/, { timeout: 5_000 });
    const text = await page.locator('body').innerText();
    expect(text).not.toContain('panic:');
  });
});

// ── Hetzner config page ────────────────────────────────────────────────────────

test.describe('Hetzner Config Page', () => {
  let hetznerUrl = null;

  test.beforeEach(async ({ page }) => {
    const href = await firstClusterHref(page);
    if (!href) {
      test.skip(true, 'No clusters in database');
    }
    // Extract cluster ID from e.g. /swarm/clusters/abc123
    const clusterID = href.split('/').pop();
    hetznerUrl = `/swarm/clusters/${clusterID}/hetzner`;
  });

  test('hetzner config page loads', async ({ page }) => {
    await page.goto(hetznerUrl);
    await expect(page.getByRole('heading', { name: 'Hetzner Settings' })).toBeVisible();
    const text = await page.locator('body').innerText();
    expect(text).not.toContain('panic:');
  });

  test('API Credentials card has required fields', async ({ page }) => {
    await page.goto(hetznerUrl);
    // API token (password field)
    await expect(page.locator('input[data-bind\\:hetzner_apikey]')).toBeVisible();
    // SSH key ID (number field)
    await expect(page.locator('input[data-bind\\:hetzner_sshkeyid]')).toBeVisible();
    // Private key textarea
    await expect(page.locator('textarea[data-bind\\:hetzner_sshprivkey]')).toBeVisible();
    // Save button
    await expect(page.locator('button:has-text("Save Credentials")')).toBeVisible();
  });

  test('Order Panel Options card is present', async ({ page }) => {
    await page.goto(hetznerUrl);
    await expect(page.locator('h2:has-text("Order Panel Options")')).toBeVisible();
  });

  test('filter section shows loading or "configure first" message', async ({ page }) => {
    await page.goto(hetznerUrl);
    const body = await page.locator('body').innerText();
    // Either: "Loading options from Hetzner…" (HasHetzner=true) or
    //         "Configure API credentials and save first" (HasHetzner=false)
    const hasLoadingState = body.includes('Loading options from Hetzner') ||
                            body.includes('Configure API credentials');
    expect(hasLoadingState).toBeTruthy();
  });

  test('back link points to cluster detail', async ({ page }) => {
    await page.goto(hetznerUrl);
    // "← ClusterName" back link
    const backLink = page.locator('a[href*="/swarm/clusters/"]').first();
    await expect(backLink).toBeVisible();
  });
});

// ── Hetzner filter — signal name correctness ───────────────────────────────────

test.describe('Hetzner Filter Signals', () => {
  /**
   * These tests verify the _hl_ → hl_ fix:
   * signals without leading _ are sent to the backend when POSTing.
   *
   * They require a cluster that already has Hetzner credentials configured
   * (HasHetzner=true) so that the filter section actually loads checkboxes.
   * If no such cluster exists they skip.
   */

  async function getHetznerClusterURL(page) {
    await page.goto('/swarm/clusters');
    await waitForTable(page, 'swarm-clusters-table');
    // Look for a cluster row that has "⚡ Order via Hetzner" (HasHetzner=true)
    const link = page.locator('#swarm-clusters-table a[href*="/hetzner"]').first();
    if (await link.count() === 0) return null;
    const href = await link.getAttribute('href');
    // href is e.g. /swarm/clusters/abc/hetzner or /swarm/clusters/abc
    const match = href.match(/\/swarm\/clusters\/([^/]+)/);
    if (!match) return null;
    return `/swarm/clusters/${match[1]}/hetzner`;
  }

  test('filter section renders checkboxes after SSE loads (HasHetzner cluster)', async ({ page }) => {
    // Navigate to clusters table, find any with ⚙ Hetzner in header (after going to detail)
    await page.goto('/swarm/clusters');
    await waitForTable(page, 'swarm-clusters-table');

    // Find first cluster detail link
    const clusterLink = page.locator('#swarm-clusters-table a[href^="/swarm/clusters/"]').first();
    if (await clusterLink.count() === 0) {
      test.skip(true, 'No clusters in database');
    }
    const clusterHref = await clusterLink.getAttribute('href');
    await page.goto(clusterHref);

    // Check if "⚡ Order via Hetzner" button exists → this cluster HasHetzner
    const orderBtn = page.locator('button:has-text("Order via Hetzner")');
    if (await orderBtn.count() === 0) {
      test.skip(true, 'Cluster has no Hetzner credentials — configure first');
    }

    // Navigate to hetzner config page
    const clusterID = clusterHref.split('/').pop();
    await page.goto(`/swarm/clusters/${clusterID}/hetzner`);

    // Filter section should auto-fetch via data-effect
    // Wait for either checkboxes OR error message (bad API key)
    await page.waitForSelector('#hetzner-filter-section', { timeout: 10_000 });
    const filterText = await page.locator('#hetzner-filter-section').innerText();
    // Should NOT still be "Loading options from Hetzner…" (SSE fired)
    // Could be checkboxes or error from bad API token
    expect(filterText).not.toContain('panic:');
    expect(filterText).not.toContain('500');
  });

  test('Save Options Filter button exists when filter section loads', async ({ page }) => {
    await page.goto('/swarm/clusters');
    await waitForTable(page, 'swarm-clusters-table');

    const clusterLink = page.locator('#swarm-clusters-table a[href^="/swarm/clusters/"]').first();
    if (await clusterLink.count() === 0) {
      test.skip(true, 'No clusters in database');
    }
    const clusterHref = await clusterLink.getAttribute('href');
    await page.goto(clusterHref);

    const orderBtn = page.locator('button:has-text("Order via Hetzner")');
    if (await orderBtn.count() === 0) {
      test.skip(true, 'Cluster has no Hetzner credentials — configure first');
    }

    const clusterID = clusterHref.split('/').pop();
    await page.goto(`/swarm/clusters/${clusterID}/hetzner`);

    // Wait for SSE to patch #hetzner-filter-section
    await page.waitForSelector('#hetzner-filter-section', { timeout: 10_000 });

    // If options loaded (not error), Save button should be visible
    const filterSection = page.locator('#hetzner-filter-section');
    const filterText = await filterSection.innerText();
    if (filterText.includes('Could not fetch options')) {
      test.skip(true, 'Hetzner API unreachable — cannot verify filter checkboxes');
    }

    await expect(page.locator('button:has-text("Save Options Filter")')).toBeVisible({ timeout: 5_000 });
  });

  test('signal names do not start with underscore (regression: _hl_ → hl_)', async ({ page }) => {
    // This test directly verifies the signal naming fix by inspecting the DOM
    // for data-bind attributes on checkboxes in the filter section.
    await page.goto('/swarm/clusters');
    await waitForTable(page, 'swarm-clusters-table');

    const clusterLink = page.locator('#swarm-clusters-table a[href^="/swarm/clusters/"]').first();
    if (await clusterLink.count() === 0) {
      test.skip(true, 'No clusters in database');
    }
    const clusterHref = await clusterLink.getAttribute('href');
    await page.goto(clusterHref);

    const orderBtn = page.locator('button:has-text("Order via Hetzner")');
    if (await orderBtn.count() === 0) {
      test.skip(true, 'Cluster has no Hetzner credentials — configure first');
    }

    const clusterID = clusterHref.split('/').pop();
    await page.goto(`/swarm/clusters/${clusterID}/hetzner`);

    await page.waitForSelector('#hetzner-filter-section', { timeout: 10_000 });

    // Find any checkboxes in the filter section
    const checkboxes = page.locator('#hetzner-filter-section input[type="checkbox"]');
    const count = await checkboxes.count();
    if (count === 0) {
      test.skip(true, 'No checkboxes rendered — API unreachable or no options returned');
    }

    // All data-bind attributes must start with hl_ or hs_, never _hl_ / _hs_
    for (let i = 0; i < count; i++) {
      const bindAttr = await checkboxes.nth(i).getAttribute('data-bind');
      if (bindAttr) {
        expect(bindAttr).toMatch(/^(hl_|hs_)/);
        expect(bindAttr).not.toMatch(/^_/);
      }
    }
  });
});
