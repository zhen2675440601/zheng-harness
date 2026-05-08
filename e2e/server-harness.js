const { spawn } = require('child_process');
const crypto = require('crypto');
const fs = require('fs/promises');
const path = require('path');

const SERVER_PORT = Number(process.env.E2E_SERVER_PORT || 8080);
const MOCK_PROVIDER_PORT = Number(process.env.E2E_MOCK_PROVIDER_PORT || 19999);
const JWT_SECRET = process.env.E2E_JWT_SECRET || 'test-token';
const REPO_ROOT = path.resolve(__dirname, '..');
const DB_PATH = path.resolve(REPO_ROOT, 'testdata', 'e2e.db');
const SERVER_BASE_URL = process.env.E2E_BASE_URL || `http://127.0.0.1:${SERVER_PORT}`;
const MOCK_PROVIDER_BASE_URL = process.env.E2E_MOCK_PROVIDER_BASE_URL || `http://127.0.0.1:${MOCK_PROVIDER_PORT}/v1`;

function buildOpenAIResponse(content) {
  return {
    id: 'chatcmpl-e2e',
    object: 'chat.completion',
    created: Math.floor(Date.now() / 1000),
    model: 'mock-gpt-4.1-mini',
    choices: [
      {
        index: 0,
        finish_reason: 'stop',
        message: {
          role: 'assistant',
          content,
        },
      },
    ],
    usage: {
      prompt_tokens: 1,
      completion_tokens: 1,
      total_tokens: 2,
    },
  };
}

function chooseMockContent(body) {
  const messages = Array.isArray(body && body.messages) ? body.messages : [];
  const promptText = messages.map((message) => {
    const content = message && message.content;
    if (typeof content === 'string') {
      return content;
    }
    if (Array.isArray(content)) {
      return content.map((part) => (part && typeof part.text === 'string' ? part.text : '')).join(' ');
    }
    return '';
  }).join('\n');

  if (/next action|tool_call|respond/i.test(promptText)) {
    return JSON.stringify({
      type: 'respond',
      summary: 'Return deterministic completion',
      response: 'Mock provider completed the requested task.',
    });
  }

  return JSON.stringify({
    summary: 'Deterministic mock plan',
    steps: ['Return a deterministic mock response to exercise end-to-end browser flows.'],
  });
}

function startMockProvider(options = {}) {
  const http = require('http');
  const port = options.port || MOCK_PROVIDER_PORT;
  const server = http.createServer(async (req, res) => {
    if (req.method === 'GET' && req.url === '/healthz') {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ status: 'ok' }));
      return;
    }

    if (req.method !== 'POST' || req.url !== '/v1/chat/completions') {
      res.writeHead(404, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ error: { message: 'not found' } }));
      return;
    }

    const chunks = [];
    for await (const chunk of req) {
      chunks.push(chunk);
    }

    let body = {};
    try {
      body = JSON.parse(Buffer.concat(chunks).toString('utf8') || '{}');
    } catch (error) {
      res.writeHead(400, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ error: { message: 'invalid json' } }));
      return;
    }

    const content = chooseMockContent(body);
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify(buildOpenAIResponse(content)));
  });

  return new Promise((resolve, reject) => {
    server.once('error', reject);
    server.listen(port, '127.0.0.1', () => resolve(server));
  });
}

async function stopMockProvider(server) {
  if (!server) {
    return;
  }
  await new Promise((resolve, reject) => {
    server.close((error) => {
      if (error) {
        reject(error);
        return;
      }
      resolve();
    });
  });
}

function makeJWT(secret = JWT_SECRET, expiresAt = null) {
  const now = new Date();
  const iatTime = expiresAt ? new Date(now.getTime() - 3600_000) : now;
  const expTime = expiresAt || new Date(now.getTime() + 12 * 3600_000);
  const header = Buffer.from(JSON.stringify({ alg: 'HS256', typ: 'JWT' })).toString('base64url');
  const payload = Buffer.from(JSON.stringify({
    sub: 'playwright-e2e',
    iat: Math.floor(iatTime.getTime() / 1000),
    exp: Math.floor(expTime.getTime() / 1000),
  })).toString('base64url');
  const signingInput = `${header}.${payload}`;
  const signature = crypto.createHmac('sha256', secret).update(signingInput).digest('base64url');
  return `${signingInput}.${signature}`;
}

async function removeIfExists(targetPath) {
  await fs.rm(targetPath, { force: true }).catch((error) => {
    if (error && error.code !== 'ENOENT') {
      throw error;
    }
  });
}

async function resetDatabase(dbPath = DB_PATH) {
  await fs.mkdir(path.dirname(dbPath), { recursive: true });
  await Promise.all([
    removeIfExists(dbPath),
    removeIfExists(`${dbPath}-shm`),
    removeIfExists(`${dbPath}-wal`),
  ]);
}

function createIsolatedDBPath() {
  const uniqueName = `e2e-${process.pid}-${Date.now()}-${Math.random().toString(16).slice(2)}.db`;
  return path.resolve(REPO_ROOT, 'testdata', uniqueName);
}

async function waitForHealth(url = `${SERVER_BASE_URL}/healthz`, timeoutMs = 30_000) {
  const deadline = Date.now() + timeoutMs;
  let lastError = null;
  while (Date.now() < deadline) {
    try {
      const response = await fetch(url);
      if (response.ok) {
        return;
      }
      lastError = new Error(`Health endpoint returned ${response.status}`);
    } catch (error) {
      lastError = error;
    }
    await new Promise((resolve) => setTimeout(resolve, 250));
  }
  throw lastError || new Error('Timed out waiting for server health endpoint');
}

async function validateWebUIAssets(port, timeoutMs = 10_000) {
  const deadline = Date.now() + timeoutMs;
  let lastError = null;
  while (Date.now() < deadline) {
    try {
      const response = await fetch(`http://127.0.0.1:${port}/web/js/app.js`);
      const contentType = String(response.headers.get('content-type') || '').toLowerCase();
      const body = await response.text();
      if (response.ok && contentType.includes('javascript') && body.includes('handleConnect') && body.includes('bindEvents')) {
        return;
      }
      lastError = new Error(`Web UI asset validation failed: status=${response.status} content-type=${contentType || 'missing'}`);
    } catch (error) {
      lastError = error;
    }
    await new Promise((resolve) => setTimeout(resolve, 250));
  }
  throw lastError || new Error('Timed out waiting for Web UI assets');
}

function spawnServerProcess(mode, options) {
  const { port, dbPath, secret, providerBaseURL } = options;
  const prebuiltPath = path.resolve(REPO_ROOT, 'testdata', 'e2e-server.exe');
  if (mode === 'prebuilt') {
    const binArgs = [
      '--db', dbPath,
      '--jwt-secret', secret,
      '--provider', 'openai',
      '--model', 'gpt-4.1-mini',
      '--api-key', 'e2e-test-key',
      '--base-url', providerBaseURL,
      '--verify-mode', 'off',
      '--web-ui-enabled',
      '--server-enable-wal',
      '--listen-address', `127.0.0.1:${port}`,
    ];
    return spawn(prebuiltPath, binArgs, {
      cwd: REPO_ROOT,
      env: { ...process.env },
      stdio: ['ignore', 'pipe', 'pipe'],
    });
  }

  const args = [
    'run', './cmd/server',
    '--db', dbPath, '--jwt-secret', secret,
    '--provider', 'openai', '--model', 'gpt-4.1-mini',
    '--api-key', 'e2e-test-key', '--base-url', providerBaseURL,
    '--verify-mode', 'off', '--web-ui-enabled', '--server-enable-wal',
    '--listen-address', `127.0.0.1:${port}`,
  ];
  return spawn('go', args, {
    cwd: REPO_ROOT,
    env: { ...process.env, GOPROXY: process.env.GOPROXY || 'off', GONOSUMCHECK: '*', GONOSUMDB: '*', GOPRIVATE: '*' },
    stdio: ['ignore', 'pipe', 'pipe'],
  });
}

async function bootServerProcess(mode, options, mockProvider) {
  const child = spawnServerProcess(mode, options);
  let output = '';
  const collect = (chunk) => {
    output += chunk.toString();
  };
  child.stdout.on('data', collect);
  child.stderr.on('data', collect);

  try {
    await waitForHealth(`http://127.0.0.1:${options.port}/healthz`);
    await validateWebUIAssets(options.port);
  } catch (error) {
    child.__e2eOutput = () => output;
    await stopServer(child);
    throw new Error(`Failed to start Go server (${mode}): ${error.message}\n${output}`.trim());
  }

  child.__e2eOutput = () => output;
  child.__mockProvider = mockProvider;
  return child;
}

async function startServer(options = {}) {
  const port = options.port || SERVER_PORT;
  const dbPath = options.dbPath || createIsolatedDBPath();
  const secret = options.jwtSecret || JWT_SECRET;
  const providerBaseURL = options.providerBaseURL || MOCK_PROVIDER_BASE_URL;
  await resetDatabase(dbPath);

  const mockProvider = await startMockProvider();

  const prebuiltPath = path.resolve(REPO_ROOT, 'testdata', 'e2e-server.exe');
  try {
    await fs.access(prebuiltPath);
    try {
      return await bootServerProcess('prebuilt', { port, dbPath, secret, providerBaseURL }, mockProvider);
    } catch (error) {
      if (!/Web UI asset validation failed|Timed out waiting for Web UI assets/i.test(error.message)) {
        await stopMockProvider(mockProvider);
        throw error;
      }
    }
  } catch {
  }

  try {
    return await bootServerProcess('go-run', { port, dbPath, secret, providerBaseURL }, mockProvider);
  } catch (error) {
    await stopMockProvider(mockProvider);
    throw error;
  }
}

async function stopServer(child) {
  if (!child) {
    return;
  }

  const mockProvider = child.__mockProvider;
  delete child.__mockProvider;

  if (child.exitCode === null) {
    await new Promise((resolve) => {
      const timer = setTimeout(() => {
        if (child.exitCode === null) {
          child.kill('SIGKILL');
        }
        resolve();
      }, 10_000);

      child.once('exit', () => {
        clearTimeout(timer);
        resolve();
      });

      child.kill('SIGTERM');
    });
  }

  await stopMockProvider(mockProvider);
}

module.exports = {
  DB_PATH,
  JWT_SECRET,
  SERVER_BASE_URL,
  createIsolatedDBPath,
  makeJWT,
  resetDatabase,
  startServer,
  stopServer,
};
