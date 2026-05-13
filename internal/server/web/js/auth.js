(function () {
  var TOKEN_KEY = 'zhengHarness.jwt';
  var VERIFY_TIMEOUT_MS = 10000;

  function safeGetStorage() {
    try {
      return window.localStorage;
    } catch (error) {
      return null;
    }
  }

  function getToken() {
    var storage = safeGetStorage();
    if (!storage) {
      return '';
    }
    var token = storage.getItem(TOKEN_KEY);
    return typeof token === 'string' ? token.trim() : '';
  }

  function setToken(token) {
    var storage = safeGetStorage();
    var normalized = typeof token === 'string' ? token.trim() : '';
    if (!storage) {
      return normalized;
    }
    if (!normalized) {
      storage.removeItem(TOKEN_KEY);
      return '';
    }
    storage.setItem(TOKEN_KEY, normalized);
    return normalized;
  }

  function clearToken() {
    var storage = safeGetStorage();
    if (storage) {
      storage.removeItem(TOKEN_KEY);
    }
  }

  function parseErrorPayload(payload, fallback) {
    if (payload && payload.error && payload.error.message) {
      return payload.error.message;
    }
    return fallback;
  }

  function mapVerificationError(status, message) {
    var normalized = String(message || '').toLowerCase();
    if (status === 401) {
      if (normalized.indexOf('expired') >= 0) {
        return 'Token expired';
      }
      if (normalized.indexOf('invalid') >= 0) {
        return 'Invalid token';
      }
      return 'Invalid or expired token';
    }
    if (normalized.indexOf('expired') >= 0) {
      return 'Token expired';
    }
    if (normalized.indexOf('invalid') >= 0) {
      return 'Invalid token';
    }
    return message || 'Token verification failed';
  }

  async function requestJSON(path, options) {
    var response = await fetch(path, Object.assign({}, options || {}));
    var payload = null;
    try {
      payload = await response.json();
    } catch (error) {
      payload = null;
    }
    if (!response.ok) {
      throw new Error(parseErrorPayload(payload, response.status + ' ' + response.statusText));
    }
    return payload;
  }

  async function verifyToken(token) {
    var normalized = typeof token === 'string' ? token.trim() : '';
    if (!normalized) {
      return { valid: false, error: 'JWT token is required.' };
    }

    var controller = typeof AbortController === 'function' ? new AbortController() : null;
    var timeoutId = null;
    if (controller && typeof window.setTimeout === 'function') {
      timeoutId = window.setTimeout(function () {
        controller.abort();
      }, VERIFY_TIMEOUT_MS);
    }

    try {
      var response = await fetch('/api/v1/sessions?page_size=1', {
        method: 'GET',
        headers: {
          Authorization: 'Bearer ' + normalized,
        },
        signal: controller ? controller.signal : undefined,
      });

      if (response.ok) {
        return { valid: true, error: '' };
      }

      var message = '';
      try {
        var payload = await response.json();
        message = parseErrorPayload(payload, '');
      } catch (error) {
        message = '';
      }

      return {
        valid: false,
        error: mapVerificationError(response.status, message),
      };
    } catch (error) {
      if (error && error.name === 'AbortError') {
        return { valid: false, error: 'Cannot reach server - check the server is running' };
      }
      return { valid: false, error: 'Cannot reach server - check the server is running' };
    } finally {
      if (timeoutId) {
        window.clearTimeout(timeoutId);
      }
    }
  }

  async function login(username, password) {
    return requestJSON('/api/v1/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username: username, password: password }),
    });
  }

  async function register(username, password) {
    return requestJSON('/api/v1/auth/register', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username: username, password: password }),
    });
  }

  function isConnected() {
    return getToken().length > 0;
  }

  window.ZhengAuth = {
    getToken: getToken,
    setToken: setToken,
    clearToken: clearToken,
    verifyToken: verifyToken,
    login: login,
    register: register,
    isConnected: isConnected,
  };

  window.__zhengAuth = {
    getToken: getToken,
    setToken: setToken,
    clearToken: clearToken,
    verifyToken: verifyToken,
    login: login,
    register: register,
    isConnected: isConnected,
  };
})();
