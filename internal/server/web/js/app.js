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

function setVisibleAuthManual(show) {
	var manual = document.getElementById('auth-manual');
	var auto = document.getElementById('auth-auto-connecting');
	if (manual) manual.style.display = show ? '' : 'none';
	if (auto) auto.style.display = show ? 'none' : '';
	if (!show && auto) auto.style.display = '';
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
    setText(els.runTaskBtn, loading ? '提交中...' : '开始执行');
  }

  function setResumeButtonState(loading) {
    isResumeSubmitting = !!loading;
    if (!els.resumeBtn) {
      return;
    }
    els.resumeBtn.disabled = !!loading;
    setText(els.resumeBtn, loading ? '继续中...' : '继续执行');
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
    toggle.textContent = '高级参数';

    var advancedFields = document.createElement('div');
    advancedFields.id = 'task-advanced-fields';
    advancedFields.style.display = 'none';

    var providerInput = document.createElement('input');
    providerInput.type = 'text';
    providerInput.id = 'task-provider';
    providerInput.setAttribute('aria-label', '提供方');

    var modelInput = document.createElement('input');
    modelInput.type = 'text';
    modelInput.id = 'task-model';
    modelInput.setAttribute('aria-label', '模型');

    var maxStepsInput = document.createElement('input');
    maxStepsInput.type = 'number';
    maxStepsInput.id = 'task-max-steps';
    maxStepsInput.setAttribute('aria-label', '最大步数');
    maxStepsInput.min = '1';
    maxStepsInput.step = '1';

    var verifyModeSelect = document.createElement('select');
    verifyModeSelect.id = 'task-verify-mode';
    verifyModeSelect.setAttribute('aria-label', '验证模式');
    ['standard', 'strict', 'off'].forEach(function (value) {
      var option = document.createElement('option');
      option.value = value;
      option.textContent = value;
      if (value === 'standard') {
        option.selected = true;
      }
      verifyModeSelect.appendChild(option);
    });

    advancedFields.appendChild(createAdvancedField('提供方', providerInput));
    advancedFields.appendChild(createAdvancedField('模型', modelInput));
    advancedFields.appendChild(createAdvancedField('最大步数', maxStepsInput));
    advancedFields.appendChild(createAdvancedField('验证模式', verifyModeSelect));

    toggle.addEventListener('click', function () {
      var isOpen = advancedFields.style.display !== 'none';
      setVisible(advancedFields, !isOpen, 'block');
      toggle.setAttribute('aria-expanded', isOpen ? 'false' : 'true');
      toggle.textContent = isOpen ? '高级参数' : '收起高级参数';
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
    setText(els.connectBtn, loading ? '连接中...' : '连接');
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
      setHTML(els.sessionList, '<p class="placeholder-copy">任务工作台已就绪，连接后将展示会话数据。</p>');
  }

  function normalizeStatus(status) {
    return String(status || 'unknown').toLowerCase();
  }

  function formatStatusLabel(status) {
    var normalized = normalizeStatus(status);
    if (!normalized) {
      return '未知';
    }
      var map = {
        created: '已创建',
        running: '运行中',
        completed: '已完成',
        failed: '失败',
        cancelled: '已取消',
      };
      return map[normalized] || normalized;
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
      '<span class="detail-label">令牌流</span>' +
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
      '<span class="detail-label">工具开始 · 步骤 ' + escapeHTML(event.step || '?') + '</span>' +
      '<div class="detail-value">' + escapeHTML(event.tool || 'unknown tool') + '</div>' +
      '<pre>' + escapeHTML(event.input || '') + '</pre>');
  }

  function renderToolEndEvent(event) {
    var outputSummary = event && event.error
      ? 'Error: ' + String(event.error)
      : (event && event.output ? String(event.output) : 'No output');
    appendStreamEvent('tool_end',
      '<span class="detail-label">工具结束 · 步骤 ' + escapeHTML(event.step || '?') + '</span>' +
      '<div class="detail-value">' + escapeHTML(event.tool || 'unknown tool') + '</div>' +
      '<pre>' + escapeHTML(outputSummary) + '</pre>');
  }

  function renderStepCompleteEvent(event) {
    appendStreamEvent('step_complete',
      '<span class="detail-label">步骤完成 · 步骤 ' + escapeHTML(event.step || '?') + '</span>' +
      '<div class="detail-value">' + escapeHTML(event.summary || 'Step completed.') + '</div>');
  }

  function renderErrorEvent(event) {
    appendStreamEvent('error',
      '<span class="detail-label">错误 · 步骤 ' + escapeHTML(event.step || '?') + '</span>' +
      '<div class="detail-value">' + escapeHTML(event.message || '未知流错误。') + '</div>');
  }

  function renderSessionCompleteEvent(event, sessionId) {
    var finalStatus = event && event.status ? String(event.status) : 'unknown';
    setStreamDisconnectedMessage('');
      setStreamStateMarkup('会话已完成：' + finalStatus, 'stream-complete');
    appendStreamEvent('session_complete',
      '<span class="detail-label">SESSION COMPLETE</span>' +
      '<div class="detail-value">最终状态：' + escapeHTML(finalStatus) + '</div>');
    if (els.viewDetailLink) {
      els.viewDetailLink.href = '#/detail/' + encodeURIComponent(pickSessionID(event, sessionId));
      setVisible(els.viewDetailLink, true, 'inline-block');
    }
  }

  function renderFallbackStreamEvent(event) {
    appendStreamEvent('unknown',
      '<span class="detail-label">流事件</span>' +
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
      { label: '全部', value: '' },
      { label: '已创建', value: 'created' },
      { label: '运行中', value: 'running' },
      { label: '已完成', value: 'completed' },
      { label: '失败', value: 'failed' },
      { label: '已取消', value: 'cancelled' },
    ];
    setHTML(els.sessionList, '<p class="placeholder-copy">正在加载会话...</p>');
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

      function sessionCard(session) {
        var sessionID = escapeHTML(pickSessionID(session, ''));
        var task = escapeHTML(truncateText(session.task || '未命名任务', 80));
        var status = renderStatusBadge(session.status || 'unknown');
        var type = escapeHTML(session.task_type || 'general');
        var timestamp = escapeHTML(formatTimestamp(session.updated_at || session.created_at));
        return '<div class="dashboard-session" data-test="dashboard-session-item">' +
          '<div class="detail-grid">' +
            buildDetailItem('状态', status) +
            buildDetailItem('任务类型', type) +
            buildDetailItem('更新时间', timestamp) +
          '</div>' +
          '<div class="detail-item">' +
            '<span class="detail-label">任务</span>' +
            '<div class="detail-value">' + task + '</div>' +
          '</div>' +
          '<div><a href="#/detail/' + sessionID + '">查看详情</a></div>' +
        '</div>';
      }

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
      var statsHTML = '<div class="dashboard-stats" data-test="dashboard-stats">' +
        '<div class="stat-chip"><span class="stat-label">已创建</span><span class="stat-value">' + escapeHTML(statusCounts.created) + '</span></div>' +
        '<div class="stat-chip"><span class="stat-label">运行中</span><span class="stat-value">' + escapeHTML(statusCounts.running) + '</span></div>' +
        '<div class="stat-chip"><span class="stat-label">已完成</span><span class="stat-value">' + escapeHTML(statusCounts.completed) + '</span></div>' +
        '<div class="stat-chip"><span class="stat-label">失败</span><span class="stat-value">' + escapeHTML(statusCounts.failed) + '</span></div>' +
        '<div class="stat-chip"><span class="stat-label">已取消</span><span class="stat-value">' + escapeHTML(statusCounts.cancelled) + '</span></div>' +
      '</div>';

      function toEpoch(value) {
        var timestamp = Date.parse(value || '');
        return Number.isFinite(timestamp) ? timestamp : 0;
      }

      function byUpdatedDesc(left, right) {
        var leftAt = toEpoch(left && (left.updated_at || left.created_at));
        var rightAt = toEpoch(right && (right.updated_at || right.created_at));
        return rightAt - leftAt;
      }

      var runningSessions = sessions.filter(function (session) {
        return normalizeStatus(session && session.status) === 'running';
      });
      var failedSessions = sessions.filter(function (session) {
        return normalizeStatus(session && session.status) === 'failed';
      });
      var focusSessions = failedSessions.concat(runningSessions).sort(byUpdatedDesc);
      var recentSessions = sessions.filter(function (session) {
        var status = normalizeStatus(session && session.status);
        return status !== 'running' && status !== 'failed';
      }).sort(byUpdatedDesc);

      if (!sessions.length) {
        setHTML(
          els.sessionList,
          '<div class="card" data-test="dashboard-empty">' +
            '<h3>暂无会话</h3>' +
            '<p class="placeholder-copy">当前没有可展示的会话，请先创建新任务。</p>' +
          '</div>'
        );
        return;
      }

      var operationsHTML = '<div class="card dashboard-ops" data-test="dashboard-ops">' +
        '<h3>运营焦点</h3>' +
        '<p class="page-subtitle">优先处理运行中与失败任务，确保执行链路稳定（统计为本页数据）。</p>' +
        '<div class="dashboard-ops-grid">' +
          '<div class="stat-chip stat-chip-emphasis" data-test="dashboard-ops-running"><span class="stat-label">运行中需关注</span><span class="stat-value">' + escapeHTML(runningSessions.length) + '</span></div>' +
          '<div class="stat-chip stat-chip-danger" data-test="dashboard-ops-failed"><span class="stat-label">失败待介入</span><span class="stat-value">' + escapeHTML(failedSessions.length) + '</span></div>' +
        '</div>' +
      '</div>';

      var focusSectionHTML = '<section class="card" data-test="dashboard-priority"><h3>优先会话</h3>' +
        (focusSessions.length
          ? focusSessions.slice(0, 6).map(sessionCard).join('')
          : '<p class="placeholder-copy">当前没有运行中或失败会话。</p>') +
      '</section>';

      var recentSectionHTML = '<section class="card" data-test="dashboard-recent"><h3>最近更新</h3>' +
        (recentSessions.length
          ? recentSessions.slice(0, 6).map(sessionCard).join('')
          : '<p class="placeholder-copy">暂无其他会话记录。</p>') +
      '</section>';

      var previousPage = dashboardState.page > 1 ? dashboardState.page - 1 : 1;
      var nextPage = dashboardState.page < totalPages ? dashboardState.page + 1 : totalPages;
      var paginationHTML = '<div class="card"><div style="display: flex; align-items: center; gap: 12px; flex-wrap: wrap;">' +
        '<button class="secondary-button" data-dashboard-page="' + escapeHTML(previousPage) + '"' + (dashboardState.page <= 1 ? ' disabled' : '') + '>&#8592; 上一页</button>' +
        '<span>第 ' + escapeHTML(dashboardState.page) + ' / ' + escapeHTML(totalPages) + ' 页</span>' +
        '<button class="secondary-button" data-dashboard-page="' + escapeHTML(nextPage) + '"' + (dashboardState.page >= totalPages ? ' disabled' : '') + '>下一页 &#8594;</button>' +
      '</div></div>';

      setHTML(
        els.sessionList,
        statsHTML + operationsHTML + filtersHTML + '<div class="dashboard-section-grid">' + focusSectionHTML + recentSectionHTML + '</div>' + paginationHTML
      );
      bindDashboardControls();
    } catch (error) {
      setHTML(els.sessionList, '<div class="card"><p class="placeholder-copy">会话加载失败：' + escapeHTML(error && error.message ? error.message : '未知错误') + '</p></div>');
    }
  }

  function renderStreamShell(sessionId) {
    setText(els.sessionId, sessionId);
    setStreamStateMarkup('正在连接会话流...', 'placeholder-copy');
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
          setStreamStateMarkup('正在实时推送会话输出...', 'stream-live');
        },
        onEvent: function (event) {
          handleStreamEvent(event, sessionId);
        },
        onError: function (error) {
          if (error && error.name === 'AbortError') {
            return;
          }
          var message = '流连接已断开，会话仍在后台运行，请到详情页查看最新状态。';
          var detail = getErrorMessage(error, '');
          if (detail) {
            message += ' 错误：' + detail;
          }
          setStreamDisconnectedMessage(message);
        },
      });
    } catch (error) {
      setStreamDisconnectedMessage('流连接已断开，会话仍在后台运行。错误：' + getErrorMessage(error, '无法建立流连接。'));
    }
  }

  async function loadDetail(sessionId) {
    setVisible(els.resumeBtn, false);
    setResumeButtonState(false);
    setHTML(els.sessionDetail, '<p class="placeholder-copy">正在加载会话详情...</p>');
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
        buildDetailItem('会话 ID', escapeHTML(pickSessionID(payload, sessionId))),
        buildDetailItem('状态', renderStatusBadge(payload.status || 'unknown')),
        buildDetailItem('任务类型', escapeHTML(payload.task_type || 'general')),
        buildDetailItem('创建时间', escapeHTML(formatTimestamp(payload.created_at))),
        buildDetailItem('更新时间', escapeHTML(formatTimestamp(payload.updated_at))),
        '</div>',
        '<div class="detail-item"><span class="detail-label">任务描述</span><div class="detail-value">' + escapeHTML(payload.task || '') + '</div></div>',
        '<div class="detail-item"><span class="detail-label">计划摘要</span><div class="detail-value"><pre>' + escapeHTML(summarizePlan(payload.plan)) + '</pre></div></div>',
        '</div>',
        '<div class="card">',
        '<div class="detail-item"><span class="detail-label">执行步骤</span><div class="detail-value">',
        steps.length ? steps.map(function (step) {
          return '<div class="detail-item">' +
            '<div class="detail-grid">' +
              buildDetailItem('步骤序号', escapeHTML(step && step.step_number != null ? step.step_number : '—')) +
              buildDetailItem('工具名称', escapeHTML((step && step.tool_name) || '—')) +
              buildDetailItem('校验状态', renderStatusBadge((step && step.status) || 'unknown')) +
            '</div>' +
            '<div class="detail-item"><span class="detail-label">观察摘要</span><div class="detail-value">' + escapeHTML(summarizeObservation(step && step.observation)) + '</div></div>' +
          '</div>';
        }).join('') : '<p class="placeholder-copy">暂无步骤记录</p>',
        '</div></div>',
        '</div>',
      ];

      if (hasProvenance) {
        html.push(
          '<div class="card">' +
            '<div class="detail-item">' +
              '<span class="detail-label">溯源信息</span>' +
              '<div class="detail-value"><pre>' + escapeHTML(stringifyValue(payload.provenance)) + '</pre></div>' +
            '</div>' +
          '</div>'
        );
      }

      if (payload && payload.termination_reason) {
        html.push(
          '<div class="card">' +
            '<div class="detail-item">' +
              '<span class="detail-label">终止原因</span>' +
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
      setHTML(els.sessionDetail, '<p class="placeholder-copy">加载会话详情失败：' + escapeHTML(error.message || '未知错误') + '</p>');
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
    var verification = { valid: false, error: '令牌校验失败' };
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
        showAuthError((verification && verification.error) || '令牌校验失败');
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
      showAuthError('请输入 JWT 令牌。');
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
      showTaskValidationError('任务描述不能为空。');
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
        throw new Error('任务已启动，但未返回会话 ID。');
      }
      if (els.taskForm) {
        els.taskForm.reset();
      }
      clearTaskValidationError();
      window.location.hash = '#/stream/' + encodeURIComponent(sessionId);
    } catch (error) {
      var status = getErrorStatus(error);
      var message = getErrorMessage(error, '任务提交失败。');
      if (status === 400 || status === 409 || status === 429 || status === 500) {
        showTaskValidationError(message);
      } else {
        showTaskValidationError('任务提交失败：' + message);
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
        showDetailActionMessage('会话不存在');
      } else if (status === 409) {
        showDetailActionMessage('会话正在运行');
      } else if (status === 429) {
        showDetailActionMessage('当前活跃会话过多，请稍后重试');
      } else {
        showDetailActionMessage('继续执行失败：' + getErrorMessage(error, '未知错误'));
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
      showAuthError('认证已过期或被拒绝，请重新输入 JWT 令牌。');
      window.location.hash = '#/';
      renderRoute();
    });
    window.addEventListener('load', renderRoute);
  }

  async function bootstrapStoredToken() {
    if (isBootstrappingAuth || typeof auth.getToken !== 'function') {
      return;
    }
    
    // 先尝试从 localStorage 恢复已有 token
    var storedToken = auth.getToken();
    if (storedToken) {
      if (els.jwtInput) {
        els.jwtInput.value = storedToken;
      }
      isBootstrappingAuth = true;
      if (typeof auth.clearToken === 'function') {
        auth.clearToken();
      }
      var connected = await connectWithToken(storedToken, { keepInput: true });
      if (connected) {
        isBootstrappingAuth = false;
        return;
      }
      if (els.jwtInput) {
        els.jwtInput.value = storedToken;
      }
      isBootstrappingAuth = false;
      resetAuthUI({ clearInput: true });
      renderAuthState();
      return;
    }
    
    // 没有已有 token，尝试从 /healthz 自动获取
    setVisibleAuthManual(false);
    try {
      var resp = await fetch('/healthz');
      if (resp.ok) {
        var data = await resp.json();
        if (data && data.dev_token) {
          var autoConnected = await connectWithToken(data.dev_token, {});
          if (autoConnected) {
            isBootstrappingAuth = false;
            return;
          }
        }
      }
    } catch (e) {
      // ignore - can't reach server or no dev token
    }
    
    // 自动获取失败，显示手动输入界面
    setVisibleAuthManual(true);
    resetAuthUI({ clearInput: true });
    renderAuthState();
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
