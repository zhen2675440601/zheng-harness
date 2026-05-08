(function () {
  function getAuth() {
    return window.ZhengAuth || {};
  }

  function normalizePath(path) {
    if (!path) {
      return '/';
    }
    return path.charAt(0) === '/' ? path : '/' + path;
  }

  function unauthorizedError() {
    return new Error('unauthorized');
  }

  function notifyUnauthorized() {
    var auth = getAuth();
    if (typeof auth.clearToken === 'function') {
      auth.clearToken();
    }
    window.dispatchEvent(new CustomEvent('zh:auth-required'));
    if (window.location.hash !== '#/') {
      window.location.hash = '#/';
    }
  }

  async function request(path, options) {
    var auth = getAuth();
    var token = typeof auth.getToken === 'function' ? auth.getToken() : '';
    var headers = new Headers((options && options.headers) || {});
    if (token) {
      headers.set('Authorization', 'Bearer ' + token);
    }
    if (!headers.has('Content-Type') && options && options.body) {
      headers.set('Content-Type', 'application/json');
    }

    var response = await fetch(normalizePath(path), Object.assign({}, options || {}, {
      headers: headers,
    }));

    if (response.status === 401) {
      notifyUnauthorized();
      throw unauthorizedError();
    }

    if (!response.ok) {
      var message = 'Request failed';
      try {
        var payload = await response.json();
        if (payload && payload.error && payload.error.message) {
          message = payload.error.message;
        }
      } catch (error) {
        message = response.status + ' ' + response.statusText;
      }
      throw new Error(message);
    }

    if (response.status === 204) {
      return null;
    }

    return response.json();
  }

  function apiRun(task) {
    return request('/api/v1/run', {
      method: 'POST',
      body: JSON.stringify(task || {}),
    });
  }

  function apiResume(sessionId) {
    return request('/api/v1/resume', {
      method: 'POST',
      body: JSON.stringify({ session_id: sessionId }),
    });
  }

  function apiInspect(sessionId) {
    return request('/api/v1/sessions/' + encodeURIComponent(sessionId) + '/inspect');
  }

  function apiListSessions(page, pageSize, status) {
    var query = new URLSearchParams();
    query.set('page', String(page || 1));
    query.set('page_size', String(pageSize || 20));
    if (status) {
      query.set('status', String(status));
    }
    return request('/api/v1/sessions?' + query.toString());
  }

  function readSSEStream(response, handlers) {
    if (!response.body || typeof response.body.getReader !== 'function') {
      return Promise.resolve();
    }

    var reader = response.body.getReader();
    var decoder = new TextDecoder();
    var buffer = '';

    function emitEventBlock(block) {
      if (!block) {
        return;
      }
      var lines = block.split(/\r?\n/);
      var eventType = 'message';
      var dataLines = [];

      lines.forEach(function (line) {
        if (!line || line.charAt(0) === ':') {
          return;
        }
        if (line.indexOf('event:') === 0) {
          eventType = line.slice(6).trim();
          return;
        }
        if (line.indexOf('data:') === 0) {
          dataLines.push(line.slice(5).trim());
        }
      });

      var payload = dataLines.join('\n');
      var parsed = null;
      try {
        parsed = payload ? JSON.parse(payload) : null;
      } catch (error) {
        parsed = { raw: payload };
      }

      if (typeof handlers.onEvent === 'function') {
        handlers.onEvent({ type: eventType, data: parsed, raw: payload });
      }
    }

    function pump() {
      return reader.read().then(function (result) {
        if (result.done) {
          if (buffer.trim()) {
            emitEventBlock(buffer.trim());
          }
          return;
        }

        buffer += decoder.decode(result.value, { stream: true });
        var blocks = buffer.split(/\r?\n\r?\n/);
        buffer = blocks.pop() || '';
        blocks.forEach(function (block) {
          emitEventBlock(block.trim());
        });
        return pump();
      });
    }

    return pump();
  }

  function apiStream(sessionId, callbacks) {
    var auth = getAuth();
    var token = typeof auth.getToken === 'function' ? auth.getToken() : '';
    if (!token) {
      notifyUnauthorized();
      throw unauthorizedError();
    }

    var handlers = callbacks || {};
    var controller = typeof AbortController === 'function' ? new AbortController() : null;
    fetch(normalizePath('/api/v1/sessions/' + encodeURIComponent(sessionId) + '/stream'), {
      method: 'GET',
      headers: {
        Accept: 'text/event-stream',
        Authorization: 'Bearer ' + token,
      },
      signal: controller ? controller.signal : undefined,
    }).then(function (response) {
      if (response.status === 401) {
        notifyUnauthorized();
        throw unauthorizedError();
      }
      if (!response.ok) {
        throw new Error(response.status + ' ' + response.statusText);
      }
      if (typeof handlers.onOpen === 'function') {
        handlers.onOpen();
      }
      return readSSEStream(response, handlers);
    }).catch(function (error) {
      if (typeof handlers.onError === 'function') {
        handlers.onError(error);
      }
    });

    return {
      close: function () {
        if (controller) {
          controller.abort();
        }
      },
      controller: controller,
    };
  }

  window.ZhengAPI = {
    apiRun: apiRun,
    apiResume: apiResume,
    apiInspect: apiInspect,
    apiListSessions: apiListSessions,
    apiStream: apiStream,
  };
})();
