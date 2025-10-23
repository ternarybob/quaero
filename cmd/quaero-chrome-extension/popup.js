// Popup script for Quaero extension

const DEFAULT_SERVER_URL = 'http://localhost:8085';
let serverUrl = DEFAULT_SERVER_URL;

// Initialize
document.addEventListener('DOMContentLoaded', async () => {
  await loadSettings();
  await updatePageInfo();
  await checkServerStatus();
  await loadLastCapture();

  // Set up event listeners
  document.getElementById('capture-auth-btn').addEventListener('click', captureAuth);
  document.getElementById('refresh-status-btn').addEventListener('click', refreshStatus);
  document.getElementById('save-settings-btn').addEventListener('click', saveSettings);
  document.getElementById('settings-toggle').addEventListener('click', toggleSettings);
});

// Load settings from storage
async function loadSettings() {
  const result = await chrome.storage.sync.get(['serverUrl']);
  serverUrl = result.serverUrl || DEFAULT_SERVER_URL;
  document.getElementById('server-url').value = serverUrl;
}

// Save settings to storage
async function saveSettings() {
  serverUrl = document.getElementById('server-url').value;
  await chrome.storage.sync.set({ serverUrl });
  showMessage('Settings saved successfully', 'success');
  await checkServerStatus();
}

// Toggle settings section
function toggleSettings() {
  const toggle = document.getElementById('settings-toggle');
  const content = document.getElementById('settings-content');
  
  toggle.classList.toggle('collapsed');
  content.classList.toggle('hidden');
}

// Check server status
async function checkServerStatus() {
  const statusElement = document.getElementById('server-status');
  const versionElement = document.getElementById('version-info');

  try {
    const response = await fetch(`${serverUrl}/api/version`, {
      method: 'GET',
      signal: AbortSignal.timeout(3000)
    });

    if (response.ok) {
      const data = await response.json();
      statusElement.textContent = 'Online';
      statusElement.className = 'status-value online';
      versionElement.textContent = `Extension: v0.1.0 | Server: v${data.version}`;
      return true;
    }
  } catch (error) {
    // Server is offline or unreachable
  }

  statusElement.textContent = 'Offline';
  statusElement.className = 'status-value offline';
  versionElement.textContent = 'Extension: v0.1.0 | Server: offline';
  return false;
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
    document.getElementById('page-url').textContent = 'Unknown';
  }
}

// Load last capture time from storage
async function loadLastCapture() {
  const result = await chrome.storage.sync.get(['lastCapture']);
  if (result.lastCapture) {
    document.getElementById('last-capture').textContent = result.lastCapture;
  }
}

// Capture authentication
async function captureAuth() {
  const button = document.getElementById('capture-auth-btn');
  button.disabled = true;
  button.textContent = 'Capturing...';

  try {
    // Get current tab
    const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });

    if (!tab || !tab.url) {
      throw new Error('No active tab found');
    }

    const url = new URL(tab.url);
    const baseURL = `${url.protocol}//${url.host}`;

    // Check if on Atlassian domain
    if (!url.hostname.includes('atlassian.net') && !url.hostname.includes('jira.com') && !url.hostname.includes('confluence')) {
      showMessage('⚠️ Please navigate to a Jira or Confluence page first', 'error');
      return;
    }

    // Get cookies
    const cookies = await chrome.cookies.getAll({ url: baseURL });

    if (cookies.length === 0) {
      throw new Error('No cookies found. Make sure you are logged in.');
    }

    // Extract tokens from cookies
    const tokens = {};
    for (const cookie of cookies) {
      if (cookie.name.includes('cloud') || cookie.name.includes('atl')) {
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

    // Send to server
    showMessage('Sending to Quaero server...', 'info');
    
    const response = await fetch(`${serverUrl}/api/auth`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(authData)
    });

    if (!response.ok) {
      const errorText = await response.text();
      throw new Error(`Server error: ${response.status} - ${errorText}`);
    }

    const result = await response.json();

    // Update last capture time
    const now = new Date().toLocaleString();
    document.getElementById('last-capture').textContent = now;
    await chrome.storage.sync.set({ lastCapture: now });

    showMessage(`✓ ${result.message || 'Authentication captured successfully!'}`, 'success');

  } catch (error) {
    console.error('Capture error:', error);
    showMessage(`✗ Error: ${error.message}`, 'error');
  } finally {
    button.disabled = false;
    button.textContent = 'Capture Authentication';
  }
}

// Refresh status
async function refreshStatus() {
  const button = document.getElementById('refresh-status-btn');
  button.disabled = true;
  button.textContent = 'Refreshing...';

  try {
    await updatePageInfo();
    await loadLastCapture();
    const isOnline = await checkServerStatus();
    
    showMessage(isOnline ? 'Status refreshed - Server online' : 'Status refreshed - Server offline', isOnline ? 'success' : 'error');
  } catch (error) {
    showMessage('Error refreshing status', 'error');
  } finally {
    button.disabled = false;
    button.textContent = 'Refresh Status';
  }
}

// Show message
function showMessage(message, type = 'info') {
  const messageBox = document.getElementById('message-box');
  messageBox.textContent = message;
  messageBox.className = `message-box ${type}`;
  messageBox.style.display = 'block';

  // Auto-hide after 4 seconds
  setTimeout(() => {
    messageBox.style.display = 'none';
  }, 4000);
}
