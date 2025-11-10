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

// Listen for messages from popup (if needed for advanced features)
chrome.runtime.onMessage.addListener((request, sender, sendResponse) => {
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
