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
  document.getElementById('capture-only-btn').addEventListener('click', captureOnly);
  document.getElementById('capture-auth-btn').addEventListener('click', captureAndCrawl);
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

// Capture page content only (no crawl)
async function captureOnly() {
  const button = document.getElementById('capture-only-btn');
  button.disabled = true;
  button.textContent = 'Capturing...';

  try {
    // Get current tab
    const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });

    if (!tab || !tab.url) {
      throw new Error('No active tab found');
    }

    // Inject content script first (in case it's not already loaded)
    showMessage('Capturing page content...', 'info');

    try {
      await chrome.scripting.executeScript({
        target: { tabId: tab.id },
        files: ['content.js']
      });
    } catch (injectError) {
      // Script might already be injected or page doesn't allow scripts
      console.log('Script injection note:', injectError.message);
    }

    // Small delay to ensure script is ready
    await new Promise(resolve => setTimeout(resolve, 100));

    // Request page content from content script
    const response = await chrome.tabs.sendMessage(tab.id, { action: 'capturePageContent' });

    if (!response || !response.success) {
      throw new Error(response?.error || 'Failed to capture page content');
    }

    // Send to server
    showMessage('Sending to server...', 'info');

    const captureRequest = {
      url: tab.url,
      html: response.html,
      title: response.metadata.title,
      description: response.metadata.description,
      timestamp: response.metadata.timestamp
    };

    const captureResponse = await fetch(`${serverUrl}/api/documents/capture`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(captureRequest)
    });

    if (!captureResponse.ok) {
      const errorData = await captureResponse.json().catch(() => ({}));
      throw new Error(errorData.error || `Capture failed: ${captureResponse.status}`);
    }

    const result = await captureResponse.json();

    // Update last capture time
    const now = new Date().toLocaleString();
    document.getElementById('last-capture').textContent = now;

    try {
      await chrome.storage.sync.set({ lastCapture: now });
    } catch (storageError) {
      console.warn('Failed to save last capture time:', storageError);
    }

    showMessage(`Page captured! Document ID: ${result.document_id}`, 'success');

  } catch (error) {
    console.error('Capture error:', error);
    showMessage(`Error: ${error.message}`, 'error');
  } finally {
    button.disabled = false;
    button.textContent = 'Capture Page';
  }
}

// Capture authentication and start crawl (using extension-captured HTML)
async function captureAndCrawl() {
  const button = document.getElementById('capture-auth-btn');
  button.disabled = true;
  button.textContent = 'Capturing & Crawling...';

  try {
    // Get current tab
    const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });

    if (!tab || !tab.url) {
      throw new Error('No active tab found');
    }

    const url = new URL(tab.url);
    const baseURL = `${url.protocol}//${url.host}`;

    // Step 1: Inject content script and capture page HTML
    showMessage('Capturing page content...', 'info');

    try {
      await chrome.scripting.executeScript({
        target: { tabId: tab.id },
        files: ['content.js']
      });
    } catch (injectError) {
      console.log('Script injection note:', injectError.message);
    }

    await new Promise(resolve => setTimeout(resolve, 100));

    const pageContent = await chrome.tabs.sendMessage(tab.id, { action: 'capturePageContent' });

    if (!pageContent || !pageContent.success) {
      throw new Error(pageContent?.error || 'Failed to capture page content');
    }

    // Step 2: Get cookies for authentication
    showMessage('Capturing authentication...', 'info');
    const cookies = await chrome.cookies.getAll({ url: baseURL });

    // Extract auth tokens
    const tokens = {};
    for (const cookie of cookies) {
      const name = cookie.name.toLowerCase();
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

    // Send auth to server
    const authResponse = await fetch(`${serverUrl}/api/auth`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
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
      console.warn('Failed to save last capture time:', storageError);
    }

    // Step 3: Start crawl with captured HTML (no browser needed)
    showMessage('Starting crawl with captured content...', 'info');

    const crawlRequest = {
      url: tab.url,
      cookies: cookies,
      html: pageContent.html,           // Include captured HTML
      title: pageContent.metadata.title,
      use_captured_html: true           // Tell server to use captured HTML
    };

    const crawlResponse = await fetch(`${serverUrl}/api/job-definitions/quick-crawl`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(crawlRequest)
    });

    if (!crawlResponse.ok) {
      const errorData = await crawlResponse.json().catch(() => ({}));
      throw new Error(errorData.error || `Crawl start failed: ${crawlResponse.status}`);
    }

    const crawlResult = await crawlResponse.json();

    showMessage(`✓ Page captured and crawl started! Job: ${crawlResult.job_id}`, 'success');

  } catch (error) {
    console.error('Capture & crawl error:', error);
    showMessage(`✗ Error: ${error.message}`, 'error');
  } finally {
    button.disabled = false;
    button.textContent = 'Capture & Crawl';
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
