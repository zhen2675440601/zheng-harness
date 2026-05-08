(function () {
  var auth = window.ZhengAuth || {};
  var api = window.ZhengAPI || {};
  var activeStream = null;
  var isAuthBusy = false;
  var isBootstrappingAuth = false;
  var isTaskSubmitting = false;
  var isResumeSubmitting = false;
  var dashboardState = {
    page: 1,
    pageSize: 10,
    status: '',
  };

  function byId(id) {
    return document.getElementById(id);
  }

  var els = {
    authScreen: byId('auth-screen'),
    authError: byId('auth-error'),
    authStatusConnected: byId('auth-status-connected'),
    mainContent: byId('main-content'),
    disconnectBtn: byId('disconnect-btn'),
    jwtInput: byId('jwt-input'),
    connectBtn: byId('connect-btn'),
    taskForm: byId('task-form'),
    taskInput: byId('task-input'),
    taskType: byId('task-type'),
    taskValidationError: byId('task-validation-error'),
    sessionList: byId('session-list'),
    pageDashboard: byId('page-dashboard'),
    pageTask: byId('page-task'),
    pageStream: byId('page-stream'),
    pageDetail: byId('page-detail'),
    sessionId: byId('session-id'),
    sessionStream: byId('session-stream'),
    sessionDetail: byId('session-detail'),
    streamDisconnected: byId('stream-disconnected'),
    streamEvents: byId('stream-events'),
    viewDetailLink: byId('view-detail-link'),
    resumeBtn: byId('resume-btn'),
    runTaskBtn: byId('run-task-btn'),
    taskAdvancedToggle: null,
    taskAdvancedFields: null,
  };

  function setVisible(element, visible, displayValue) {
    if (!element) {
      return;
    }
    element.style.display = visible ? (displayValue || '') : 'none';
  }

  function setText(element, value) {
    if (element) {
      element.textContent = value;
    }
  }

  function setHTML(element, html) {
    if (element) {
      element.innerHTML = html;
    }
  }

  function escapeHTML(value) {
    return String(value == null ? '' : value)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#39;');
  }

  function clearAuthError() {
    setText(els.authError, '');
    setVisible(els.authError, false);
  }

  function showAuthError(message) {
    setText(els.authError, message);
    setVisible(els.authError, true);
  }

  function clearTaskValidationError() {
    setText(els.taskValidationError, '');
    setVisible(els.taskValidationError, false);
  }

  function showTaskValidationError(message) {
    setText(els.taskValidationError, message);
    setVisible(els.taskValidationError, true);
  }

  function getErrorStatus(error) {
    var message = error && error.message ? String(error.message) : '';
    var match = message.match(/\b(400|404|409|429|500)\b/);
    if (match) {
      return Number(match[1]);
    }
    if (/not found/i.test(message)) {
      return 404;
    }
    if (/already running/i.test(message)) {
      return 409;
    }
    if (/too many/i.test(message)) {
      return 429;
    }
    return 0;
  }

  function getErrorMessage(error, fallbackMessage) {
    var message = error && error.message ? String(error.message).trim() : '';
    if (!message) {
      return fallbackMessage;
    }
    return message;
  }

  function ensureDetailActionMessage() {
    if (!els.sessionDetail) {
      return null;
    }
    var existing = byId('detail-action-message');
    if (existing) {
      return existing;
    }
    var message = document.createElement('div');
    message.id = 'detail-action-message';
    message.className = 'placeholder-copy';
    message.style.display = 'none';
    els.sessionDetail.appendChild(message);
    return message;
  }

  function clearDetailActionMessage() {
    var message = ensureDetailActionMessage();
    if (!message) {
      return;
    }
    setText(message, '');
    setVisible(message, false);
  }

  function showDetailActionMessage(messageText) {
    var message = ensureDetailActionMessage();
    if (!message) {
      return;
    }
    setText(message, messageText);
    setVisible(message, true, 'block');
  }

  function setTaskSubmitButtonState(loading) {
    isTaskSubmitting = !!loading;
    if (!els.runTaskBtn) {
      return;
    }
    els.runTaskBtn.disabled = !!loading;
    setText(els.runTaskBtn, loading ? 'Submitting...' : 'Run Task');
  }

  function setResumeButtonState(loading) {
    isResumeSubmitting = !!loading;
    if (!els.resumeBtn) {
      return;
    }
    els.resumeBtn.disabled = !!loading;
    setText(els.resumeBtn, loading ? 'Resuming...' : 'Resume');
  }

  function createAdvancedField(labelText, inputElement) {
    var wrapper = document.createElement('div');
    wrapper.className = 'form-group';

    var label = document.createElement('label');
    label.setAttribute('for', inputElement.id);
    label.textContent = labelText;

    wrapper.appendChild(label);
    wrapper.appendChild(inputElement);
    return wrapper;
  }

  function ensureTaskAdvancedOptions() {
    if (!els.taskForm) {
      return;
    }
    if (els.taskAdvancedFields && els.taskAdvancedToggle) {
      return;
    }

    var toggle = document.createElement('button');
    toggle.type = 'button';
    toggle.id = 'task-advanced-toggle';
    toggle.className = 'secondary-button';
    toggle.setAttribute('aria-expanded', 'false');
    toggle.textContent = 'Advanced options';

    var advancedFields = document.createElement('div');
    advancedFields.id = 'task-advanced-fields';
    advancedFields.style.display = 'none';

    var providerInput = document.createElement('input');
    providerInput.type = 'text';
    providerInput.id = 'task-provider';
    providerInput.setAttribute('aria-label', 'Provider');

    var modelInput = document.createElement('input');
    modelInput.type = 'text';
    modelInput.id = 'task-model';
    modelInput.setAttribute('aria-label', 'Model');

    var maxStepsInput = document.createElement('input');
    maxStepsInput.type = 'number';
    maxStepsInput.id = 'task-max-steps';
    maxStepsInput.setAttribute('aria-label', 'Max Steps');
    maxStepsInput.min = '1';
    maxStepsInput.step = '1';

    var verifyModeSelect = document.createElement('select');
    verifyModeSelect.id = 'task-verify-mode';
    verifyModeSelect.setAttribute('aria-label', 'Verify Mode');
    ['standard', 'strict', 'off'].forEach(function (value) {
      var option = document.createElement('option');
      option.value = value;
      option.textContent = value;
      if (value === 'standard') {
        option.selected = true;
      }
      verifyModeSelect.appendChild(option);
    });

    advancedFields.appendChild(createAdvancedField('Provider', providerInput));
    advancedFields.appendChild(createAdvancedField('Model', modelInput));
    advancedFields.appendChild(createAdvancedField('Max Steps', maxStepsInput));
    advancedFields.appendChild(createAdvancedField('Verify Mode', verifyModeSelect));

    toggle.addEventListener('click', function () {
      var isOpen = advancedFields.style.display !== 'none';
      setVisible(advancedFields, !isOpen, 'block');
      toggle.setAttribute('aria-expanded', isOpen ? 'false' : 'true');
      toggle.textContent = isOpen ? 'Advanced options' : 'Hide advanced options';
    });

    var validationError = els.taskValidationError;
    if (validationError && validationError.parentNode === els.taskForm) {
      els.taskForm.insertBefore(toggle, validationError);
      els.taskForm.insertBefore(advancedFields, validationError);
    } else {
      els.taskForm.appendChild(toggle);
      els.taskForm.appendChild(advancedFields);
    }

    els.taskAdvancedToggle = toggle;
    els.taskAdvancedFields = advancedFields;
  }

  function disconnectStream() {
    if (activeStream && typeof activeStream.close === 'function') {
      activeStream.close();
    }
    activeStream = null;
  }

  function isConnected() {
    return typeof auth.isConnected === 'function' && auth.isConnected();
  }

  function setMainContentEnabled(enabled) {
    if (!els.mainContent || !els.mainContent.querySelectorAll) {
      return;
    }
    var controls = els.mainContent.querySelectorAll('button, input, select, textarea');
    Array.prototype.forEach.call(controls, function (control) {
      if (!control || control.id === 'disconnect-btn') {
        return;
      }
      control.disabled = !enabled;
    });
  }

  function setConnectButtonState(loading) {
    isAuthBusy = !!loading;
    if (!els.connectBtn) {
      return;
    }
    els.connectBtn.disabled = !!loading;
    setText(els.connectBtn, loading ? 'Connecting...' : 'Connect');
  }

  function resetAuthUI(options) {
    var settings = options || {};
    clearAuthError();
    setVisible(els.authStatusConnected, false);
    setConnectButtonState(false);
    if (settings.clearInput && els.jwtInput) {
      els.jwtInput.value = '';
    }
  }

  function renderAuthState() {
    var connected = isConnected();
    setVisible(els.authScreen, !connected, 'flex');
    setVisible(els.mainContent, connected, 'block');
    setVisible(els.disconnectBtn, connected, 'inline-block');
    setVisible(els.authStatusConnected, connected, 'block');
    setMainContentEnabled(connected);
    if (!connected) {
      disconnectStream();
    }
  }

  function showPage(pageName) {
    var pages = {
      dashboard: els.pageDashboard,
      task: els.pageTask,
      stream: els.pageStream,
      detail: els.pageDetail,
    };

    Object.keys(pages).forEach(function (key) {
      setVisible(pages[key], key === pageName, 'block');
    });

    var navDashboard = document.querySelector('[data-test="nav-dashboard"]');
    var navTask = document.querySelector('[data-test="nav-task"]');
    if (navDashboard) {
      navDashboard.classList.toggle('active', pageName === 'dashboard' || pageName === 'detail' || pageName === 'stream');
    }
    if (navTask) {
      navTask.classList.toggle('active', pageName === 'task');
    }
  }

  function renderDashboardPlaceholder() {
    setHTML(els.sessionList, '<p class="placeholder-copy">Session dashboard wiring is ready. Listing data will populate here once dashboard rendering lands.</p>');
  }

  function normalizeStatus(status) {
    return String(status || 'unknown').toLowerCase();
  }

  function formatStatusLabel(status) {
    var normalized = normalizeStatus(status);
    if (!normalized) {
      return 'Unknown';
    }
    return normalized.charAt(0).toUpperCase() + normalized.slice(1);
  }

  function getStatusColor(status) {
    var normalized = normalizeStatus(status);
    if (normalized === 'completed') {
      return 'green';
    }
    if (normalized === 'running' || normalized === 'created') {
      return '#2563eb';
    }
    if (normalized === 'failed') {
      return 'red';
    }
    if (normalized === 'cancelled') {
      return 'gray';
    }
    return 'inherit';
  }

  function renderStatusBadge(status) {
    return '<span style="color: ' + escapeHTML(getStatusColor(status)) + '; font-weight: 600;">' + escapeHTML(formatStatusLabel(status)) + '</span>';
  }

  function formatTimestamp(value) {
    if (!value) {
      return '—';
    }
    var date = new Date(value);
    if (Number.isNaN(date.getTime())) {
      return String(value);
    }
    return date.toLocaleString();
  }

  function truncateText(value, maxLength) {
    var text = String(value == null ? '' : value);
    if (text.length <= maxLength) {
      return text;
    }
    return text.slice(0, Math.max(0, maxLength - 1)).trimEnd() + '…';
  }

  function pickSessionID(payload, fallback) {
    if (payload && payload.session_id) {
      return String(payload.session_id);
    }
    if (payload && payload.id) {
      return String(payload.id);
    }
    return String(fallback || '');
  }

  function stringifyValue(value) {
    if (value == null) {
      return '';
    }
    if (typeof value === 'string') {
      return value;
    }
    try {
      return JSON.stringify(value, null, 2);
    } catch (error) {
      return String(value);
    }
  }

  function summarizePlan(plan) {
    if (plan == null) {
      return '—';
    }
    if (typeof plan === 'string') {
      return plan || '—';
    }
    if (Array.isArray(plan)) {
      return plan.map(function (item) {
        return typeof item === 'string' ? item : stringifyValue(item);
      }).join('\n');
    }
    if (plan.summary) {
      return String(plan.summary);
    }
    if (Array.isArray(plan.steps) && plan.steps.length) {
      return plan.steps.map(function (step, index) {
        if (typeof step === 'string') {
          return (index + 1) + '. ' + step;
        }
        return (index + 1) + '. ' + stringifyValue(step);
      }).join('\n');
    }
    return stringifyValue(plan) || '—';
  }

  function summarizeObservation(observation) {
    if (observation == null) {
      return '—';
    }
    if (typeof observation === 'string') {
      return truncateText(observation, 240) || '—';
    }
    if (observation.summary) {
      return truncateText(String(observation.summary), 240) || '—';
    }
    if (observation.message) {
      return truncateText(String(observation.message), 240) || '—';
    }
    return truncateText(stringifyValue(observation), 240) || '—';
  }

  function hasProvenanceData(provenance) {
    if (provenance == null) {
      return false;
    }
    if (typeof provenance === 'string') {
      return provenance.trim() !== '';
    }
    if (Array.isArray(provenance)) {
      return provenance.length > 0;
    }
    if (typeof provenance === 'object') {
      return Object.keys(provenance).length > 0;
    }
    return true;
  }

  function buildDetailItem(label, value) {
    return '<div class="detail-item"><span class="detail-label">' + escapeHTML(label) + '</span><div class="detail-value">' + value + '</div></div>';
  }

  function bindDashboardControls() {
    if (!els.sessionList) {
      return;
    }
    var filterButtons = els.sessionList.querySelectorAll('[data-dashboard-status]');
    Array.prototype.forEach.call(filterButtons, function (button) {
      button.addEventListener('click', function () {
        dashboardState.status = button.getAttribute('data-dashboard-status') || '';
        dashboardState.page = 1;
        loadDashboard();
      });
    });

    var paginationButtons = els.sessionList.querySelectorAll('[data-dashboard-page]');
    Array.prototype.forEach.call(paginationButtons, function (button) {
      button.addEventListener('click', function () {
        var nextPage = Number(button.getAttribute('data-dashboard-page') || dashboardState.page);
        if (!nextPage || nextPage < 1 || nextPage === dashboardState.page) {
          return;
        }
        dashboardState.page = nextPage;
        loadDashboard();
      });
    });
  }

  function setStreamStateMarkup(message, className) {
    var classes = ['detail-item'];
    if (className) {
      classes.push(className);
    }
    setHTML(els.sessionStream, '<div class="' + classes.join(' ') + '">' + escapeHTML(message) + '</div>');
  }

  function setStreamDisconnectedMessage(message) {
    setText(els.streamDisconnected, message || '');
    setVisible(els.streamDisconnected, !!message, 'block');
  }

  function clearStreamEvents() {
    if (els.streamEvents) {
      els.streamEvents.innerHTML = '';
    }
  }

  function createStreamEventEntry(eventType, bodyHTML) {
    var entry = document.createElement('div');
    entry.className = 'detail-item stream-event stream-event-' + eventType;
    entry.setAttribute('data-event-type', eventType);
    entry.innerHTML = bodyHTML;
    return entry;
  }

  function appendStreamEvent(eventType, bodyHTML) {
    if (!els.streamEvents) {
      return null;
    }
    var entry = createStreamEventEntry(eventType, bodyHTML);
    els.streamEvents.appendChild(entry);
    while (els.streamEvents.childNodes.length > 100) {
      els.streamEvents.removeChild(els.streamEvents.firstChild);
    }
    return entry;
  }

  function ensureTokenDeltaEntry() {
    if (!els.streamEvents) {
      return null;
    }
    var existing = els.streamEvents.querySelector('[data-event-type="token_delta"][data-token-buffer="true"]');
    if (existing) {
      return existing;
    }
    var entry = appendStreamEvent('token_delta',
      '<span class="detail-label">TOKEN STREAM</span>' +
      '<pre class="detail-value stream-token-buffer"></pre>');
    if (entry) {
      entry.setAttribute('data-token-buffer', 'true');
      entry.setAttribute('data-token-text', '');
    }
    return entry;
  }

  function updateTokenDelta(delta) {
    var entry = ensureTokenDeltaEntry();
    if (!entry) {
      return;
    }
    var nextText = (entry.getAttribute('data-token-text') || '') + String(delta == null ? '' : delta);
    entry.setAttribute('data-token-text', nextText);
    var buffer = entry.querySelector('.stream-token-buffer');
    if (buffer) {
      buffer.textContent = nextText;
    }
  }

  function renderToolStartEvent(event) {
    appendStreamEvent('tool_start',
      '<span class="detail-label">TOOL START · step ' + escapeHTML(event.step || '?') + '</span>' +
      '<div class="detail-value">' + escapeHTML(event.tool || 'unknown tool') + '</div>' +
      '<pre>' + escapeHTML(event.input || '') + '</pre>');
  }

  function renderToolEndEvent(event) {
    var outputSummary = event && event.error
      ? 'Error: ' + String(event.error)
      : (event && event.output ? String(event.output) : 'No output');
    appendStreamEvent('tool_end',
      '<span class="detail-label">TOOL END · step ' + escapeHTML(event.step || '?') + '</span>' +
      '<div class="detail-value">' + escapeHTML(event.tool || 'unknown tool') + '</div>' +
      '<pre>' + escapeHTML(outputSummary) + '</pre>');
  }

  function renderStepCompleteEvent(event) {
    appendStreamEvent('step_complete',
      '<span class="detail-label">STEP COMPLETE · step ' + escapeHTML(event.step || '?') + '</span>' +
      '<div class="detail-value">' + escapeHTML(event.summary || 'Step completed.') + '</div>');
  }

  function renderErrorEvent(event) {
    appendStreamEvent('error',
      '<span class="detail-label">ERROR · step ' + escapeHTML(event.step || '?') + '</span>' +
      '<div class="detail-value">' + escapeHTML(event.message || 'Unknown stream error.') + '</div>');
  }

  function renderSessionCompleteEvent(event, sessionId) {
    var finalStatus = event && event.status ? String(event.status) : 'unknown';
    setStreamDisconnectedMessage('');
    setStreamStateMarkup('Session complete: ' + finalStatus, 'stream-complete');
    appendStreamEvent('session_complete',
      '<span class="detail-label">SESSION COMPLETE</span>' +
      '<div class="detail-value">Final status: ' + escapeHTML(finalStatus) + '</div>');
    if (els.viewDetailLink) {
      els.viewDetailLink.href = '#/detail/' + encodeURIComponent(pickSessionID(event, sessionId));
      setVisible(els.viewDetailLink, true, 'inline-block');
    }
  }

  function renderFallbackStreamEvent(event) {
    appendStreamEvent('unknown',
      '<span class="detail-label">STREAM EVENT</span>' +
      '<pre>' + escapeHTML(JSON.stringify(event || {}, null, 2)) + '</pre>');
  }

  function handleStreamEvent(event, sessionId) {
    var eventType = event && event.type ? String(event.type) : 'unknown';
    var payload = event && event.data && typeof event.data === 'object' ? event.data : (event || {});
    if (eventType === 'token_delta') {
      updateTokenDelta(payload && payload.delta);
      return;
    }
    if (eventType === 'tool_start') {
      renderToolStartEvent(payload || {});
      return;
    }
    if (eventType === 'tool_end') {
      renderToolEndEvent(payload || {});
      return;
    }
    if (eventType === 'step_complete') {
      renderStepCompleteEvent(payload || {});
      return;
    }
    if (eventType === 'error') {
      renderErrorEvent(payload || {});
      return;
    }
    if (eventType === 'session_complete') {
      renderSessionCompleteEvent(payload || {}, sessionId);
      return;
    }
    renderFallbackStreamEvent(payload);
  }

  async function loadDashboard() {
    var statusFilters = [
      { label: 'All', value: '' },
      { label: 'Created', value: 'created' },
      { label: 'Running', value: 'running' },
      { label: 'Completed', value: 'completed' },
      { label: 'Failed', value: 'failed' },
      { label: 'Cancelled', value: 'cancelled' },
    ];
    setHTML(els.sessionList, '<p class="placeholder-copy">Loading sessions...</p>');
    if (!isConnected() || typeof api.apiListSessions !== 'function') {
      return;
    }
    try {
      var payload = await api.apiListSessions(dashboardState.page, dashboardState.pageSize, dashboardState.status);
      var sessions = payload && Array.isArray(payload.sessions) ? payload.sessions : [];
      var page = payload && payload.page ? Number(payload.page) : dashboardState.page;
      var pageSize = payload && payload.page_size ? Number(payload.page_size) : dashboardState.pageSize;
      var total = payload && typeof payload.total === 'number' ? payload.total : sessions.length;
      var totalPages = Math.max(1, Math.ceil((total || 0) / (pageSize || dashboardState.pageSize || 10)));
      dashboardState.page = Math.min(Math.max(page || 1, 1), totalPages);
      dashboardState.pageSize = pageSize || dashboardState.pageSize;

      var filtersHTML = '<div class="card"><div style="display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 8px;">' + statusFilters.map(function (filter) {
        var isActive = filter.value === dashboardState.status;
        return '<button class="secondary-button" data-dashboard-status="' + escapeHTML(filter.value) + '"' + (isActive ? ' aria-pressed="true" style="font-weight: 600;"' : '') + '>' + escapeHTML(filter.label) + '</button>';
      }).join('') + '</div></div>';

      var sessionsHTML = sessions.length ? sessions.map(function (session) {
        var sessionID = escapeHTML(pickSessionID(session, ''));
        var task = escapeHTML(truncateText(session.task || 'Untitled task', 80));
        var status = renderStatusBadge(session.status || 'unknown');
        var type = escapeHTML(session.task_type || 'general');
        var timestamp = escapeHTML(formatTimestamp(session.updated_at || session.created_at));
        return '<div class="card">' +
          '<div class="detail-grid">' +
            buildDetailItem('Status', status) +
            buildDetailItem('Task Type', type) +
            buildDetailItem('Timestamp', timestamp) +
          '</div>' +
          '<div class="detail-item">' +
            '<span class="detail-label">Task</span>' +
            '<div class="detail-value">' + task + '</div>' +
          '</div>' +
          '<div><a href="#/detail/' + sessionID + '">View details</a></div>' +
        '</div>';
      }).join('') : '<div class="card"><p class="placeholder-copy">No sessions found</p></div>';

      var statusCounts = {
        created: 0,
        running: 0,
        completed: 0,
        failed: 0,
        cancelled: 0,
      };
      sessions.forEach(function (session) {
        var key = normalizeStatus(session && session.status);
        if (Object.prototype.hasOwnProperty.call(statusCounts, key)) {
          statusCounts[key] += 1;
        }
      });
      var statsHTML = '<div class="dashboard-stats">' +
        '<div class="stat-chip"><span class="stat-label">Created</span><span class="stat-value">' + escapeHTML(statusCounts.created) + '</span></div>' +
        '<div class="stat-chip"><span class="stat-label">Running</span><span class="stat-value">' + escapeHTML(statusCounts.running) + '</span></div>' +
        '<div class="stat-chip"><span class="stat-label">Completed</span><span class="stat-value">' + escapeHTML(statusCounts.completed) + '</span></div>' +
        '<div class="stat-chip"><span class="stat-label">Failed</span><span class="stat-value">' + escapeHTML(statusCounts.failed) + '</span></div>' +
        '<div class="stat-chip"><span class="stat-label">Cancelled</span><span class="stat-value">' + escapeHTML(statusCounts.cancelled) + '</span></div>' +
      '</div>';

      var previousPage = dashboardState.page > 1 ? dashboardState.page - 1 : 1;
      var nextPage = dashboardState.page < totalPages ? dashboardState.page + 1 : totalPages;
      var paginationHTML = '<div class="card"><div style="display: flex; align-items: center; gap: 12px; flex-wrap: wrap;">' +
        '<button class="secondary-button" data-dashboard-page="' + escapeHTML(previousPage) + '"' + (dashboardState.page <= 1 ? ' disabled' : '') + '>&#8592; Previous</button>' +
        '<span>Page ' + escapeHTML(dashboardState.page) + ' of ' + escapeHTML(totalPages) + '</span>' +
        '<button class="secondary-button" data-dashboard-page="' + escapeHTML(nextPage) + '"' + (dashboardState.page >= totalPages ? ' disabled' : '') + '>Next &#8594;</button>' +
      '</div></div>';

      setHTML(els.sessionList, statsHTML + filtersHTML + sessionsHTML + paginationHTML);
      bindDashboardControls();
    } catch (error) {
      setHTML(els.sessionList, '<div class="card"><p class="placeholder-copy">Unable to load sessions: ' + escapeHTML(error && error.message ? error.message : 'unknown error') + '</p></div>');
    }
  }

  function renderStreamShell(sessionId) {
    setText(els.sessionId, sessionId);
    setStreamStateMarkup('Connecting to session stream...', 'placeholder-copy');
    clearStreamEvents();
    setStreamDisconnectedMessage('');
    if (els.viewDetailLink) {
      els.viewDetailLink.href = '#/detail/' + encodeURIComponent(sessionId);
      setVisible(els.viewDetailLink, false, 'inline-block');
    }
  }

  function startStreamPreview(sessionId) {
    disconnectStream();
    if (!isConnected() || typeof api.apiStream !== 'function') {
      return;
    }
    try {
      activeStream = api.apiStream(sessionId, {
        onOpen: function () {
          setStreamDisconnectedMessage('');
          setStreamStateMarkup('Streaming live session output...', 'stream-live');
        },
        onEvent: function (event) {
          handleStreamEvent(event, sessionId);
        },
        onError: function (error) {
          if (error && error.name === 'AbortError') {
            return;
          }
          var message = 'Stream disconnected. The session continues running. View detail for current status.';
          var detail = getErrorMessage(error, '');
          if (detail) {
            message += ' Error: ' + detail;
          }
          setStreamDisconnectedMessage(message);
        },
      });
    } catch (error) {
      setStreamDisconnectedMessage('Stream disconnected. The session continues running. View detail for current status. Error: ' + getErrorMessage(error, 'Unable to open stream.'));
    }
  }

  async function loadDetail(sessionId) {
    setVisible(els.resumeBtn, false);
    setResumeButtonState(false);
    setHTML(els.sessionDetail, '<p class="placeholder-copy">Loading session detail...</p>');
    if (!isConnected() || typeof api.apiInspect !== 'function') {
      return;
    }
    try {
      var payload = await api.apiInspect(sessionId);
      var steps = payload && Array.isArray(payload.steps) ? payload.steps : [];
      var hasProvenance = hasProvenanceData(payload && payload.provenance);
      var html = [
        '<div class="card">',
        '<div class="detail-grid">',
        buildDetailItem('Session ID', escapeHTML(pickSessionID(payload, sessionId))),
        buildDetailItem('Status', renderStatusBadge(payload.status || 'unknown')),
        buildDetailItem('Task Type', escapeHTML(payload.task_type || 'general')),
        buildDetailItem('Created', escapeHTML(formatTimestamp(payload.created_at))),
        buildDetailItem('Updated', escapeHTML(formatTimestamp(payload.updated_at))),
        '</div>',
        '<div class="detail-item"><span class="detail-label">Task description</span><div class="detail-value">' + escapeHTML(payload.task || '') + '</div></div>',
        '<div class="detail-item"><span class="detail-label">Plan summary</span><div class="detail-value"><pre>' + escapeHTML(summarizePlan(payload.plan)) + '</pre></div></div>',
        '</div>',
        '<div class="card">',
        '<div class="detail-item"><span class="detail-label">Steps</span><div class="detail-value">',
        steps.length ? steps.map(function (step) {
          return '<div class="detail-item">' +
            '<div class="detail-grid">' +
              buildDetailItem('Step #', escapeHTML(step && step.step_number != null ? step.step_number : '—')) +
              buildDetailItem('Tool name', escapeHTML((step && step.tool_name) || '—')) +
              buildDetailItem('Verification status', renderStatusBadge((step && step.status) || 'unknown')) +
            '</div>' +
            '<div class="detail-item"><span class="detail-label">Observation summary</span><div class="detail-value">' + escapeHTML(summarizeObservation(step && step.observation)) + '</div></div>' +
          '</div>';
        }).join('') : '<p class="placeholder-copy">No steps recorded</p>',
        '</div></div>',
        '</div>',
      ];

      if (hasProvenance) {
        html.push(
          '<div class="card">' +
            '<div class="detail-item">' +
              '<span class="detail-label">Provenance</span>' +
              '<div class="detail-value"><pre>' + escapeHTML(stringifyValue(payload.provenance)) + '</pre></div>' +
            '</div>' +
          '</div>'
        );
      }

      if (payload && payload.termination_reason) {
        html.push(
          '<div class="card">' +
            '<div class="detail-item">' +
              '<span class="detail-label">Termination reason</span>' +
              '<div class="detail-value">' + escapeHTML(payload.termination_reason) + '</div>' +
            '</div>' +
          '</div>'
        );
      }

      setHTML(els.sessionDetail, html.join(''));
      var status = String((payload && payload.status) || '');
      setVisible(els.resumeBtn, status === 'created' || status === 'running', 'inline-block');
      if (els.resumeBtn) {
        els.resumeBtn.dataset.sessionId = sessionId;
      }
      clearDetailActionMessage();
    } catch (error) {
      setHTML(els.sessionDetail, '<p class="placeholder-copy">Unable to load session detail: ' + escapeHTML(error.message || 'unknown error') + '</p>');
    }
  }

  function parseRoute() {
    var rawHash = window.location.hash || '#/';
    var route = rawHash.replace(/^#/, '');
    if (!route || route === '/') {
      return { name: 'dashboard' };
    }
    if (route === '/task') {
      return { name: 'task' };
    }
    var streamMatch = route.match(/^\/stream\/([^/]+)$/);
    if (streamMatch) {
      return { name: 'stream', sessionId: decodeURIComponent(streamMatch[1]) };
    }
    var detailMatch = route.match(/^\/detail\/([^/]+)$/);
    if (detailMatch) {
      return { name: 'detail', sessionId: decodeURIComponent(detailMatch[1]) };
    }
    return { name: 'dashboard' };
  }

  async function renderRoute() {
    renderAuthState();
    if (!isConnected()) {
      return;
    }

    var route = parseRoute();
    showPage(route.name);

    if (route.name !== 'stream') {
      disconnectStream();
    }

    if (route.name === 'dashboard') {
      await loadDashboard();
      return;
    }

    if (route.name === 'task') {
      clearTaskValidationError();
      setTaskSubmitButtonState(false);
      return;
    }

    if (route.name === 'stream') {
      renderStreamShell(route.sessionId || '');
      startStreamPreview(route.sessionId || '');
      return;
    }

    if (route.name === 'detail') {
      await loadDetail(route.sessionId || '');
    }
  }

  async function connectWithToken(token, options) {
    var settings = options || {};
    var verification = { valid: false, error: 'Token verification failed' };
    setConnectButtonState(true);
    clearAuthError();
    setVisible(els.authStatusConnected, false);
    try {
      if (typeof auth.verifyToken === 'function') {
        verification = await auth.verifyToken(token);
      }
      if (!verification || !verification.valid) {
        if (typeof auth.clearToken === 'function') {
          auth.clearToken();
        }
        showAuthError((verification && verification.error) || 'Token verification failed');
        renderAuthState();
        return false;
      }
      if (typeof auth.setToken === 'function') {
        auth.setToken(token);
      }
      setVisible(els.authStatusConnected, true, 'block');
      clearAuthError();
      await renderRoute();
      return true;
    } finally {
      if (!settings.keepInput) {
        // no-op path retained for explicit behavior; failed attempts keep user input.
      }
      setConnectButtonState(false);
    }
  }

  async function handleConnect(event) {
    event.preventDefault();
    if (isAuthBusy) {
      return;
    }
    clearAuthError();
    var token = (els.jwtInput && els.jwtInput.value) ? els.jwtInput.value.trim() : '';
    if (!token) {
      showAuthError('JWT token is required.');
      return;
    }
    await connectWithToken(token, { keepInput: true });
  }

  function handleDisconnect(event) {
    if (event) {
      event.preventDefault();
    }
    if (typeof auth.clearToken === 'function') {
      auth.clearToken();
    }
    resetAuthUI({ clearInput: true });
    disconnectStream();
    window.location.hash = '#/';
    renderRoute();
  }

  async function handleTaskSubmit(event) {
    event.preventDefault();
    if (isTaskSubmitting || typeof api.apiRun !== 'function') {
      return;
    }
    var taskText = (els.taskInput && els.taskInput.value) ? els.taskInput.value.trim() : '';
    if (!taskText) {
      showTaskValidationError('Task description is required.');
      return;
    }
    clearTaskValidationError();

    var payload = {
      task: taskText,
      task_type: els.taskType && els.taskType.value ? els.taskType.value : 'coding',
    };
    var providerInput = byId('task-provider');
    var modelInput = byId('task-model');
    var maxStepsInput = byId('task-max-steps');
    var verifyModeSelect = byId('task-verify-mode');

    if (providerInput && providerInput.value.trim()) {
      payload.provider = providerInput.value.trim();
    }
    if (modelInput && modelInput.value.trim()) {
      payload.model = modelInput.value.trim();
    }
    if (maxStepsInput && maxStepsInput.value.trim()) {
      payload.max_steps = Number(maxStepsInput.value);
    }
    if (verifyModeSelect && verifyModeSelect.value) {
      payload.verify_mode = verifyModeSelect.value;
    }

    setTaskSubmitButtonState(true);
    try {
      var response = await api.apiRun(payload);
      var sessionId = response && response.session_id ? response.session_id : '';
      if (!sessionId) {
        throw new Error('Task started but no session ID was returned.');
      }
      if (els.taskForm) {
        els.taskForm.reset();
      }
      clearTaskValidationError();
      window.location.hash = '#/stream/' + encodeURIComponent(sessionId);
    } catch (error) {
      var status = getErrorStatus(error);
      var message = getErrorMessage(error, 'Task submission failed.');
      if (status === 400 || status === 409 || status === 429 || status === 500) {
        showTaskValidationError(message);
      } else {
        showTaskValidationError('Task submission failed: ' + message);
      }
    } finally {
      setTaskSubmitButtonState(false);
    }
  }

  async function handleResume(event) {
    event.preventDefault();
    if (isResumeSubmitting) {
      return;
    }
    var sessionId = event.currentTarget && event.currentTarget.dataset ? event.currentTarget.dataset.sessionId : '';
    if (!sessionId || typeof api.apiResume !== 'function') {
      return;
    }
    clearDetailActionMessage();
    setResumeButtonState(true);
    try {
      var response = await api.apiResume(sessionId);
      var nextSessionId = response && response.session_id ? response.session_id : sessionId;
      window.location.hash = '#/stream/' + encodeURIComponent(nextSessionId);
    } catch (error) {
      var status = getErrorStatus(error);
      if (status === 404) {
        showDetailActionMessage('Session not found');
      } else if (status === 409) {
        showDetailActionMessage('Session already running');
      } else if (status === 429) {
        showDetailActionMessage('Too many active sessions, try again later');
      } else {
        showDetailActionMessage('Resume failed: ' + getErrorMessage(error, 'unknown error'));
      }
    } finally {
      setResumeButtonState(false);
    }
  }

  function bindEvents() {
    ensureTaskAdvancedOptions();
    if (els.connectBtn) {
      els.connectBtn.addEventListener('click', handleConnect);
    }
    if (els.disconnectBtn) {
      els.disconnectBtn.classList.add('danger-button');
      els.disconnectBtn.addEventListener('click', handleDisconnect);
    }
    if (els.taskForm) {
      els.taskForm.addEventListener('submit', handleTaskSubmit);
    }
    if (els.resumeBtn) {
      els.resumeBtn.addEventListener('click', handleResume);
    }
    window.addEventListener('hashchange', renderRoute);
    window.addEventListener('zh:auth-required', function () {
      if (typeof auth.clearToken === 'function') {
        auth.clearToken();
      }
      resetAuthUI({ clearInput: true });
      showAuthError('Authentication expired or was rejected. Re-enter your JWT token to continue.');
      window.location.hash = '#/';
      renderRoute();
    });
    window.addEventListener('load', renderRoute);
  }

  async function bootstrapStoredToken() {
    if (isBootstrappingAuth || typeof auth.getToken !== 'function') {
      return;
    }
    var storedToken = auth.getToken();
    if (!storedToken) {
      resetAuthUI({ clearInput: true });
      renderAuthState();
      return;
    }
    if (els.jwtInput) {
      els.jwtInput.value = storedToken;
    }
    isBootstrappingAuth = true;
    if (typeof auth.clearToken === 'function') {
      auth.clearToken();
    }
    var connected = await connectWithToken(storedToken, { keepInput: true });
    if (!connected && els.jwtInput) {
      els.jwtInput.value = storedToken;
    }
    isBootstrappingAuth = false;
  }

  bootstrapStoredToken();
  bindEvents();

  // Expose internal functions for E2E testing
  window.__zhengApp = {
    connectWithToken: connectWithToken,
    renderRoute: renderRoute,
    renderAuthState: renderAuthState,
    handleStreamEvent: handleStreamEvent,
    handleConnect: handleConnect,
    handleDisconnect: handleDisconnect,
    loadDashboard: loadDashboard,
    loadDetail: loadDetail,
  };
})();
