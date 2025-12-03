// Background service worker for Quaero extension

console.log('Quaero extension loaded');
console.log('Extension ID:', chrome.runtime.id);

// For testing: Store extension ID in chrome.storage so it can be accessed
// This allows automated tests to discover the extension ID
if (chrome.runtime.id) {
  chrome.storage.local.set({ extensionId: chrome.runtime.id }, () => {
    console.log('Extension ID stored for testing:', chrome.runtime.id);
  });
}

// ============================================================================
// Sidepanel as Default Action
// ============================================================================

// Automatically open sidepanel when extension icon is clicked
chrome.sidePanel.setPanelBehavior({ openPanelOnActionClick: true })
  .catch((error) => console.error('Failed to set panel behavior:', error));

// ============================================================================
// Recording State Management
// ============================================================================

/**
 * Generate a unique session ID for recording sessions
 * @returns {string} Unique session ID with timestamp
 */
function generateSessionId() {
  const timestamp = Date.now();
  const random = Math.random().toString(36).substring(2, 15);
  return `session_${timestamp}_${random}`;
}

/**
 * Start a new recording session
 * @returns {Promise<{sessionId: string, timestamp: number}>}
 */
async function startRecording() {
  const sessionId = generateSessionId();
  const timestamp = Date.now();

  const recordingState = {
    recording: true,
    sessionId: sessionId,
    startTime: timestamp,
    capturedUrls: []
  };

  await chrome.storage.local.set({ recordingState });
  console.log('Recording started:', sessionId);

  return { sessionId, timestamp };
}

/**
 * Stop the current recording session
 * @returns {Promise<{sessionId: string, capturedUrls: Array}>}
 */
async function stopRecording() {
  const { recordingState } = await chrome.storage.local.get('recordingState');

  if (!recordingState) {
    throw new Error('No active recording session');
  }

  const sessionId = recordingState.sessionId;
  const capturedUrls = recordingState.capturedUrls || [];
  const stopTime = Date.now();

  // Preserve session history
  const sessionHistory = {
    sessionId: sessionId,
    startTime: recordingState.startTime,
    stopTime: stopTime,
    duration: stopTime - recordingState.startTime,
    capturedUrls: capturedUrls,
    captureCount: capturedUrls.length
  };

  // Store in session history
  const { sessionHistories = [] } = await chrome.storage.local.get('sessionHistories');
  sessionHistories.push(sessionHistory);

  // Update recording state to stopped
  recordingState.recording = false;
  recordingState.stopTime = stopTime;

  await chrome.storage.local.set({
    recordingState,
    sessionHistories
  });

  console.log('Recording stopped:', sessionId, 'Captures:', capturedUrls.length);

  return { sessionId, capturedUrls };
}

/**
 * Get the current recording state
 * @returns {Promise<{recording: boolean, sessionId?: string, startTime?: number, capturedUrls?: Array}>}
 */
async function getRecordingState() {
  const { recordingState } = await chrome.storage.local.get('recordingState');

  if (!recordingState) {
    return { recording: false };
  }

  return {
    recording: recordingState.recording,
    sessionId: recordingState.sessionId,
    startTime: recordingState.startTime,
    capturedUrls: recordingState.capturedUrls || [],
    captureCount: (recordingState.capturedUrls || []).length
  };
}

/**
 * Add a captured URL to the current recording session
 * @param {string} url - The URL that was captured
 * @param {string} docId - The document ID assigned by the backend
 * @param {string} title - The page title
 * @returns {Promise<boolean>} Success status
 */
async function addCapturedUrl(url, docId, title) {
  const { recordingState } = await chrome.storage.local.get('recordingState');

  if (!recordingState || !recordingState.recording) {
    console.warn('Cannot add captured URL: No active recording session');
    return false;
  }

  const captureEntry = {
    url: url,
    docId: docId,
    title: title,
    timestamp: Date.now()
  };

  recordingState.capturedUrls = recordingState.capturedUrls || [];
  recordingState.capturedUrls.push(captureEntry);

  await chrome.storage.local.set({ recordingState });
  console.log('Captured URL added to session:', url, 'docId:', docId);

  return true;
}

/**
 * Add a failed upload entry
 * @param {string} url - The URL that failed to upload
 * @param {string} title - The page title
 * @param {string} error - The error message
 * @param {string} html - The captured HTML (for retry)
 * @returns {Promise<boolean>} Success status
 */
async function addFailedUpload(url, title, error, html) {
  const { failedUploads = [] } = await chrome.storage.local.get('failedUploads');

  const failedEntry = {
    url: url,
    title: title,
    error: error,
    html: html,
    timestamp: Date.now()
  };

  // Keep only most recent 20 failed uploads
  failedUploads.unshift(failedEntry);
  if (failedUploads.length > 20) {
    failedUploads.length = 20;
  }

  await chrome.storage.local.set({ failedUploads });
  console.log('Failed upload added:', url, 'error:', error);

  return true;
}

/**
 * Get all failed uploads
 * @returns {Promise<Array>} Array of failed upload entries
 */
async function getFailedUploads() {
  const { failedUploads = [] } = await chrome.storage.local.get('failedUploads');
  return failedUploads;
}

/**
 * Clear a failed upload entry (after successful retry)
 * @param {string} url - The URL to remove from failed uploads
 * @returns {Promise<boolean>} Success status
 */
async function clearFailedUpload(url) {
  const { failedUploads = [] } = await chrome.storage.local.get('failedUploads');
  const filtered = failedUploads.filter(entry => entry.url !== url);
  await chrome.storage.local.set({ failedUploads: filtered });
  console.log('Cleared failed upload:', url);
  return true;
}

/**
 * Clear all failed uploads
 * @returns {Promise<boolean>} Success status
 */
async function clearAllFailedUploads() {
  await chrome.storage.local.set({ failedUploads: [] });
  console.log('Cleared all failed uploads');
  return true;
}

// ============================================================================
// Message Handlers
// ============================================================================

// Listen for messages from popup (if needed for advanced features)
chrome.runtime.onMessage.addListener((request, sender, sendResponse) => {
  // Recording state management handlers
  if (request.action === 'startRecording') {
    startRecording()
      .then(result => {
        sendResponse({ success: true, data: result });
      })
      .catch(error => {
        sendResponse({ success: false, error: error.message });
      });
    return true; // Keep message channel open for async response
  }

  if (request.action === 'stopRecording') {
    stopRecording()
      .then(result => {
        sendResponse({ success: true, data: result });
      })
      .catch(error => {
        sendResponse({ success: false, error: error.message });
      });
    return true; // Keep message channel open for async response
  }

  if (request.action === 'getRecordingState') {
    getRecordingState()
      .then(state => {
        sendResponse({ success: true, data: state });
      })
      .catch(error => {
        sendResponse({ success: false, error: error.message });
      });
    return true; // Keep message channel open for async response
  }

  if (request.action === 'addCapturedUrl') {
    const { url, docId, title } = request;
    addCapturedUrl(url, docId, title)
      .then(success => {
        sendResponse({ success: success });
      })
      .catch(error => {
        sendResponse({ success: false, error: error.message });
      });
    return true; // Keep message channel open for async response
  }

  // Failed uploads handlers
  if (request.action === 'getFailedUploads') {
    getFailedUploads()
      .then(uploads => {
        sendResponse({ success: true, data: uploads });
      })
      .catch(error => {
        sendResponse({ success: false, error: error.message });
      });
    return true;
  }

  if (request.action === 'clearFailedUpload') {
    clearFailedUpload(request.url)
      .then(success => {
        sendResponse({ success: success });
      })
      .catch(error => {
        sendResponse({ success: false, error: error.message });
      });
    return true;
  }

  if (request.action === 'clearAllFailedUploads') {
    clearAllFailedUploads()
      .then(success => {
        sendResponse({ success: success });
      })
      .catch(error => {
        sendResponse({ success: false, error: error.message });
      });
    return true;
  }

  if (request.action === 'retryFailedUpload') {
    (async () => {
      try {
        const { failedUploads = [] } = await chrome.storage.local.get('failedUploads');
        const entry = failedUploads.find(e => e.url === request.url);
        if (!entry) {
          sendResponse({ success: false, error: 'Entry not found' });
          return;
        }
        // Try to send to backend again
        const result = await sendCaptureToBackend(entry.html, { title: entry.title, url: entry.url }, entry.url);
        // Remove from failed uploads on success
        await clearFailedUpload(entry.url);
        // Add to captured URLs if recording
        await addCapturedUrl(entry.url, result.docId, entry.title);
        sendResponse({ success: true, data: result });
      } catch (error) {
        sendResponse({ success: false, error: error.message });
      }
    })();
    return true;
  }

  // Existing auth capture handler
  if (request.action === 'captureAuth') {
    captureAuthData()
      .then(authData => {
        sendResponse({ success: true, data: authData });
      })
      .catch(error => {
        sendResponse({ success: false, error: error.message });
      });
    return true; // Keep message channel open for async response
  }
});

// ============================================================================
// Helper Functions
// ============================================================================

// Capture authentication data from current tab
async function captureAuthData() {
  const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });

  if (!tab || !tab.url) {
    throw new Error('No active tab found');
  }

  const url = new URL(tab.url);
  const baseURL = `${url.protocol}//${url.host}`;

  // Get cookies for the domain
  const cookies = await chrome.cookies.getAll({ url: baseURL });

  // Inject content script to extract auth tokens from page (generic approach)
  const [{ result: pageTokens }] = await chrome.scripting.executeScript({
    target: { tabId: tab.id },
    func: () => {
      const tokens = {};

      // Extract common auth-related meta tags
      const metaTags = document.querySelectorAll('meta[name], meta[property]');
      metaTags.forEach(meta => {
        const name = meta.getAttribute('name') || meta.getAttribute('property');
        const content = meta.getAttribute('content');
        if (name && content) {
          // Store any meta tag that might contain auth info
          if (name.toLowerCase().includes('token') ||
              name.toLowerCase().includes('csrf') ||
              name.toLowerCase().includes('auth') ||
              name.toLowerCase().includes('session')) {
            tokens[name] = content;
          }
        }
      });

      // Try to extract auth tokens from localStorage
      try {
        for (let i = 0; i < localStorage.length; i++) {
          const key = localStorage.key(i);
          if (key && (key.toLowerCase().includes('token') ||
                      key.toLowerCase().includes('auth') ||
                      key.toLowerCase().includes('session'))) {
            tokens[key] = localStorage.getItem(key);
          }
        }
      } catch (e) {
        // localStorage might be blocked
      }

      return tokens;
    }
  });

  // Build tokens object from page tokens and cookies
  const tokens = { ...pageTokens };

  // Get user agent
  const userAgent = navigator.userAgent;

  // Return auth data
  return {
    cookies: cookies,
    tokens: tokens,
    userAgent: userAgent,
    baseUrl: baseURL,
    timestamp: Date.now()
  };
}

// ============================================================================
// Auto-Capture Tab Navigation
// ============================================================================

// Track last capture time per tab to implement debouncing
const lastCaptureTime = new Map();

// Debounce threshold in milliseconds
const DEBOUNCE_THRESHOLD_MS = 1000;

/**
 * Check if a URL should be skipped for auto-capture
 * @param {string} url - The URL to check
 * @returns {boolean} True if the URL should be skipped
 */
function shouldSkipUrl(url) {
  if (!url) return true;

  const protocolsToSkip = [
    'chrome://',
    'chrome-extension://',
    'about:',
    'file://',
    'edge://',
    'extension://'
  ];

  return protocolsToSkip.some(protocol => url.startsWith(protocol));
}

/**
 * Check if a URL was already captured in the current session
 * @param {string} url - The URL to check
 * @returns {Promise<boolean>} True if the URL was already captured
 */
async function isUrlAlreadyCaptured(url) {
  const { recordingState } = await chrome.storage.local.get('recordingState');

  if (!recordingState || !recordingState.capturedUrls) {
    return false;
  }

  return recordingState.capturedUrls.some(entry => entry.url === url);
}

/**
 * Get the server URL from storage
 * @returns {Promise<string>} The server URL
 */
async function getServerUrl() {
  const { serverUrl } = await chrome.storage.sync.get({
    serverUrl: 'http://localhost:8085'
  });
  return serverUrl;
}

/**
 * Send captured HTML and metadata to the backend server
 * @param {string} html - The captured HTML
 * @param {Object} metadata - Page metadata
 * @param {string} url - The page URL
 * @returns {Promise<{docId: string}>} The document ID from the server
 */
async function sendCaptureToBackend(html, metadata, url) {
  const serverUrl = await getServerUrl();
  const endpoint = `${serverUrl}/api/documents/capture`;

  // Build payload matching server's CaptureRequest format
  const payload = {
    url: url,
    html: html,
    title: metadata.title || '',
    description: metadata.description || '',
    timestamp: metadata.timestamp || new Date().toISOString()
  };

  console.log('Sending capture to backend:', endpoint, 'URL:', url);

  const response = await fetch(endpoint, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json'
    },
    body: JSON.stringify(payload)
  });

  if (!response.ok) {
    const errorText = await response.text();
    throw new Error(`Backend capture failed: ${response.status} - ${errorText}`);
  }

  const result = await response.json();
  console.log('Capture sent successfully, docId:', result.document_id);

  return result;
}

/**
 * Capture authentication data for a specific URL
 * @param {string} url - The URL to capture auth data for
 * @returns {Promise<Object>} Authentication data
 */
async function captureAuthDataForUrl(url) {
  try {
    const urlObj = new URL(url);
    const baseURL = `${urlObj.protocol}//${urlObj.host}`;

    // Get cookies for the domain
    const cookies = await chrome.cookies.getAll({ url: baseURL });

    // Get user agent
    const userAgent = navigator.userAgent;

    return {
      cookies: cookies,
      tokens: {},
      userAgent: userAgent,
      baseUrl: baseURL,
      timestamp: Date.now()
    };
  } catch (error) {
    console.warn('Failed to capture auth data for URL:', url, error);
    return {
      cookies: [],
      tokens: {},
      userAgent: navigator.userAgent,
      baseUrl: '',
      timestamp: Date.now()
    };
  }
}

/**
 * Check if URL has a matching job definition config
 * @param {string} url - The URL to check
 * @returns {Promise<boolean>} True if URL has matching config
 */
async function hasMatchingConfig(url) {
  try {
    const serverUrl = await getServerUrl();
    const response = await fetch(
      `${serverUrl}/api/job-definitions/match-config?url=${encodeURIComponent(url)}`
    );

    if (!response.ok) {
      console.warn('Failed to check matching config for URL:', url);
      return false;
    }

    const config = await response.json();
    return config.matched === true;
  } catch (error) {
    console.warn('Error checking matching config:', error);
    return false;
  }
}

/**
 * Perform auto-capture for a tab
 * @param {number} tabId - The tab ID
 * @param {string} url - The tab URL
 */
async function performAutoCapture(tabId, url) {
  try {
    console.log('Auto-capture triggered for tab:', tabId, 'URL:', url);

    // Check if URL should be skipped
    if (shouldSkipUrl(url)) {
      console.log('Auto-capture skipped - URL protocol not supported:', url);
      return;
    }

    // Check if recording is enabled
    const recordingState = await getRecordingState();
    if (!recordingState.recording) {
      console.log('Auto-capture skipped - recording not enabled');
      return;
    }

    // Check if URL has a matching job definition config
    const hasConfig = await hasMatchingConfig(url);
    if (!hasConfig) {
      console.log('Auto-capture skipped - no matching job definition config for URL:', url);
      return;
    }

    // Check debounce threshold
    const now = Date.now();
    const lastCapture = lastCaptureTime.get(tabId);
    if (lastCapture && (now - lastCapture) < DEBOUNCE_THRESHOLD_MS) {
      console.log('Auto-capture skipped - debounce threshold not met:', now - lastCapture, 'ms');
      return;
    }

    // Check if URL already captured
    const alreadyCaptured = await isUrlAlreadyCaptured(url);
    if (alreadyCaptured) {
      console.log('Auto-capture skipped - URL already captured in session:', url);
      return;
    }

    // Update last capture time
    lastCaptureTime.set(tabId, now);

    // Inject content script if not already injected
    try {
      await chrome.scripting.executeScript({
        target: { tabId: tabId },
        files: ['content.js']
      });
    } catch (error) {
      // Content script might already be injected, continue
      console.log('Content script injection note:', error.message);
    }

    // Trigger capture with images
    const response = await chrome.tabs.sendMessage(tabId, {
      action: 'capturePageWithImages'
    });

    if (!response.success) {
      throw new Error(response.error || 'Capture failed');
    }

    console.log('Page captured successfully:', url);

    // Send to backend
    try {
      const backendResult = await sendCaptureToBackend(
        response.html,
        response.metadata,
        url
      );

      // Update captured URLs list
      await addCapturedUrl(url, backendResult.document_id, response.metadata.title);

      console.log('Auto-capture completed successfully for:', url, 'docId:', backendResult.document_id);
    } catch (backendError) {
      console.error('Backend upload failed for:', url, 'Error:', backendError);
      // Track the failed upload for retry
      await addFailedUpload(url, response.metadata.title, backendError.message, response.html);
    }

  } catch (error) {
    console.error('Auto-capture failed for tab:', tabId, 'URL:', url, 'Error:', error);
  }
}

/**
 * Tab update listener for auto-capture
 */
chrome.tabs.onUpdated.addListener((tabId, changeInfo, tab) => {
  // Only trigger when page load is complete
  if (changeInfo.status === 'complete' && tab.url) {
    // Perform auto-capture asynchronously (fire and forget)
    performAutoCapture(tabId, tab.url).catch(error => {
      console.error('Auto-capture error:', error);
    });
  }
});
