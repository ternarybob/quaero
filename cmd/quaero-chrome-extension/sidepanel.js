// Sidepanel script for Quaero extension

const DEFAULT_SERVER_URL = 'http://localhost:8085';
let ws;
let reconnectInterval;

// Initialize
document.addEventListener('DOMContentLoaded', async () => {
  await loadSettings();
  await updatePageInfo();
  connectWebSocket();

  // Load recording state on init
  await loadRecordingState();

  // Set up event listeners
  document.getElementById('capture-auth-btn').addEventListener('click', captureAndCrawl);
  document.getElementById('refresh-status-btn').addEventListener('click', refreshStatus);
  document.getElementById('save-settings-btn').addEventListener('click', saveSettings);
  document.getElementById('recording-toggle').addEventListener('change', toggleRecording);
});

// Load settings from storage
async function loadSettings() {
  const result = await chrome.storage.sync.get(['serverUrl']);
  const serverUrl = result.serverUrl || DEFAULT_SERVER_URL;
  document.getElementById('server-url').value = serverUrl;
}

// Save settings to storage
async function saveSettings() {
  const serverUrl = document.getElementById('server-url').value;
  await chrome.storage.sync.set({ serverUrl });
  showSuccess('Settings saved successfully');
}

// Connect to WebSocket for real-time status updates
function connectWebSocket() {
  const serverUrl = document.getElementById('server-url').value;
  const url = new URL(serverUrl);
  const wsProtocol = url.protocol === 'https:' ? 'wss:' : 'ws:';
  const wsUrl = `${wsProtocol}//${url.host}/ws`;

  ws = new WebSocket(wsUrl);

  ws.onopen = function() {
    console.log('WebSocket connected to Quaero server');
    updateServerStatus(true);
    if (reconnectInterval) {
      clearInterval(reconnectInterval);
      reconnectInterval = null;
    }
  };

  ws.onmessage = function(event) {
    const message = JSON.parse(event.data);

    if (message.type === 'status') {
      updateStatus(message.payload);
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
}

// Update server status indicator
function updateServerStatus(online) {
  const statusElement = document.getElementById('server-status');
  const versionElement = document.getElementById('version-info');

  if (online) {
    statusElement.textContent = 'Online';
    statusElement.className = 'status-value online';

    // Fetch version info once connected
    const serverUrl = document.getElementById('server-url').value;
    fetch(`${serverUrl}/api/version`)
      .then(response => response.json())
      .then(data => {
        versionElement.textContent = `Extension: v0.1.0 | Server: v${data.version}`;
      })
      .catch(() => {
        versionElement.textContent = 'Extension: v0.1.0 | Server: unknown';
      });
  } else {
    statusElement.textContent = 'Offline';
    statusElement.className = 'status-value offline';
    versionElement.textContent = 'Extension: v0.1.0 | Server: offline';
  }
}

// Update status from WebSocket message
function updateStatus(status) {
  // Extension can display additional status info if needed
  console.log('Status update:', status);
}

// Update current page info
async function updatePageInfo() {
  try {
    const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
    if (tab && tab.url) {
      const url = new URL(tab.url);
      document.getElementById('page-url').textContent = url.hostname;
    }
  } catch (error) {
    console.error('Error getting page info:', error);
  }
}

// Capture authentication and start crawl
async function captureAndCrawl() {
  const button = document.getElementById('capture-auth-btn');
  button.disabled = true;
  button.textContent = 'Capturing & Starting Crawl...';

  try {
    // Get current tab
    const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });

    if (!tab || !tab.url) {
      throw new Error('No active tab found');
    }

    const url = new URL(tab.url);
    const baseURL = `${url.protocol}//${url.host}`;

    // Get cookies
    const cookies = await chrome.cookies.getAll({ url: baseURL });

    // Extract all auth-related tokens from cookies (generic approach)
    const tokens = {};
    for (const cookie of cookies) {
      const name = cookie.name.toLowerCase();
      // Capture cookies that might contain auth info
      if (name.includes('token') || name.includes('auth') ||
          name.includes('session') || name.includes('csrf') ||
          name.includes('jwt') || name.includes('bearer')) {
        tokens[cookie.name] = cookie.value;
      }
    }

    // Build auth data
    const authData = {
      cookies: cookies,
      tokens: tokens,
      userAgent: navigator.userAgent,
      baseUrl: baseURL,
      timestamp: Date.now()
    };

    const serverUrl = document.getElementById('server-url').value;

    // Step 1: Capture authentication
    const authResponse = await fetch(`${serverUrl}/api/auth`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(authData)
    });

    if (!authResponse.ok) {
      throw new Error(`Auth capture failed: ${authResponse.status}`);
    }

    // Update last capture time
    const now = new Date().toLocaleString();
    document.getElementById('last-capture').textContent = now;

    try {
      await chrome.storage.sync.set({ lastCapture: now });
    } catch (storageError) {
      console.warn('Failed to save last capture time to storage:', storageError);
    }

    // Step 2: Start quick crawl
    const crawlRequest = {
      url: tab.url,
      cookies: cookies
    };

    const crawlResponse = await fetch(`${serverUrl}/api/job-definitions/quick-crawl`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(crawlRequest)
    });

    if (!crawlResponse.ok) {
      const errorData = await crawlResponse.json().catch(() => ({}));
      throw new Error(errorData.error || `Crawl start failed: ${crawlResponse.status}`);
    }

    const crawlResult = await crawlResponse.json();

    showSuccess(`Auth captured and crawl started! Job ID: ${crawlResult.job_id}`);

  } catch (error) {
    console.error('Capture & crawl error:', error);
    showError(`Failed to capture & crawl: ${error.message}`);
  } finally {
    button.disabled = false;
    button.textContent = 'Capture & Crawl';
  }
}

// Crawl current page
async function crawlCurrentPage() {
  const button = document.getElementById('crawl-page-btn');
  button.disabled = true;
  button.textContent = 'Starting Crawl...';

  try {
    // Get current tab
    const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });

    if (!tab || !tab.url) {
      throw new Error('No active tab found');
    }

    // Get cookies for auth (optional, will be used if available)
    const url = new URL(tab.url);
    const baseURL = `${url.protocol}//${url.host}`;
    const cookies = await chrome.cookies.getAll({ url: baseURL });

    // Build quick crawl request
    const crawlRequest = {
      url: tab.url,
      cookies: cookies
      // max_depth and max_pages will use server defaults
    };

    // Send to server
    const serverUrl = document.getElementById('server-url').value;
    const response = await fetch(`${serverUrl}/api/job-definitions/quick-crawl`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(crawlRequest)
    });

    if (!response.ok) {
      const errorData = await response.json().catch(() => ({}));
      throw new Error(errorData.error || `Server error: ${response.status}`);
    }

    const result = await response.json();

    showSuccess(`Crawl started! Job ID: ${result.job_id}`);

  } catch (error) {
    console.error('Crawl error:', error);
    showError(`Failed to start crawl: ${error.message}`);
  } finally {
    button.disabled = false;
    button.textContent = 'Crawl Current Page';
  }
}

// Refresh status
async function refreshStatus() {
  await updatePageInfo();

  // Update last capture time from storage
  const result = await chrome.storage.sync.get(['lastCapture']);
  if (result.lastCapture) {
    document.getElementById('last-capture').textContent = result.lastCapture;
  }

  // Reconnect WebSocket if disconnected
  if (!ws || ws.readyState !== WebSocket.OPEN) {
    connectWebSocket();
  }

  showSuccess('Status refreshed');
}

// Show success message
function showSuccess(message) {
  const element = document.getElementById('success-message');
  element.textContent = message;
  element.style.display = 'block';
  setTimeout(() => {
    element.style.display = 'none';
  }, 3000);
}

// Show error message
function showError(message) {
  const element = document.getElementById('error-message');
  element.textContent = message;
  element.style.display = 'block';
  setTimeout(() => {
    element.style.display = 'none';
  }, 5000);
}


// ============================================================================
// Recording Management Functions
// ============================================================================

/**
 * Load recording state from background and update UI
 */
async function loadRecordingState() {
  try {
    const response = await chrome.runtime.sendMessage({ action: 'getRecordingState' });

    if (response.success) {
      updateRecordingUI(response.data);
      
      // Update capture history display
      if (response.data.capturedUrls) {
        updateCaptureHistory(response.data.capturedUrls);
      }
    } else {
      console.error('Failed to load recording state:', response.error);
    }
  } catch (error) {
    console.error('Error loading recording state:', error);
  }
}

/**
 * Toggle recording on/off
 */
async function toggleRecording() {
  const toggle = document.getElementById('recording-toggle');
  const isRecording = toggle.checked;

  try {
    const action = isRecording ? 'startRecording' : 'stopRecording';
    const response = await chrome.runtime.sendMessage({ action });

    if (response.success) {
      if (isRecording) {
        showSuccess(`Recording started! Session: ${response.data.sessionId}`);
      } else {
        showSuccess(`Recording stopped. Captured ${response.data.capturedUrls.length} pages.`);
      }

      // Update UI to reflect new state
      updateRecordingUI({
        recording: isRecording,
        sessionId: response.data.sessionId,
        captureCount: isRecording ? 0 : response.data.capturedUrls.length
      });
    } else {
      // Revert toggle if failed
      toggle.checked = !isRecording;
      showError(`Failed to ${action}: ${response.error}`);
    }
  } catch (error) {
    // Revert toggle if failed
    toggle.checked = !isRecording;
    showError(`Error toggling recording: ${error.message}`);
  }
}

/**
 * Update recording UI elements based on state
 * @param {Object} state - Recording state object
 * @param {boolean} state.recording - Whether recording is active
 * @param {number} state.captureCount - Number of pages captured
 * @param {string} state.sessionId - Current session ID (if recording)
 */
function updateRecordingUI(state) {
  const indicator = document.getElementById('recording-indicator');
  const toggle = document.getElementById('recording-toggle');
  const captureCount = document.getElementById('capture-count');

  // Update recording indicator
  if (state.recording) {
    indicator.classList.add('active');
  } else {
    indicator.classList.remove('active');
  }

  // Update toggle checkbox
  toggle.checked = state.recording;

  // Update capture count display
  const count = state.captureCount || 0;
  captureCount.textContent = `${count} page${count !== 1 ? 's' : ''}`;
}

/**
 * Capture the current page with embedded images
 */
async function captureCurrentPage() {
  try {
    // Get current tab
    const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });

    if (!tab || !tab.url) {
      throw new Error('No active tab found');
    }

    // Check if recording is active
    const stateResponse = await chrome.runtime.sendMessage({ action: 'getRecordingState' });
    if (!stateResponse.success || !stateResponse.data.recording) {
      showError('Recording is not active. Please start recording first.');
      return;
    }

    console.log('Capturing page:', tab.url);

    // Inject content script if needed
    try {
      await chrome.scripting.executeScript({
        target: { tabId: tab.id },
        files: ['content.js']
      });
    } catch (injectError) {
      // Script might already be injected, continue
      console.log('Content script injection note:', injectError.message);
    }

    // Send message to content script to capture page with images
    const captureResponse = await chrome.tabs.sendMessage(tab.id, {
      action: 'capturePageWithImages'
    });

    if (!captureResponse.success) {
      throw new Error(captureResponse.error || 'Failed to capture page content');
    }

    console.log('Page content captured, sending to server...');

    // Get cookies for authentication
    const url = new URL(tab.url);
    const baseURL = `${url.protocol}//${url.host}`;
    const cookies = await chrome.cookies.getAll({ url: baseURL });

    // Prepare capture payload
    const capturePayload = {
      url: captureResponse.metadata.url,
      title: captureResponse.metadata.title,
      html: captureResponse.html,
      metadata: {
        ...captureResponse.metadata,
        capturedAt: new Date().toISOString(),
        sessionId: stateResponse.data.sessionId
      },
      cookies: cookies
    };

    // Post to server
    const serverUrl = document.getElementById('server-url').value;
    const serverResponse = await fetch(`${serverUrl}/api/documents/capture`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(capturePayload)
    });

    if (!serverResponse.ok) {
      const errorData = await serverResponse.json().catch(() => ({}));
      throw new Error(errorData.error || `Server error: ${serverResponse.status}`);
    }

    const result = await serverResponse.json();
    console.log('Page captured successfully:', result);

    // Track captured URL in background
    await chrome.runtime.sendMessage({
      action: 'addCapturedUrl',
      url: tab.url,
      docId: result.doc_id || result.id || 'unknown',
      title: captureResponse.metadata.title
    });

    // Refresh recording state to update capture count and history
    await loadRecordingState();

    showSuccess(`Page captured successfully! Doc ID: ${result.doc_id || result.id}`);

  } catch (error) {
    console.error('Capture error:', error);
    showError(`Failed to capture page: ${error.message}`);
  }
}


// ============================================================================
// Capture History Functions
// ============================================================================

/**
 * Format relative time for display (e.g., "2 min ago", "1 hour ago")
 * @param {number} timestamp - Unix timestamp in milliseconds
 * @returns {string} Formatted relative time string
 */
function formatRelativeTime(timestamp) {
  const now = Date.now();
  const diffMs = now - timestamp;
  const diffSec = Math.floor(diffMs / 1000);
  const diffMin = Math.floor(diffSec / 60);
  const diffHour = Math.floor(diffMin / 60);
  const diffDay = Math.floor(diffHour / 24);

  if (diffSec < 60) {
    return 'Just now';
  } else if (diffMin < 60) {
    return `${diffMin} min${diffMin !== 1 ? 's' : ''} ago`;
  } else if (diffHour < 24) {
    return `${diffHour} hour${diffHour !== 1 ? 's' : ''} ago`;
  } else {
    return `${diffDay} day${diffDay !== 1 ? 's' : ''} ago`;
  }
}

/**
 * Update the capture history display
 * @param {Array} capturedUrls - Array of captured URL objects with {url, docId, title, timestamp}
 */
function updateCaptureHistory(capturedUrls) {
  const container = document.getElementById('capture-history-list');
  
  if (!container) {
    console.warn('Capture history container not found');
    return;
  }

  // Clear existing content
  container.innerHTML = '';

  // Check if there are any captures
  if (!capturedUrls || capturedUrls.length === 0) {
    container.innerHTML = '<div class="empty-state">No pages captured yet</div>';
    return;
  }

  // Render each capture as a list item
  capturedUrls.forEach(capture => {
    const item = document.createElement('div');
    item.className = 'capture-history-item';
    
    const title = document.createElement('div');
    title.className = 'capture-item-title';
    title.textContent = capture.title || capture.url || 'Untitled Page';
    title.title = capture.title || capture.url; // Tooltip for truncated text
    
    const time = document.createElement('div');
    time.className = 'capture-item-time';
    time.textContent = formatRelativeTime(capture.timestamp);
    
    item.appendChild(title);
    item.appendChild(time);
    container.appendChild(item);
  });
}
