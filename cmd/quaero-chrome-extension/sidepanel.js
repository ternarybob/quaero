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
  document.getElementById('refresh-config-btn').addEventListener('click', refreshServerConfig);
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

async function refreshServerConfig() {
  const serverUrl = document.getElementById('server-url').value;
  const btn = document.getElementById('refresh-config-btn');

  btn.disabled = true;
  btn.textContent = 'Refreshing...';

  try {
    // Call server to reload job definitions
    const response = await fetch(`${serverUrl}/api/job-definitions/reload`, {
      method: 'POST'
    });

    if (!response.ok) {
      throw new Error('Failed to reload config');
    }

    const result = await response.json();
    showSuccess(`Config reloaded: ${result.loaded || 0} job definitions`);

    // Refresh the current page's config display
    await refreshLinksAndConfig();

  } catch (error) {
    console.error('Error refreshing config:', error);
    showError('Failed to refresh config: ' + error.message);
  } finally {
    btn.disabled = false;
    btn.textContent = 'Refresh Config';
  }
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
      // Only log as error if actively crawling, otherwise it's just a normal disconnect
      if (isCrawling) {
        console.error('WebSocket error during active crawl:', error);
      } else {
        console.warn('WebSocket connection issue (will reconnect):', error);
      }
      updateServerStatus(false);
    };

    ws.onclose = function() {
      // Only log as error if disconnected during active crawl
      if (isCrawling) {
        console.error('WebSocket disconnected during active crawl');
      } else {
        console.log('WebSocket disconnected (will reconnect)');
      }
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

// ============================================================================
// Crawl & Capture
// ============================================================================

let currentCrawlConfig = null;
let currentLinks = [];
let isCrawling = false;

// Initialize crawl section when panel opens
document.addEventListener('DOMContentLoaded', async () => {
  // Set up crawl event listeners
  document.getElementById('start-crawl-btn').addEventListener('click', startCrawl);
  document.getElementById('refresh-links-btn').addEventListener('click', refreshLinksAndConfig);

  // Initial load
  await refreshLinksAndConfig();
});

// Check for matching config and extract links from current page
async function refreshLinksAndConfig() {
  const configIndicator = document.getElementById('config-indicator');
  const configName = document.getElementById('config-name');
  const startBtn = document.getElementById('start-crawl-btn');
  const linksPreview = document.getElementById('crawl-links-preview');

  configIndicator.className = 'config-indicator';
  configName.textContent = 'Checking...';
  startBtn.disabled = true;

  try {
    // Get current tab URL
    const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
    if (!tab || !tab.url) {
      configName.textContent = 'No active tab';
      return;
    }

    const serverUrl = document.getElementById('server-url').value;

    // Get matching config from server
    const configResponse = await fetch(
      `${serverUrl}/api/job-definitions/match-config?url=${encodeURIComponent(tab.url)}`
    );

    if (!configResponse.ok) {
      throw new Error('Failed to get config');
    }

    currentCrawlConfig = await configResponse.json();

    // Update UI based on match
    if (currentCrawlConfig.matched) {
      configIndicator.className = 'config-indicator matched';
      configName.textContent = currentCrawlConfig.job_definition.name;
    } else {
      configIndicator.className = 'config-indicator no-match';
      configName.textContent = 'Default config (no match)';
    }

    // Extract links from current page
    await extractLinksFromPage(tab.id);

    // Enable start button if we have links
    startBtn.disabled = currentLinks.length === 0;
    linksPreview.style.display = 'block';

  } catch (error) {
    console.error('Error refreshing config:', error);
    configIndicator.className = 'config-indicator';
    configName.textContent = 'Error loading config';
  }
}

// Extract links from the current page and filter them
async function extractLinksFromPage(tabId) {
  const linksList = document.getElementById('crawl-links-list');
  const linksCount = document.getElementById('links-count');

  try {
    // Inject script to extract links
    const results = await chrome.scripting.executeScript({
      target: { tabId: tabId },
      func: () => {
        const links = [];
        const seen = new Set();

        document.querySelectorAll('a[href]').forEach(a => {
          const href = a.getAttribute('href');
          if (!href) return;

          // Skip non-http links
          if (href.startsWith('javascript:') ||
              href.startsWith('mailto:') ||
              href.startsWith('tel:') ||
              href.startsWith('#')) {
            return;
          }

          try {
            const absoluteUrl = new URL(href, window.location.href).href;
            // Only same-origin links
            if (new URL(absoluteUrl).origin === window.location.origin) {
              if (!seen.has(absoluteUrl)) {
                seen.add(absoluteUrl);
                links.push(absoluteUrl);
              }
            }
          } catch (e) {
            // Invalid URL
          }
        });

        return links;
      }
    });

    let extractedLinks = results[0]?.result || [];

    // Apply include/exclude patterns if we have config
    if (currentCrawlConfig?.crawl_config) {
      extractedLinks = filterLinks(
        extractedLinks,
        currentCrawlConfig.crawl_config.include_patterns || [],
        currentCrawlConfig.crawl_config.exclude_patterns || []
      );
    }

    currentLinks = extractedLinks;

    // Update UI
    linksCount.textContent = `${currentLinks.length} links found`;
    linksList.innerHTML = '';

    if (currentLinks.length === 0) {
      linksList.innerHTML = '<div class="link-item">No matching links found</div>';
    } else {
      // Show first 20 links
      const displayLinks = currentLinks.slice(0, 20);
      displayLinks.forEach(link => {
        const item = document.createElement('div');
        item.className = 'link-item';
        item.textContent = link;
        item.title = link;
        linksList.appendChild(item);
      });

      if (currentLinks.length > 20) {
        const more = document.createElement('div');
        more.className = 'link-item';
        more.style.color = '#7f8c8d';
        more.textContent = `... and ${currentLinks.length - 20} more`;
        linksList.appendChild(more);
      }
    }

  } catch (error) {
    console.error('Error extracting links:', error);
    linksCount.textContent = 'Error extracting links';
    linksList.innerHTML = '<div class="link-item">Failed to extract links</div>';
    currentLinks = [];
  }
}

// Filter links using include/exclude patterns
function filterLinks(links, includePatterns, excludePatterns) {
  return links.filter(link => {
    // Check exclude patterns first
    for (const pattern of excludePatterns) {
      if (matchPattern(link, pattern)) {
        return false;
      }
    }

    // If no include patterns, include all non-excluded
    if (includePatterns.length === 0) {
      return true;
    }

    // Check include patterns
    for (const pattern of includePatterns) {
      if (matchPattern(link, pattern)) {
        return true;
      }
    }

    return false;
  });
}

// Match URL against a pattern (simple substring match for now)
function matchPattern(url, pattern) {
  // Convert simple patterns to work as substring matches
  // Patterns like "/wiki/spaces/" should match if URL contains it
  if (pattern.startsWith('/')) {
    // Path pattern - check if URL path contains it
    try {
      const urlPath = new URL(url).pathname;
      return urlPath.includes(pattern);
    } catch {
      return false;
    }
  }

  // Otherwise do simple substring match
  return url.includes(pattern);
}

// Start the crawl
async function startCrawl() {
  if (isCrawling || currentLinks.length === 0) return;

  const startBtn = document.getElementById('start-crawl-btn');
  const btnText = document.getElementById('crawl-btn-text');
  const progressDiv = document.getElementById('crawl-progress');
  const progressFill = document.getElementById('crawl-progress-fill');
  const progressText = document.getElementById('crawl-progress-text');
  const includeCurrentPage = document.getElementById('include-current-page').checked;

  isCrawling = true;
  startBtn.disabled = true;
  btnText.textContent = 'Crawling...';
  progressDiv.style.display = 'block';
  progressFill.style.width = '0%';
  progressText.textContent = 'Starting crawl...';

  try {
    const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
    if (!tab) throw new Error('No active tab');

    const serverUrl = document.getElementById('server-url').value;

    // Get cookies for authentication
    const cookies = await chrome.cookies.getAll({ url: tab.url });

    // Get current page HTML if including it
    let html = '';
    let title = tab.title || '';

    if (includeCurrentPage) {
      const results = await chrome.scripting.executeScript({
        target: { tabId: tab.id },
        func: () => document.documentElement.outerHTML
      });
      html = results[0]?.result || '';
    }

    // Start crawl
    const response = await fetch(`${serverUrl}/api/job-definitions/crawl-links`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        start_url: tab.url,
        links: currentLinks,
        job_definition_id: currentCrawlConfig?.job_definition?.id || '',
        cookies: cookies.map(c => ({
          name: c.name,
          value: c.value,
          domain: c.domain,
          path: c.path
        })),
        html: html,
        title: title,
        include_current_page: includeCurrentPage
      })
    });

    if (!response.ok) {
      throw new Error('Failed to start crawl');
    }

    const result = await response.json();

    progressFill.style.width = '100%';
    progressText.textContent = result.message;
    showSuccess(`Crawl started: ${result.links_to_crawl} pages queued`);

    // Reset after delay
    setTimeout(() => {
      progressDiv.style.display = 'none';
      btnText.textContent = 'Start Crawl';
      startBtn.disabled = false;
      isCrawling = false;
    }, 3000);

  } catch (error) {
    console.error('Crawl error:', error);
    showError(`Crawl failed: ${error.message}`);
    progressDiv.style.display = 'none';
    btnText.textContent = 'Start Crawl';
    startBtn.disabled = false;
    isCrawling = false;
  }
}

// Refresh crawl config when tab changes
chrome.tabs.onActivated.addListener(async () => {
  await refreshLinksAndConfig();
});

chrome.tabs.onUpdated.addListener(async (tabId, changeInfo, tab) => {
  if (changeInfo.status === 'complete') {
    await refreshLinksAndConfig();
  }
});
