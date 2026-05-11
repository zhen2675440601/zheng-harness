(function () {
  var auth = window.ZhengAuth || {};
  var api = window.ZhengAPI || {};

  var activeStream = null;
  var isAuthBusy = false;
  var isBootstrappingAuth = false;
  var isTaskSubmitting = false;
  var isResumeSubmitting = false;

  var historyState = {
    page: 1,
    pageSize: 20,
    status: '',
  };

  var state = {
    currentConversationId: '',
    currentSessionId: '',
    currentStatus: 'ready',
    transcript: [],
    streamBuffer: '',
    streamEvents: [],
    streamingTurnIndex: null,
    historyOpen: false,
    inspectOpen: false,
  };

  function byId(id) {
    return document.getElementById(id);
  }

  var els = {
    authScreen: byId('auth-screen'),
    authError: byId('auth-error'),
    authStatusConnected: byId('auth-status-connected'),
    mainContent: byId('main-content'),
    jwtInput: byId('jwt-input'),
    connectBtn: byId('connect-btn'),
    disconnectBtn: byId('disconnect-btn'),
    historyToggle: byId('history-toggle'),
    historyClose: byId('history-close'),
    historyRefresh: byId('history-refresh'),
    historyStatusFilter: byId('history-status-filter'),
    historyPanel: byId('history-panel'),
    historyError: byId('history-error'),
    conversationList: byId('conversation-list'),
    composerForm: byId('composer-form'),
    composerInput: byId('composer-input'),
    composerTaskType: byId('composer-task-type'),
    sendMessageBtn: byId('send-message-btn'),
    composerValidationError: byId('composer-validation-error'),
    composerError: byId('composer-error'),
    resumeBtn: byId('resume-btn'),
    inspectToggle: byId('inspect-toggle'),
    inspectClose: byId('inspect-close'),
    inspectPanel: byId('inspect-panel'),
    inspectDetail: byId('inspect-detail'),
    conversationStream: byId('conversation-stream'),
    streamDisconnected: byId('stream-disconnected'),
    connectionIndicator: byId('connection-indicator'),
    connectionLabel: byId('connection-label'),
    streamStatusText: byId('stream-status-text'),
    conversationId: byId('conversation-id'),
    conversationStatus: byId('conversation-status'),
    conversationTitle: byId('conversation-title'),
    conversationSubtitle: byId('conversation-subtitle'),
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

  function normalizeHash(hash) {
    var value = String(hash || '').trim();
    if (!value || value === '#') {
      return '#/chat';
    }
    if (value === '#/' || value === '#') {
      return '#/chat';
    }
    return value;
  }

  function parseHash(hash) {
    var normalized = normalizeHash(hash).replace(/^#/, '');
    var path = normalized || '/chat';
    var parts = path.split('/').filter(Boolean);
    var info = {
      route: 'chat',
      conversationId: '',
      inspect: false,
      history: false,
    };

    if (parts[0] === 'history') {
      info.history = true;
      return info;
    }
    if (parts[0] === 'chat' || parts.length === 0) {
      info.conversationId = parts[1] ? decodeURIComponent(parts[1]) : '';
      info.inspect = parts[2] === 'inspect';
      return info;
    }
    return info;
  }

  function updateHash(options) {
    var next = '#/chat';
    if (options && options.conversationId) {
      next += '/' + encodeURIComponent(options.conversationId);
    }
    if (options && options.inspect) {
      next += '/inspect';
    }
    if (window.location.hash !== next) {
      window.location.hash = next;
      return;
    }
    syncRouteToState();
  }

  function clearAuthError() {
    setText(els.authError, '');
    setVisible(els.authError, false);
  }

  function showAuthError(message) {
    setText(els.authError, message);
    setVisible(els.authError, true, 'block');
  }

  function clearComposerValidationError() {
    setText(els.composerValidationError, '');
    setVisible(els.composerValidationError, false);
  }

  function showComposerValidationError(message) {
    setText(els.composerValidationError, message);
    setVisible(els.composerValidationError, true, 'block');
  }

  function clearComposerError() {
    setText(els.composerError, '');
    setVisible(els.composerError, false);
  }

  function showComposerError(message) {
    setText(els.composerError, message);
    setVisible(els.composerError, true, 'block');
  }

  function clearHistoryError() {
    setText(els.historyError, '');
    setVisible(els.historyError, false);
  }

  function showHistoryError(message) {
    setText(els.historyError, message);
    setVisible(els.historyError, true, 'block');
  }

  function setDisconnectBanner(message) {
    if (!message) {
      setText(els.streamDisconnected, '');
      setVisible(els.streamDisconnected, false);
      return;
    }
    setText(els.streamDisconnected, message);
    setVisible(els.streamDisconnected, true, 'block');
  }

  function setConnectionState(kind, label) {
    if (els.connectionIndicator) {
      els.connectionIndicator.className = 'status-dot ' + (kind === 'danger' ? 'red' : kind === 'warning' ? 'amber' : 'green');
    }
    setText(els.connectionLabel, label);
  }

  function setStreamStatus(text) {
    setText(els.streamStatusText, text);
  }

  function formatDate(value) {
    if (!value) {
      return '—';
    }
    var date = new Date(value);
    if (isNaN(date.getTime())) {
      return String(value);
    }
    return date.toLocaleString('zh-CN', {
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
    });
  }

  function formatFullDate(value) {
    if (!value) {
      return '';
    }
    var date = new Date(value);
    if (isNaN(date.getTime())) {
      return String(value);
    }
    return date.toLocaleString('zh-CN');
  }

  function statusClass(status) {
    var normalized = String(status || 'created').toLowerCase();
    if (normalized === 'success') {
      normalized = 'completed';
    }
    if (normalized === 'blocked' || normalized === 'blocked_input') {
      normalized = 'running';
    }
    if (normalized === 'pending') {
      normalized = 'created';
    }
    return 'status-' + normalized.replace(/[^a-z0-9_-]/g, '-');
  }

  function statusLabel(status) {
    var normalized = String(status || 'created').toLowerCase();
    var mapping = {
      ready: 'ready',
      created: '已创建',
      pending: '已创建',
      active: '进行中',
      running: '运行中',
      blocked: '待继续',
      blocked_input: '待继续',
      completed: '已完成',
      success: '已完成',
      failed: '失败',
      cancelled: '已取消',
      interrupted: '已中断',
      archived: '已归档'
    };
    return mapping[normalized] || normalized;
  }

  function isResumableStatus(status) {
    var normalized = String(status || '').toLowerCase();
    return normalized === 'running' || normalized === 'blocked' || normalized === 'blocked_input' || normalized === 'active';
  }

  function getErrorMessage(error, fallbackMessage) {
    var message = error && error.message ? String(error.message).trim() : '';
    return message || fallbackMessage;
  }

  function deriveSessionIdFromStreamURL(streamURL) {
    var value = String(streamURL || '');
    var match = value.match(/\/sessions\/([^/]+)\/stream/i);
    return match ? decodeURIComponent(match[1]) : '';
  }

  function closeActiveStream() {
    if (activeStream && typeof activeStream.close === 'function') {
      activeStream.close();
    }
    activeStream = null;
  }

  function resetWorkspace() {
    closeActiveStream();
    state.currentConversationId = '';
    state.currentSessionId = '';
    state.currentStatus = 'ready';
    state.transcript = [];
    state.streamBuffer = '';
    state.streamEvents = [];
    state.streamingTurnIndex = null;
    renderTranscript();
    renderHeader();
    renderInspectPlaceholder();
    setDisconnectBanner('');
    clearComposerError();
  }

  function renderHeader() {
    var conversationId = state.currentConversationId || '未创建';
    var title = state.currentConversationId ? '对话 ' + state.currentConversationId : '新对话';
    var subtitle = '发送第一条消息以创建会话。';
    if (state.currentConversationId && state.currentSessionId) {
      subtitle = '当前会话 ' + state.currentSessionId + ' · 最近流式输出将在当前转录中内联展示。';
    } else if (state.currentConversationId) {
      subtitle = '历史转录已载入，可继续追加消息。';
    }

    setText(els.conversationId, conversationId);
    setText(els.conversationTitle, title);
    setText(els.conversationSubtitle, subtitle);
    if (els.conversationStatus) {
      els.conversationStatus.className = 'status-badge ' + statusClass(state.currentStatus);
      els.conversationStatus.textContent = statusLabel(state.currentStatus);
    }
    setVisible(els.resumeBtn, !!state.currentSessionId && isResumableStatus(state.currentStatus), 'inline-flex');
  }

  function renderInspectPlaceholder() {
    if (!els.inspectDetail) {
      return;
    }
    if (!state.currentSessionId) {
      setHTML(els.inspectDetail, '<p class="placeholder-copy">选择或创建对话后，可在此查看计划、步骤、工具调用与验证结果。</p>');
    }
  }

  function buildStreamingEntry() {
    if (state.streamingTurnIndex == null) {
      return '';
    }
    var eventHTML = state.streamEvents.length
      ? '<div class="stream-event-list">' + state.streamEvents.map(function (item) {
        return '<div class="stream-event-item">' + escapeHTML(item) + '</div>';
      }).join('') + '</div>'
      : '';
    var content = state.streamBuffer || '等待智能体返回内容…';
    return [
      '<article class="chat-message assistant streaming" data-turn-index="', escapeHTML(state.streamingTurnIndex), '">',
      '<div class="message-meta"><span>assistant · 流式中</span></div>',
      '<div class="message-bubble">',
      '<div class="message-content">', escapeHTML(content).replace(/\n/g, '<br>'), '</div>',
      eventHTML,
      '</div>',
      '</article>'
    ].join('');
  }

  function renderTranscript() {
    if (!els.conversationStream) {
      return;
    }
    if (!state.transcript.length && state.streamingTurnIndex == null) {
      setHTML(els.conversationStream, '<div class="transcript-empty"><p>这里会显示完整对话记录、工具进度和状态变化。</p><p>先发送一条消息，开始新的智能体协作。</p></div>');
      return;
    }

    var html = state.transcript.map(function (message) {
      var role = String(message.role || 'system').toLowerCase();
      var roleLabel = role === 'user' ? 'you' : role;
      return [
        '<article class="chat-message ', escapeHTML(role), '" data-turn-index="', escapeHTML(message.turn_index), '">',
        '<div class="message-meta"><span>', escapeHTML(roleLabel), '</span><span>', escapeHTML(formatDate(message.timestamp)), '</span></div>',
        '<div class="message-bubble">',
        '<div class="message-content">', escapeHTML(message.content || '').replace(/\n/g, '<br>'), '</div>',
        '</div>',
        '</article>'
      ].join('');
    }).join('');

    html += buildStreamingEntry();
    setHTML(els.conversationStream, html);
    els.conversationStream.scrollTop = els.conversationStream.scrollHeight;
  }

  function renderHistoryList(items) {
    if (!els.conversationList) {
      return;
    }
    if (!items.length) {
      setHTML(els.conversationList, '<div class="history-empty">暂无会话记录。</div>');
      return;
    }

    setHTML(els.conversationList, items.map(function (item) {
      var conversationId = item.conversation_id || item.session_id || '';
      var active = conversationId && conversationId === state.currentConversationId;
      return [
        '<button class="history-item', active ? ' active' : '', '" type="button" data-action="open-chat" data-conversation-id="', escapeHTML(conversationId), '" data-session-id="', escapeHTML(item.session_id || ''), '">',
        '<div class="history-item-top">',
        '<span class="history-title">', escapeHTML(item.task || '未命名对话'), '</span>',
        '<span class="status-badge ', statusClass(item.status), '">', escapeHTML(statusLabel(item.status)), '</span>',
        '</div>',
        '<div class="history-item-meta">',
        '<span>', escapeHTML(item.task_type || 'general'), '</span>',
        '<span>', escapeHTML(formatDate(item.updated_at || item.created_at)), '</span>',
        '</div>',
        '<div class="history-item-id">', escapeHTML(conversationId || item.session_id || ''), '</div>',
        '</button>'
      ].join('');
    }).join(''));
  }

  function renderInspect(data) {
    if (!els.inspectDetail) {
      return;
    }
    if (!data) {
      renderInspectPlaceholder();
      return;
    }

    var steps = Array.isArray(data.steps) ? data.steps : [];
    var stepHTML = steps.length ? steps.map(function (step) {
      return [
        '<div class="detail-item stream-event ', step.status === 'failed' ? 'stream-event-error' : '', '">',
        '<div class="detail-item-head"><strong>Step ', escapeHTML(step.step_number), '</strong><span class="status-badge ', statusClass(step.status || 'running'), '">', escapeHTML(statusLabel(step.status || 'running')), '</span></div>',
        step.tool_name ? '<div class="detail-inline"><span class="detail-label-inline">工具</span><code>' + escapeHTML(step.tool_name) + '</code></div>' : '',
        step.observation ? '<p class="detail-observation">' + escapeHTML(step.observation) + '</p>' : '',
        step.tool_input ? '<pre class="detail-json">' + escapeHTML(JSON.stringify(step.tool_input, null, 2)) + '</pre>' : '',
        '</div>'
      ].join('');
    }).join('') : '<p class="placeholder-copy">暂无步骤详情。</p>';

    setHTML(els.inspectDetail, [
      '<div class="detail-grid">',
      '<div class="detail-item"><span class="detail-label">Session</span><span class="detail-value">', escapeHTML(data.session_id || state.currentSessionId || ''), '</span></div>',
      '<div class="detail-item"><span class="detail-label">状态</span><span class="detail-value">', escapeHTML(statusLabel(data.status)), '</span></div>',
      '<div class="detail-item"><span class="detail-label">创建时间</span><span class="detail-value">', escapeHTML(formatFullDate(data.created_at)), '</span></div>',
      '<div class="detail-item"><span class="detail-label">更新时间</span><span class="detail-value">', escapeHTML(formatFullDate(data.updated_at)), '</span></div>',
      '</div>',
      '<div class="detail-block">',
      '<h4>任务</h4>',
      '<p>', escapeHTML(data.task || ''), '</p>',
      data.plan ? '<pre class="detail-json">' + escapeHTML(data.plan) + '</pre>' : '',
      '</div>',
      '<div class="detail-block">',
      '<h4>步骤</h4>',
      stepHTML,
      '</div>'
    ].join(''));
  }

  function applyHistoryVisibility() {
    if (!els.historyPanel || !els.mainContent) {
      return;
    }
    els.historyPanel.classList.toggle('panel-open', !!state.historyOpen);
    els.mainContent.classList.toggle('history-open', !!state.historyOpen);
  }

  function applyInspectVisibility() {
    setVisible(els.inspectPanel, !!state.inspectOpen, 'block');
  }

  async function loadHistory() {
    clearHistoryError();
    try {
      var payload = await api.apiListChats({
        page: historyState.page,
        pageSize: historyState.pageSize,
        status: historyState.status,
      });
      renderHistoryList((payload && payload.conversations) || []);
    } catch (error) {
      showHistoryError(getErrorMessage(error, '加载历史会话失败'));
    }
  }

  async function loadInspect(sessionId) {
    if (!sessionId) {
      renderInspectPlaceholder();
      return;
    }
    setHTML(els.inspectDetail, '<p class="placeholder-copy">正在加载 Inspect…</p>');
    try {
      var payload = await api.apiInspect(sessionId);
      renderInspect(payload);
    } catch (error) {
      setHTML(els.inspectDetail, '<div class="inline-banner inline-banner-danger">' + escapeHTML(getErrorMessage(error, '加载 Inspect 失败')) + '</div>');
    }
  }

  function dedupeMessages(messages) {
    var seen = {};
    return (messages || []).filter(function (message) {
      var key = [message.turn_index, message.role, message.timestamp, message.content].join('::');
      if (seen[key]) {
        return false;
      }
      seen[key] = true;
      return true;
    });
  }

  async function loadTranscript(conversationId, sessionIdHint) {
    if (!conversationId) {
      resetWorkspace();
      return;
    }
    closeActiveStream();
    setConnectionState('green', '已连接');
    setStreamStatus('载入中');
    setDisconnectBanner('');
    clearComposerError();

    try {
      var payload = await api.apiGetTranscript(conversationId);
      state.currentConversationId = payload.conversation_id || conversationId;
      state.currentSessionId = sessionIdHint || state.currentSessionId;
      state.currentStatus = payload.status || 'active';
      state.transcript = dedupeMessages(payload.messages || []);
      state.streamBuffer = '';
      state.streamEvents = [];
      state.streamingTurnIndex = null;
      renderHeader();
      renderTranscript();
      if (state.inspectOpen) {
        loadInspect(state.currentSessionId);
      } else {
        renderInspectPlaceholder();
      }
      setStreamStatus('空闲');
    } catch (error) {
      showComposerError(getErrorMessage(error, '加载对话失败'));
      setStreamStatus('异常');
      setConnectionState('danger', '加载失败');
    }
  }

  function appendLocalUserMessage(messageText) {
    var lastTurnIndex = state.transcript.length ? Number(state.transcript[state.transcript.length - 1].turn_index || 0) : -1;
    var nextTurnIndex = Math.max(lastTurnIndex + 1, 0);
    state.transcript.push({
      turn_index: nextTurnIndex,
      role: 'user',
      content: messageText,
      timestamp: new Date().toISOString(),
    });
    renderTranscript();
    return nextTurnIndex;
  }

  function pushStreamEventSummary(type, payload) {
    if (type === 'tool_start' && payload && payload.tool_name) {
      state.streamEvents.push('工具启动：' + payload.tool_name);
      return;
    }
    if (type === 'tool_end' && payload && payload.tool_name) {
      state.streamEvents.push('工具完成：' + payload.tool_name);
      return;
    }
    if (type === 'step_complete' && payload && payload.step_summary) {
      state.streamEvents.push('步骤完成：' + payload.step_summary);
      return;
    }
    if (type === 'error' && payload && payload.message) {
      state.streamEvents.push('错误：' + payload.message);
    }
  }

  function parsePayload(raw) {
    if (!raw || typeof raw !== 'object') {
      return {};
    }
    if (raw.payload && typeof raw.payload === 'object') {
      return raw.payload;
    }
    return raw;
  }

  async function finalizeStream() {
    if (!state.currentConversationId) {
      return;
    }
    await loadTranscript(state.currentConversationId, state.currentSessionId);
  }

  function openStream(sessionId, turnIndex) {
    closeActiveStream();
    state.currentSessionId = sessionId || state.currentSessionId;
    state.streamingTurnIndex = turnIndex;
    state.streamBuffer = '';
    state.streamEvents = [];
    setDisconnectBanner('');
    setConnectionState('green', '流式连接中');
    setStreamStatus('流式中');
    renderHeader();
    renderTranscript();

    activeStream = api.apiStream(sessionId, {
      onOpen: function () {
        setConnectionState('green', '流式已连接');
        setStreamStatus('接收中');
      },
      onEvent: function (event) {
        var payload = parsePayload(event.data);
        if (event.type === 'token_delta') {
          state.streamBuffer += payload.content || '';
        } else if (event.type === 'session_complete') {
          state.currentStatus = payload.status || 'completed';
        } else if (event.type === 'error') {
          state.currentStatus = 'failed';
        } else if (!state.currentStatus || state.currentStatus === 'ready') {
          state.currentStatus = 'running';
        }
        pushStreamEventSummary(event.type, payload);
        renderHeader();
        renderTranscript();
      },
      onComplete: function () {
        setConnectionState('green', '已连接');
        setStreamStatus('已完成');
        finalizeStream();
      },
      onError: function (error) {
        setConnectionState('warning', '流式中断');
        setStreamStatus('已中断');
        setDisconnectBanner(getErrorMessage(error, '流式连接中断，部分内容可能仍在服务端继续执行。请刷新历史或重新载入对话。'));
      },
    });
  }

  function setTaskSubmitButtonState(loading) {
    isTaskSubmitting = !!loading;
    if (!els.sendMessageBtn) {
      return;
    }
    els.sendMessageBtn.disabled = !!loading;
    setText(els.sendMessageBtn, loading ? '发送中…' : '发送消息');
  }

  function setResumeButtonState(loading) {
    isResumeSubmitting = !!loading;
    if (!els.resumeBtn) {
      return;
    }
    els.resumeBtn.disabled = !!loading;
    setText(els.resumeBtn, loading ? '继续中…' : '继续执行');
  }

  async function submitComposer() {
    if (isTaskSubmitting) {
      return;
    }
    clearComposerValidationError();
    clearComposerError();

    var message = els.composerInput && typeof els.composerInput.value === 'string' ? els.composerInput.value.trim() : '';
    if (!message) {
      showComposerValidationError('请输入消息后再发送。');
      return;
    }

    setTaskSubmitButtonState(true);
    state.currentStatus = state.currentConversationId ? 'running' : 'created';
    var turnIndex = appendLocalUserMessage(message);
    renderHeader();

    try {
      var payload;
      if (!state.currentConversationId) {
        payload = await api.apiStartChat({
          message: message,
          task_type: els.composerTaskType ? els.composerTaskType.value : 'general',
        });
        state.currentConversationId = payload.conversation_id || '';
        state.currentSessionId = payload.session_id || deriveSessionIdFromStreamURL(payload.stream_url);
        state.currentStatus = payload.status || 'running';
      } else {
        payload = await api.apiReplyChat(state.currentConversationId, message);
        state.currentSessionId = deriveSessionIdFromStreamURL(payload.stream_url) || state.currentSessionId;
        state.currentStatus = 'running';
      }

      els.composerInput.value = '';
      updateHash({ conversationId: state.currentConversationId, inspect: state.inspectOpen });
      renderHeader();
      loadHistory();
      openStream(state.currentSessionId, turnIndex);
    } catch (error) {
      showComposerError(getErrorMessage(error, '发送消息失败'));
      state.currentStatus = 'failed';
      renderHeader();
    } finally {
      setTaskSubmitButtonState(false);
    }
  }

  async function resumeConversation() {
    if (!state.currentSessionId || isResumeSubmitting) {
      return;
    }
    clearComposerError();
    setResumeButtonState(true);
    try {
      var payload = await api.apiResume(state.currentSessionId);
      state.currentSessionId = payload.session_id || state.currentSessionId;
      state.currentStatus = payload.status || 'running';
      renderHeader();
      openStream(state.currentSessionId, state.transcript.length ? Number(state.transcript[state.transcript.length - 1].turn_index || 0) : 0);
      loadHistory();
    } catch (error) {
      showComposerError(getErrorMessage(error, '继续执行失败'));
    } finally {
      setResumeButtonState(false);
    }
  }

  async function openConversation(conversationId, sessionId) {
    state.currentConversationId = conversationId || '';
    state.currentSessionId = sessionId || '';
    renderHeader();
    await loadTranscript(conversationId, sessionId);
  }

  function syncRouteToState() {
    var route = parseHash(window.location.hash);
    state.historyOpen = route.history;
    state.inspectOpen = route.inspect;
    applyHistoryVisibility();
    applyInspectVisibility();
    if (state.inspectOpen) {
      loadInspect(state.currentSessionId);
    }
    if (route.conversationId && route.conversationId !== state.currentConversationId) {
      openConversation(route.conversationId, '');
      return;
    }
    if (!route.conversationId && state.currentConversationId && !route.inspect) {
      updateHash({ conversationId: state.currentConversationId, inspect: false });
    }
  }

  function showAuthenticatedShell() {
    setVisible(els.authScreen, false);
    setVisible(els.mainContent, true, 'block');
    setVisible(els.authStatusConnected, true, 'inline-flex');
    setConnectionState('green', '已连接');
    setStreamStatus('空闲');
    applyHistoryVisibility();
    applyInspectVisibility();
  }

  function showAuthScreen() {
    setVisible(els.mainContent, false);
    setVisible(els.authScreen, true, 'flex');
    setVisible(els.authStatusConnected, false);
    resetWorkspace();
  }

  async function connectWithToken(token) {
    if (isAuthBusy) {
      return;
    }
    isAuthBusy = true;
    clearAuthError();
    try {
      var result = await auth.verifyToken(token);
      if (!result || !result.valid) {
        throw new Error((result && result.error) || 'Token verification failed');
      }
      auth.setToken(token);
      showAuthenticatedShell();
      if (!window.location.hash || window.location.hash === '#/' || window.location.hash === '#') {
        window.location.hash = '#/chat';
      }
      syncRouteToState();
      loadHistory();
    } catch (error) {
      auth.clearToken();
      showAuthScreen();
      showAuthError(getErrorMessage(error, '连接失败'));
    } finally {
      isAuthBusy = false;
    }
  }

  async function bootstrapAuth() {
    if (isBootstrappingAuth) {
      return;
    }
    isBootstrappingAuth = true;
    var token = typeof auth.getToken === 'function' ? auth.getToken() : '';
    if (!token) {
      showAuthScreen();
      isBootstrappingAuth = false;
      return;
    }
    try {
      var result = await auth.verifyToken(token);
      if (result && result.valid) {
        showAuthenticatedShell();
        if (!window.location.hash || window.location.hash === '#/' || window.location.hash === '#') {
          window.location.hash = '#/chat';
        }
        syncRouteToState();
        loadHistory();
      } else {
        auth.clearToken();
        showAuthScreen();
      }
    } finally {
      isBootstrappingAuth = false;
    }
  }

  function bindEvents() {
    if (els.connectBtn) {
      els.connectBtn.addEventListener('click', function () {
        connectWithToken(els.jwtInput ? els.jwtInput.value : '');
      });
    }

    if (els.jwtInput) {
      els.jwtInput.addEventListener('keydown', function (event) {
        if (event.key === 'Enter') {
          event.preventDefault();
          connectWithToken(els.jwtInput.value);
        }
      });
    }

    if (els.disconnectBtn) {
      els.disconnectBtn.addEventListener('click', function () {
        auth.clearToken();
        showAuthScreen();
        window.location.hash = '#/chat';
      });
    }

    if (els.composerForm) {
      els.composerForm.addEventListener('submit', function (event) {
        event.preventDefault();
        submitComposer();
      });
    }

    if (els.historyToggle) {
      els.historyToggle.addEventListener('click', function () {
        state.historyOpen = !state.historyOpen;
        applyHistoryVisibility();
      });
    }

    if (els.historyClose) {
      els.historyClose.addEventListener('click', function () {
        state.historyOpen = false;
        applyHistoryVisibility();
      });
    }

    if (els.historyRefresh) {
      els.historyRefresh.addEventListener('click', function () {
        loadHistory();
      });
    }

    if (els.historyStatusFilter) {
      els.historyStatusFilter.addEventListener('change', function () {
        historyState.status = els.historyStatusFilter.value;
        loadHistory();
      });
    }

    if (els.conversationList) {
      els.conversationList.addEventListener('click', function (event) {
        var target = event.target && event.target.closest ? event.target.closest('[data-action="open-chat"]') : null;
        if (!target) {
          return;
        }
        updateHash({ conversationId: target.getAttribute('data-conversation-id') || '', inspect: false });
        openConversation(target.getAttribute('data-conversation-id') || '', target.getAttribute('data-session-id') || '');
      });
    }

    if (els.inspectToggle) {
      els.inspectToggle.addEventListener('click', function () {
        state.inspectOpen = !state.inspectOpen;
        applyInspectVisibility();
        updateHash({ conversationId: state.currentConversationId, inspect: state.inspectOpen });
        if (state.inspectOpen) {
          loadInspect(state.currentSessionId);
        }
      });
    }

    if (els.inspectClose) {
      els.inspectClose.addEventListener('click', function () {
        state.inspectOpen = false;
        applyInspectVisibility();
        updateHash({ conversationId: state.currentConversationId, inspect: false });
      });
    }

    if (els.resumeBtn) {
      els.resumeBtn.addEventListener('click', function () {
        resumeConversation();
      });
    }

    window.addEventListener('hashchange', syncRouteToState);
    window.addEventListener('zh:auth-required', function () {
      showAuthScreen();
      showAuthError('登录状态已失效，请重新输入 JWT。');
    });
  }

  bindEvents();
  renderHeader();
  renderInspectPlaceholder();
  renderTranscript();
  bootstrapAuth();
})();
