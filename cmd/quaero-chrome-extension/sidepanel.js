// Sidepanel script for Quaero extension

const DEFAULT_SERVER_URL = 'http://localhost:8085';
const MAX_LIST_ITEMS = 20;
let ws;
let reconnectInterval;

// Initialize
document.addEventListener('DOMContentLoaded', async () => {
  await loadSettings();
  connectWebSocket();

  // Load initial state
  await loadRecordingState();
  await loadFailedUploads();

  // Set up event listeners
  document.getElementById('recording-toggle').addEventListener('change', toggleRecording);
  document.getElementById('stop-recording-btn').addEventListener('click', stopRecordingFromBanner);
  document.getElementById('save-settings-btn').addEventListener('click', saveSettings);
  document.getElementById('clear-failed-btn').addEventListener('click', clearAllFailed);
});

// ============================================================================
// Settings
// ============================================================================

async function loadSettings() {
  const result = await chrome.storage.sync.get(['serverUrl']);
  const serverUrl = result.serverUrl || DEFAULT_SERVER_URL;
  document.getElementById('server-url').value = serverUrl;
}

async function saveSettings() {
  const serverUrl = document.getElementById('server-url').value;
  await chrome.storage.sync.set({ serverUrl });
  showSuccess('Settings saved');

  // Reconnect WebSocket with new URL
  if (ws) {
    ws.close();
  }
  connectWebSocket();
}

// ============================================================================
// WebSocket & Server Status
// ============================================================================

function connectWebSocket() {
  const serverUrl = document.getElementById('server-url').value;

  try {
    const url = new URL(serverUrl);
    const wsProtocol = url.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${wsProtocol}//${url.host}/ws`;

    ws = new WebSocket(wsUrl);

    ws.onopen = function() {
      console.log('WebSocket connected');
      updateServerStatus(true);
      if (reconnectInterval) {
        clearInterval(reconnectInterval);
        reconnectInterval = null;
      }
    };

    ws.onmessage = function(event) {
      const message = JSON.parse(event.data);
      if (message.type === 'status') {
        console.log('Status update:', message.payload);
      }
    };

    ws.onerror = function(error) {
      console.error('WebSocket error:', error);
      updateServerStatus(false);
    };

    ws.onclose = function() {
      console.log('WebSocket disconnected');
      updateServerStatus(false);

      // Reconnect after 3 seconds
      if (!reconnectInterval) {
        reconnectInterval = setInterval(function() {
          connectWebSocket();
        }, 3000);
      }
    };
  } catch (error) {
    console.error('Invalid server URL:', error);
    updateServerStatus(false);
  }
}

function updateServerStatus(online) {
  const dot = document.getElementById('server-status-dot');
  const text = document.getElementById('server-status-text');

  if (online) {
    dot.className = 'server-status-dot online';
    text.textContent = 'Connected';
  } else {
    dot.className = 'server-status-dot offline';
    text.textContent = 'Offline';
  }
}

// ============================================================================
// Recording Management
// ============================================================================

async function loadRecordingState() {
  try {
    const response = await chrome.runtime.sendMessage({ action: 'getRecordingState' });

    if (response.success) {
      updateRecordingUI(response.data);

      if (response.data.capturedUrls) {
        updateCapturedList(response.data.capturedUrls);
      }
    }
  } catch (error) {
    console.error('Error loading recording state:', error);
  }
}

async function toggleRecording() {
  const toggle = document.getElementById('recording-toggle');
  const isRecording = toggle.checked;

  try {
    const action = isRecording ? 'startRecording' : 'stopRecording';
    const response = await chrome.runtime.sendMessage({ action });

    if (response.success) {
      updateRecordingUI({
        recording: isRecording,
        sessionId: response.data.sessionId,
        capturedUrls: isRecording ? [] : response.data.capturedUrls,
        captureCount: isRecording ? 0 : response.data.capturedUrls.length
      });

      if (isRecording) {
        showSuccess('Recording started');
      } else {
        showSuccess(`Recording stopped. ${response.data.capturedUrls.length} pages captured.`);
      }
    } else {
      toggle.checked = !isRecording;
      showError(`Failed: ${response.error}`);
    }
  } catch (error) {
    toggle.checked = !isRecording;
    showError(`Error: ${error.message}`);
  }
}

async function stopRecordingFromBanner() {
  document.getElementById('recording-toggle').checked = false;
  await toggleRecording();
  document.getElementById('recording-toggle').checked = false;
}

function updateRecordingUI(state) {
  const indicator = document.getElementById('recording-indicator');
  const toggle = document.getElementById('recording-toggle');
  const statusLabel = document.getElementById('recording-status-label');
  const banner = document.getElementById('recording-banner');
  const bannerCount = document.getElementById('banner-count');
  const capturedCount = document.getElementById('captured-count');

  const count = state.captureCount || (state.capturedUrls ? state.capturedUrls.length : 0);

  // Update toggle
  toggle.checked = state.recording;

  // Update indicator and label
  if (state.recording) {
    indicator.classList.add('active');
    statusLabel.textContent = 'Recording Active';
    banner.classList.add('active');
    bannerCount.textContent = `(${count} pages)`;
  } else {
    indicator.classList.remove('active');
    statusLabel.textContent = 'Recording Off';
    banner.classList.remove('active');
  }

  // Update captured count
  capturedCount.textContent = count;

  // Update captured list
  if (state.capturedUrls) {
    updateCapturedList(state.capturedUrls);
  }
}

// ============================================================================
// Page Lists
// ============================================================================

function formatRelativeTime(timestamp) {
  const now = Date.now();
  const diffMs = now - timestamp;
  const diffSec = Math.floor(diffMs / 1000);
  const diffMin = Math.floor(diffSec / 60);
  const diffHour = Math.floor(diffMin / 60);

  if (diffSec < 60) {
    return 'Just now';
  } else if (diffMin < 60) {
    return `${diffMin}m ago`;
  } else if (diffHour < 24) {
    return `${diffHour}h ago`;
  } else {
    return new Date(timestamp).toLocaleDateString();
  }
}

function updateCapturedList(capturedUrls) {
  const container = document.getElementById('captured-list');

  if (!container) return;

  container.innerHTML = '';

  if (!capturedUrls || capturedUrls.length === 0) {
    container.innerHTML = '<div class="empty-state">No pages captured yet</div>';
    return;
  }

  // Limit to MAX_LIST_ITEMS, most recent first
  const items = capturedUrls.slice(-MAX_LIST_ITEMS).reverse();

  items.forEach(capture => {
    const item = document.createElement('div');
    item.className = 'page-item';

    const title = document.createElement('div');
    title.className = 'page-item-title';
    title.textContent = capture.title || capture.url || 'Untitled';
    title.title = capture.url;

    const meta = document.createElement('div');
    meta.className = 'page-item-meta';
    meta.innerHTML = `<span>${formatRelativeTime(capture.timestamp)}</span>`;

    item.appendChild(title);
    item.appendChild(meta);
    container.appendChild(item);
  });
}

// ============================================================================
// Failed Uploads
// ============================================================================

async function loadFailedUploads() {
  try {
    const response = await chrome.runtime.sendMessage({ action: 'getFailedUploads' });

    if (response.success) {
      updateFailedList(response.data);
    }
  } catch (error) {
    console.error('Error loading failed uploads:', error);
  }
}

function updateFailedList(failedUploads) {
  const section = document.getElementById('failed-section');
  const container = document.getElementById('failed-list');
  const failedCount = document.getElementById('failed-count');

  if (!failedUploads || failedUploads.length === 0) {
    section.style.display = 'none';
    failedCount.textContent = '0';
    return;
  }

  section.style.display = 'block';
  failedCount.textContent = failedUploads.length;
  container.innerHTML = '';

  // Limit to MAX_LIST_ITEMS
  const items = failedUploads.slice(0, MAX_LIST_ITEMS);

  items.forEach(entry => {
    const item = document.createElement('div');
    item.className = 'page-item failed';

    const title = document.createElement('div');
    title.className = 'page-item-title';
    title.textContent = entry.title || entry.url || 'Untitled';
    title.title = entry.url;

    const meta = document.createElement('div');
    meta.className = 'page-item-meta';

    const time = document.createElement('span');
    time.textContent = formatRelativeTime(entry.timestamp);

    const retryBtn = document.createElement('button');
    retryBtn.className = 'retry-btn';
    retryBtn.textContent = 'Retry';
    retryBtn.onclick = () => retryUpload(entry.url);

    meta.appendChild(time);
    meta.appendChild(retryBtn);
    item.appendChild(title);
    item.appendChild(meta);
    container.appendChild(item);
  });
}

async function retryUpload(url) {
  showSuccess('Retrying upload...');

  try {
    const response = await chrome.runtime.sendMessage({
      action: 'retryFailedUpload',
      url: url
    });

    if (response.success) {
      showSuccess('Upload successful!');
      await loadFailedUploads();
      await loadRecordingState();
    } else {
      showError(`Retry failed: ${response.error}`);
    }
  } catch (error) {
    showError(`Error: ${error.message}`);
  }
}

async function clearAllFailed() {
  try {
    const response = await chrome.runtime.sendMessage({ action: 'clearAllFailedUploads' });

    if (response.success) {
      showSuccess('Cleared all failed uploads');
      await loadFailedUploads();
    }
  } catch (error) {
    showError(`Error: ${error.message}`);
  }
}

// ============================================================================
// Messages
// ============================================================================

function showSuccess(message) {
  const element = document.getElementById('success-message');
  element.textContent = message;
  element.style.display = 'block';
  setTimeout(() => {
    element.style.display = 'none';
  }, 3000);
}

function showError(message) {
  const element = document.getElementById('error-message');
  element.textContent = message;
  element.style.display = 'block';
  setTimeout(() => {
    element.style.display = 'none';
  }, 5000);
}

// ============================================================================
// Periodic Refresh
// ============================================================================

// Refresh state every 5 seconds to catch auto-captures
setInterval(async () => {
  await loadRecordingState();
  await loadFailedUploads();
}, 5000);
