// Sidepanel script for Quaero extension

const DEFAULT_SERVER_URL = 'http://localhost:8085';
let ws;
let reconnectInterval;

// Initialize
document.addEventListener('DOMContentLoaded', async () => {
  await loadSettings();
  await updatePageInfo();
  connectWebSocket();

  // Set up event listeners
  document.getElementById('capture-auth-btn').addEventListener('click', captureAndCrawl);
  document.getElementById('refresh-status-btn').addEventListener('click', refreshStatus);
  document.getElementById('save-settings-btn').addEventListener('click', saveSettings);
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
