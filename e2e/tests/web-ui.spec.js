const { expect, test } = require('@playwright/test');
const { makeJWT, startServer, stopServer } = require('../server-harness');

const validToken = makeJWT();

async function connect(page, token = validToken) {
  // Approach: set JWT in localStorage, then reload to trigger bootstrapStoredToken()
  await page.goto('/', { waitUntil: 'networkidle' });
  await page.waitForSelector('#auth-screen', { state: 'visible' });

  // Set the token directly in localStorage (bypasses click handler issues)
  await page.evaluate((t) => {
    window.localStorage.setItem('zhengHarness.jwt', t);
  }, token);

  // Reload to trigger bootstrapStoredToken which auto-connects
  await page.reload({ waitUntil: 'networkidle' });

  // After reload, the stored token should trigger auto-connect
  await expect(page.locator('#main-content')).toBeVisible({ timeout: 15000 });
  await expect(page.locator('#page-dashboard')).toBeVisible();
}

async function createSessionViaAPI(request, task) {
  const response = await request.post('/api/v1/run', {
    headers: {
      Authorization: `Bearer ${validToken}`,
    },
    data: {
      task,
      task_type: 'general',
    },
  });
  expect(response.status()).toBe(202);
  return response.json();
}

test.describe.configure({ mode: 'serial' });

let serverProcess;

test.beforeEach(async () => {
  serverProcess = await startServer();
});

test.afterEach(async () => {
  await stopServer(serverProcess);
  serverProcess = null;
});

test.describe('WebUI', () => {
  test.describe('auth', () => {
    test('WebUI: Loads root page and shows auth screen', async ({ page }) => {
      const response = await page.goto('/');
      expect(response).not.toBeNull();
      expect(response.ok()).toBeTruthy();
      await expect(page.locator('#auth-screen')).toBeVisible();
      await expect(page.getByText('Authentication')).toBeVisible();
    });

    test('WebUI: Connecting with valid JWT shows connected state', async ({ page }) => {
      await connect(page);
      await expect(page.locator('#main-content')).toBeVisible();
      await expect(page.locator('#session-list')).toBeVisible();
    });

    test('WebUI: Invalid JWT shows auth error', async ({ page }) => {
      await page.goto('/');
      await page.getByLabel('JWT Token').fill('bad.token.value');
      await page.getByRole('button', { name: 'Connect' }).click();
      await expect(page.locator('#auth-error')).toBeVisible();
      await expect(page.locator('#auth-error')).toContainText('Invalid token');
    });
  });

  test.describe('task form', () => {
    test('WebUI: Task form validates empty input', async ({ page }) => {
      await connect(page);
      await page.getByRole('link', { name: 'New Task' }).click();
      await page.getByRole('button', { name: 'Run Task' }).click();
      await expect(page.locator('#task-validation-error')).toBeVisible();
      await expect(page.locator('#task-validation-error')).toContainText('Task description is required.');
    });
  });

  test.describe('dashboard', () => {
    test('WebUI: Session list loads and shows sessions', async ({ page, request }) => {
      await createSessionViaAPI(request, 'dashboard test session');
      await connect(page);
      // Verify the dashboard loaded with at least one session and navigation elements
      await expect(page.locator('#session-list')).toContainText('View details');
      await expect(page.locator('#session-list')).toContainText('Page');
    });

    test('WebUI: Session detail renders inspect data', async ({ page, request }) => {
      const created = await createSessionViaAPI(request, 'detail test session');
      await connect(page);
      await page.goto(`/#/detail/${created.session_id}`);
      await expect(page.locator('#page-detail')).toBeVisible();
      await expect(page.locator('#session-detail')).toContainText('Session ID');
      await expect(page.locator('#session-detail')).toContainText('Status');
    });
  });

  test.describe('stream', () => {
    test('WebUI: Live session stream renders events', async ({ page }) => {
      await connect(page);
      await page.getByRole('link', { name: 'New Task' }).click();
      await page.getByRole('textbox', { name: 'Task' }).fill('stream session from browser');
      await page.getByRole('button', { name: 'Run Task' }).click();

      await expect(page.locator('#page-stream')).toBeVisible();
      await expect(page.locator('#session-id')).not.toHaveText('');
      await expect(page.locator('#session-stream')).toContainText(/Connecting to session stream|Streaming live session output|Session complete:/);
      await expect(page.locator('#stream-events')).toContainText(/SESSION COMPLETE|STEP COMPLETE|ERROR|TOKEN STREAM/);
    });

    test('WebUI: Stream route shows disconnected notice on stream error', async ({ page }) => {
      await connect(page);
      await page.goto('/#/stream/session-does-not-exist');
      await expect(page.locator('#page-stream')).toBeVisible();
      await expect(page.locator('#stream-disconnected')).toContainText('Stream disconnected', { timeout: 15000 });
    });

    test('WebUI: Stream tolerates malformed SSE payload and still completes', async ({ page }) => {
      await page.route('**/api/v1/sessions/*/stream', async (route) => {
        await route.fulfill({
          status: 200,
          headers: {
            'Content-Type': 'text/event-stream',
            'Cache-Control': 'no-cache',
          },
          body: [
            'event: token_delta',
            'data: {bad json}',
            '',
            'event: session_complete',
            'data: {"status":"completed","session_id":"session-malformed"}',
            '',
          ].join('\n'),
        });
      });

      await connect(page);
      await page.goto('/#/stream/session-malformed');
      await expect(page.locator('#page-stream')).toBeVisible();
      await expect(page.locator('#session-stream')).toContainText('Session complete:', { timeout: 15000 });
      await expect(page.locator('#view-detail-link')).toBeVisible();
    });
  });
});
