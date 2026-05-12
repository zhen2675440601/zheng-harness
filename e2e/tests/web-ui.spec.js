const { expect, test } = require('@playwright/test');
const { makeJWT, startServer, stopServer } = require('../server-harness');

const validToken = makeJWT();

async function connect(page, token = validToken) {
  await page.goto('/', { waitUntil: 'networkidle' });
  await page.waitForSelector('#auth-screen', { state: 'visible' });

  await page.evaluate((t) => {
    window.localStorage.setItem('zhengHarness.jwt', t);
  }, token);

  await page.reload({ waitUntil: 'networkidle' });

  await expect(page.locator('#main-content')).toBeVisible({ timeout: 15000 });
  await expect(page.locator('#page-chat')).toBeVisible();
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

async function connectManually(page, token) {
  await page.goto('/', { waitUntil: 'networkidle' });
  await expect(page.locator('#auth-screen')).toBeVisible();
  await page.getByLabel(/JWT Token|JWT 令牌/).fill(token);
  await page.getByRole('button', { name: /Connect|进入聊天工作台|连接/ }).click();
}

async function submitMessage(page, message) {
  const responsePromise = page.waitForResponse((response) => {
    return response.url().includes('/api/v1/chat/start') && response.request().method() === 'POST';
  });

  await page.getByRole('textbox', { name: /消息内容/ }).fill(message);
  await page.getByRole('button', { name: /发送消息/ }).click();

  const response = await responsePromise;
  expect(response.status()).toBe(202);

  await expect(page.locator('#conversation-id')).not.toHaveText('未创建');
  await expect(page.locator('#conversation-stream')).toContainText(message, { timeout: 15000 });
  await expect(page.locator('#conversation-stream')).toContainText(/assistant|Mock provider completed the requested task|Deterministic mock plan|会话已完成：/, { timeout: 15000 });
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
    test('WebUI: loads root page and shows auth screen', async ({ page }) => {
      const response = await page.goto('/');
      expect(response).not.toBeNull();
      expect(response.ok()).toBeTruthy();
      await expect(page.locator('#auth-screen')).toBeVisible();
      await expect(page.getByText(/Authentication|连接 Harness 智能体/)).toBeVisible();
    });

    test('WebUI: connecting with valid JWT shows chat workspace', async ({ page }) => {
      await connect(page);
      await expect(page.locator('#main-content')).toBeVisible();
      await expect(page.locator('#conversation-list')).toBeVisible();
    });

    test('WebUI: invalid JWT shows auth error', async ({ page }) => {
      await page.goto('/');
      await page.getByLabel(/JWT Token|JWT 令牌/).fill('bad.token.value');
      await page.getByRole('button', { name: /Connect|连接/ }).click();
      await expect(page.locator('#auth-error')).toBeVisible();
      await expect(page.locator('#auth-error')).toContainText('Invalid token');
    });
  });

  test.describe('composer', () => {
    test('WebUI: composer validates empty input', async ({ page }) => {
      await connect(page);
      await page.getByRole('button', { name: /发送消息/ }).click();
      await expect(page.locator('#composer-validation-error')).toBeVisible();
      await expect(page.locator('#composer-validation-error')).toContainText(/请输入消息后再发送。/);
    });
  });

  test.describe('history', () => {
    test('WebUI: conversation list loads and shows sessions', async ({ page, request }) => {
      await createSessionViaAPI(request, 'history test session');
      await connect(page);
      await expect(page.locator('#conversation-list')).toContainText(/history test session/);
    });

    test('WebUI: inspect panel renders session data for selected conversation', async ({ page, request }) => {
      const created = await createSessionViaAPI(request, 'detail test session');
      await connect(page);
      await page.getByRole('button', { name: /历史会话/ }).click();
      await page.locator('[data-action="open-chat"]').first().click();
      await page.getByRole('button', { name: /查看 Inspect/ }).click();
      await expect(page.locator('#inspect-detail')).toContainText(created.session_id);
      await expect(page.locator('#inspect-detail')).toContainText(/状态|Status/);
    });
  });

  test.describe('stream', () => {
    test('WebUI: live conversation stream renders events', async ({ page }) => {
      await connect(page);
      await page.getByRole('textbox', { name: /消息内容/ }).fill('stream session from browser');
      await page.getByRole('button', { name: /发送消息/ }).click();

      await expect(page.locator('#page-chat')).toBeVisible();
      await expect(page.locator('#conversation-id')).not.toHaveText('未创建');
      await expect(page.locator('#conversation-stream')).toContainText(/user|assistant|流式中|会话已完成：|正在连接会话流|正在实时推送会话输出/, { timeout: 15000 });
    });

    test('WebUI: missing conversation route surfaces load failure', async ({ page }) => {
      await connect(page);
      await page.goto('/#/chat/conversation-does-not-exist');
      await expect(page.locator('#page-chat')).toBeVisible();
      await expect(page.locator('#composer-error')).toContainText(/加载对话失败|not found/i, { timeout: 15000 });
    });

    test('WebUI: malformed SSE payload still completes current stream', async ({ page }) => {
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
      await page.getByRole('textbox', { name: /消息内容/ }).fill('malformed sse test');
      await page.getByRole('button', { name: /发送消息/ }).click();
      await expect(page.locator('#conversation-stream')).toContainText(/会话已完成：/, { timeout: 15000 });
    });
  });

  test.describe('reliability', () => {
    test('WebUI: manual JWT login succeeds and persists workspace on reload', async ({ page }) => {
      await connectManually(page, validToken);
      await expect(page.locator('#main-content')).toBeVisible({ timeout: 15000 });
      await expect(page.locator('#conversation-list')).toBeVisible();

      await page.reload({ waitUntil: 'networkidle' });

      await expect(page.locator('#main-content')).toBeVisible({ timeout: 15000 });
      await expect(page.locator('#page-chat')).toBeVisible();
      await expect(page.locator('#auth-screen')).toBeHidden();
    });

    test('WebUI: invalid JWT keeps user on auth screen with visible error feedback', async ({ page }) => {
      await connectManually(page, 'bad.token.value');
      await expect(page.locator('#auth-screen')).toBeVisible();
      await expect(page.locator('#auth-error')).toBeVisible();
      await expect(page.locator('#auth-error')).toContainText(/invalid token/i);
      await expect(page.locator('#main-content')).toBeHidden();
    });

    test('WebUI: expired JWT returns to auth screen after bootstrap fails', async ({ page }) => {
      const expiredToken = makeJWT(undefined, new Date(Date.now() - 60_000));
      await page.goto('/', { waitUntil: 'networkidle' });
      await page.waitForSelector('#auth-screen', { state: 'visible' });
      await page.evaluate((token) => {
        window.localStorage.setItem('zhengHarness.jwt', token);
      }, expiredToken);
      await page.reload({ waitUntil: 'networkidle' });
      await expect(page.locator('#auth-screen')).toBeVisible();
      await expect(page.locator('#auth-error')).toContainText(/expired|过期/i);
    });

    test('WebUI: empty composer blocks submission until message is provided', async ({ page }) => {
      await connect(page);
      await page.getByRole('button', { name: /发送消息/ }).click();
      await expect(page.locator('#composer-validation-error')).toBeVisible();
      await expect(page.locator('#composer-validation-error')).toContainText(/请输入消息后再发送。/);
      await expect(page.locator('#conversation-id')).toHaveText('未创建');
    });

    test('WebUI: normal chat submission streams response and records session history', async ({ page }) => {
      await connect(page);
      await submitMessage(page, 'reliability happy path message');
      await expect(page.locator('#conversation-stream')).toContainText('reliability happy path message');
      await expect(page.locator('#conversation-list')).toContainText(/reliability happy path message/, { timeout: 15000 });
    });

    test('WebUI: visible submit failure feedback is shown when chat start request fails', async ({ page }) => {
      await page.route('**/api/v1/chat/start', async (route) => {
        await route.fulfill({
          status: 500,
          contentType: 'application/json',
          body: JSON.stringify({
            error: {
              code: 'internal_error',
              message: 'mocked chat start failure',
            },
            request_id: 'req-playwright-failure',
          }),
        });
      });

      await connect(page);
      await page.getByRole('textbox', { name: /消息内容/ }).fill('should fail to submit');
      await page.getByRole('button', { name: /发送消息/ }).click();

      await expect(page.locator('#composer-error')).toBeVisible();
      await expect(page.locator('#composer-error')).toContainText(/mocked chat start failure/);
      await expect(page.locator('#conversation-id')).toHaveText('未创建');
    });

    test('WebUI: refresh preserves active conversation transcript and session continuity', async ({ page }) => {
      await connect(page);
      await submitMessage(page, 'refresh continuity message');

      const conversationId = (await page.locator('#conversation-id').textContent()).trim();
      expect(conversationId).toBeTruthy();
      expect(conversationId).not.toBe('未创建');

      await page.reload({ waitUntil: 'networkidle' });

      await expect(page.locator('#main-content')).toBeVisible({ timeout: 15000 });
      await expect(page.locator('#conversation-id')).toHaveText(conversationId);
      await expect(page.locator('#conversation-stream')).toContainText('refresh continuity message', { timeout: 15000 });
      await expect(page.locator('#conversation-stream')).toContainText(/assistant|Mock provider completed the requested task|Deterministic mock plan|会话已完成：/, { timeout: 15000 });
      await expect(page.locator('#conversation-list')).toContainText(/refresh continuity message/, { timeout: 15000 });
    });

    test('WebUI: stale conversation deep link recovers after re-authentication', async ({ page, request }) => {
      await createSessionViaAPI(request, 'stale route test session');
      await connect(page);
      await page.goto('/#/chat/conversation-does-not-exist', { waitUntil: 'networkidle' });
      await expect(page.locator('#composer-error')).toContainText(/加载对话失败|not found/i, { timeout: 15000 });

      await page.evaluate(() => {
        window.localStorage.removeItem('zhengHarness.jwt');
      });
      await page.reload({ waitUntil: 'networkidle' });
      await expect(page.locator('#auth-screen')).toBeVisible();
      await page.getByLabel(/JWT Token|JWT 令牌/).fill(validToken);
      await page.getByRole('button', { name: /Connect|连接/ }).click();
      await expect(page.locator('#conversation-list')).toBeVisible();
      await expect(page.locator('#composer-error')).toHaveCount(0);
    });
  });
});
