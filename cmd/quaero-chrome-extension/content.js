// Content script for Quaero extension
// Captures page HTML and metadata when requested

console.log('Quaero content script loaded');

// Listen for messages from popup
chrome.runtime.onMessage.addListener((request, sender, sendResponse) => {
  if (request.action === 'capturePageContent') {
    try {
      // Capture full page HTML
      const html = document.documentElement.outerHTML;

      // Capture metadata
      const metadata = {
        title: document.title,
        url: window.location.href,
        description: document.querySelector('meta[name="description"]')?.content || '',
        canonical: document.querySelector('link[rel="canonical"]')?.href || window.location.href,
        language: document.documentElement.lang || 'en',
        timestamp: new Date().toISOString()
      };

      sendResponse({
        success: true,
        html: html,
        metadata: metadata
      });
    } catch (error) {
      sendResponse({
        success: false,
        error: error.message
      });
    }
  }

  // Return true to indicate async response
  return true;
});
