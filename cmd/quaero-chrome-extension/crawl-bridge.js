// -----------------------------------------------------------------------
// Quaero Crawl Bridge - Extension Execution Engine
// Exposes window.quaeroStartCrawl() for chromedp control
// Handles page navigation, content extraction, and data transmission
// -----------------------------------------------------------------------

(function() {
  'use strict';

  console.log('Quaero Crawl Bridge loaded');

  // Configuration defaults
  const DEFAULT_PAGE_TIMEOUT = 30000;
  const DEFAULT_WAIT_TIME = 2000;
  const DEFAULT_SERVER_URL = 'http://127.0.0.1:8086';

  // Crawl state
  let crawlState = {
    active: false,
    sessionId: null,
    currentIndex: 0,
    links: [],
    results: [],
    config: {}
  };

  /**
   * Convert all images in a document clone to base64
   * @param {Document} docClone - Cloned document
   * @returns {Promise<void>}
   */
  async function convertImagesToBase64(docClone) {
    const images = docClone.querySelectorAll('img[src]');

    const imagePromises = Array.from(images).map(async (img) => {
      const originalSrc = img.getAttribute('src');

      try {
        if (originalSrc.startsWith('data:')) return;

        const absoluteSrc = new URL(originalSrc, window.location.href).href;

        // Try canvas method first
        const tempImg = document.createElement('img');
        tempImg.crossOrigin = 'anonymous';

        await new Promise((resolve, reject) => {
          tempImg.onload = resolve;
          tempImg.onerror = reject;
          tempImg.src = absoluteSrc;
        });

        const canvas = document.createElement('canvas');
        canvas.width = tempImg.naturalWidth || tempImg.width;
        canvas.height = tempImg.naturalHeight || tempImg.height;

        const ctx = canvas.getContext('2d');
        ctx.drawImage(tempImg, 0, 0);

        const dataUri = canvas.toDataURL('image/png');
        img.setAttribute('src', dataUri);

      } catch (error) {
        // Fallback to fetch with credentials
        try {
          const response = await fetch(new URL(originalSrc, window.location.href).href, {
            credentials: 'include'
          });

          if (response.ok) {
            const blob = await response.blob();
            const dataUri = await new Promise((resolve) => {
              const reader = new FileReader();
              reader.onloadend = () => resolve(reader.result);
              reader.readAsDataURL(blob);
            });
            img.setAttribute('src', dataUri);
          }
        } catch (fetchError) {
          console.warn(`Failed to convert image: ${originalSrc}`);
        }
      }
    });

    await Promise.allSettled(imagePromises);
  }

  /**
   * Extract page content including full HTML
   * @param {boolean} includeImages - Whether to convert images to base64
   * @returns {Promise<Object>} Page content object
   */
  async function extractPageContent(includeImages = true) {
    const startTime = performance.now();

    let html;
    if (includeImages) {
      const docClone = document.cloneNode(true);
      await convertImagesToBase64(docClone);
      html = docClone.documentElement.outerHTML;
    } else {
      html = document.documentElement.outerHTML;
    }

    const renderTime = Math.round(performance.now() - startTime);

    // Extract metadata
    const metadata = {
      title: document.title,
      url: window.location.href,
      description: document.querySelector('meta[name="description"]')?.content || '',
      canonical: document.querySelector('link[rel="canonical"]')?.href || window.location.href,
      language: document.documentElement.lang || 'en',
      timestamp: new Date().toISOString()
    };

    // Extract links if configured
    const links = [];
    document.querySelectorAll('a[href]').forEach(a => {
      const href = a.getAttribute('href');
      if (href && !href.startsWith('javascript:') && !href.startsWith('#') &&
          !href.startsWith('mailto:') && !href.startsWith('tel:')) {
        try {
          const absoluteUrl = new URL(href, window.location.href).href;
          if (!links.includes(absoluteUrl)) {
            links.push(absoluteUrl);
          }
        } catch (e) {
          // Invalid URL, skip
        }
      }
    });

    return {
      url: window.location.href,
      html: html,
      title: metadata.title,
      metadata: metadata,
      links: links,
      renderTime: renderTime,
      contentSize: html.length
    };
  }

  /**
   * Send crawl result to the service API
   * @param {Object} result - Crawl result object
   * @param {string} serverUrl - Server URL
   * @param {string} sessionId - Session ID
   * @returns {Promise<Object>} Server response
   */
  async function sendResultToServer(result, serverUrl, sessionId) {
    const endpoint = `${serverUrl}/api/crawl-data?session_id=${encodeURIComponent(sessionId)}`;

    console.log(`Sending crawl result to ${endpoint}`);

    try {
      const response = await fetch(endpoint, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-Session-ID': sessionId
        },
        body: JSON.stringify(result)
      });

      if (!response.ok) {
        throw new Error(`Server returned ${response.status}: ${await response.text()}`);
      }

      const data = await response.json();
      console.log('Result sent successfully:', data);
      return data;

    } catch (error) {
      console.error('Failed to send result to server:', error);
      throw error;
    }
  }

  /**
   * Navigate to a URL and wait for page load
   * @param {string} url - Target URL
   * @param {number} timeout - Navigation timeout in ms
   * @returns {Promise<void>}
   */
  function navigateWithTimeout(url, timeout) {
    return new Promise((resolve, reject) => {
      const timeoutId = setTimeout(() => {
        reject(new Error(`Navigation timeout after ${timeout}ms`));
      }, timeout);

      // Navigate using location.href for same-origin, or window.open for cross-origin
      try {
        window.location.href = url;
        // Note: This won't actually resolve because page will reload
        // The resolve should happen on the new page load
        setTimeout(() => {
          clearTimeout(timeoutId);
          resolve();
        }, 1000);
      } catch (error) {
        clearTimeout(timeoutId);
        reject(error);
      }
    });
  }

  /**
   * Process a single link in the crawl queue
   * @param {number} index - Link index
   * @returns {Promise<Object>} Crawl result
   */
  async function processSingleLink(index) {
    if (index >= crawlState.links.length) {
      console.log('All links processed');
      return null;
    }

    const link = crawlState.links[index];
    console.log(`Processing link ${index + 1}/${crawlState.links.length}: ${link}`);

    try {
      // If we're already on this URL, extract content directly
      if (window.location.href === link ||
          new URL(link, window.location.href).href === window.location.href) {
        const content = await extractPageContent(true);
        return content;
      }

      // Navigate to the link
      // Note: Navigation will cause page reload, so we need to persist state
      localStorage.setItem('quaero_crawl_state', JSON.stringify({
        ...crawlState,
        currentIndex: index
      }));

      window.location.href = link;
      return null; // Page will reload

    } catch (error) {
      console.error(`Error processing link ${link}:`, error);
      return {
        url: link,
        error: error.message,
        html: '',
        title: '',
        metadata: {},
        links: [],
        renderTime: 0,
        contentSize: 0
      };
    }
  }

  /**
   * Resume crawl from persisted state (after page navigation)
   */
  async function resumeCrawlFromState() {
    const savedState = localStorage.getItem('quaero_crawl_state');
    if (!savedState) return false;

    try {
      const state = JSON.parse(savedState);
      if (!state.active) return false;

      console.log('Resuming crawl from saved state:', state);
      crawlState = state;

      // Extract current page content
      const content = await extractPageContent(true);

      // Send result to server
      await sendResultToServer(content, state.config.serverUrl, state.sessionId);

      // Move to next link
      crawlState.currentIndex++;
      crawlState.results.push(content);

      // Continue with next link or finish
      if (crawlState.currentIndex < crawlState.links.length) {
        // Save state and navigate to next link
        localStorage.setItem('quaero_crawl_state', JSON.stringify(crawlState));

        // Small delay before next navigation
        setTimeout(() => {
          const nextLink = crawlState.links[crawlState.currentIndex];
          console.log(`Navigating to next link: ${nextLink}`);
          window.location.href = nextLink;
        }, crawlState.config.waitTime || DEFAULT_WAIT_TIME);
      } else {
        // Crawl complete
        console.log('Crawl completed!');
        crawlState.active = false;
        localStorage.removeItem('quaero_crawl_state');
      }

      return true;

    } catch (error) {
      console.error('Failed to resume crawl:', error);
      localStorage.removeItem('quaero_crawl_state');
      return false;
    }
  }

  /**
   * Main entry point for starting a crawl session
   * Called by chromedp via window.quaeroStartCrawl(data)
   *
   * @param {Object} data - Crawl configuration
   * @param {string} data.sessionId - Unique session ID
   * @param {string[]} data.links - Array of URLs to crawl
   * @param {string} data.serverUrl - URL of the service API
   * @param {number} data.pageTimeout - Timeout per page in ms
   * @param {number} data.waitTime - Wait time after page load in ms
   * @param {boolean} data.includeLinks - Whether to extract links from pages
   * @returns {Object} Initial status
   */
  window.quaeroStartCrawl = async function(data) {
    console.log('quaeroStartCrawl called with:', data);

    // Validate input
    if (!data || typeof data !== 'object') {
      return { error: 'Invalid crawl data', success: false };
    }

    if (!data.sessionId) {
      return { error: 'Session ID is required', success: false };
    }

    if (!Array.isArray(data.links) || data.links.length === 0) {
      // If no links provided, just extract current page
      console.log('No links provided, extracting current page only');

      try {
        const content = await extractPageContent(true);
        await sendResultToServer(content, data.serverUrl || DEFAULT_SERVER_URL, data.sessionId);
        return { success: true, message: 'Current page extracted and sent', pagesProcessed: 1 };
      } catch (error) {
        return { error: error.message, success: false };
      }
    }

    // Initialize crawl state
    crawlState = {
      active: true,
      sessionId: data.sessionId,
      currentIndex: 0,
      links: data.links,
      results: [],
      config: {
        serverUrl: data.serverUrl || DEFAULT_SERVER_URL,
        pageTimeout: data.pageTimeout || DEFAULT_PAGE_TIMEOUT,
        waitTime: data.waitTime || DEFAULT_WAIT_TIME,
        includeLinks: data.includeLinks !== false
      }
    };

    console.log(`Starting crawl session ${data.sessionId} with ${data.links.length} links`);

    // Check if first link is current page
    const firstLink = crawlState.links[0];
    const currentUrl = window.location.href;

    if (firstLink === currentUrl || new URL(firstLink, currentUrl).href === currentUrl) {
      // Extract current page first
      try {
        const content = await extractPageContent(true);
        await sendResultToServer(content, crawlState.config.serverUrl, crawlState.sessionId);
        crawlState.results.push(content);
        crawlState.currentIndex = 1;

        console.log('Current page extracted, moving to next link');

        // If more links, continue
        if (crawlState.currentIndex < crawlState.links.length) {
          localStorage.setItem('quaero_crawl_state', JSON.stringify(crawlState));

          setTimeout(() => {
            const nextLink = crawlState.links[crawlState.currentIndex];
            console.log(`Navigating to: ${nextLink}`);
            window.location.href = nextLink;
          }, crawlState.config.waitTime);
        }

        return {
          success: true,
          message: 'Crawl started',
          sessionId: data.sessionId,
          totalLinks: data.links.length,
          currentIndex: crawlState.currentIndex
        };

      } catch (error) {
        return { error: error.message, success: false };
      }
    } else {
      // Navigate to first link
      localStorage.setItem('quaero_crawl_state', JSON.stringify(crawlState));

      setTimeout(() => {
        console.log(`Navigating to first link: ${firstLink}`);
        window.location.href = firstLink;
      }, 500);

      return {
        success: true,
        message: 'Crawl initiated, navigating to first link',
        sessionId: data.sessionId,
        totalLinks: data.links.length
      };
    }
  };

  /**
   * Alias for compatibility
   */
  window.startCrawl = window.quaeroStartCrawl;

  /**
   * Get current crawl status
   */
  window.quaeroGetCrawlStatus = function() {
    return {
      active: crawlState.active,
      sessionId: crawlState.sessionId,
      currentIndex: crawlState.currentIndex,
      totalLinks: crawlState.links.length,
      resultsCount: crawlState.results.length
    };
  };

  /**
   * Stop the current crawl
   */
  window.quaeroStopCrawl = function() {
    crawlState.active = false;
    localStorage.removeItem('quaero_crawl_state');
    console.log('Crawl stopped');
    return { success: true, message: 'Crawl stopped' };
  };

  /**
   * Extract current page content on demand
   */
  window.quaeroExtractPage = async function(includeImages = true) {
    return await extractPageContent(includeImages);
  };

  // Auto-resume crawl on page load if there's saved state
  document.addEventListener('DOMContentLoaded', function() {
    setTimeout(() => {
      resumeCrawlFromState();
    }, 1000);
  });

  // Also try immediately if document is already loaded
  if (document.readyState === 'complete' || document.readyState === 'interactive') {
    setTimeout(() => {
      resumeCrawlFromState();
    }, 1000);
  }

  console.log('Quaero Crawl Bridge initialized');
  console.log('Available functions: window.quaeroStartCrawl(data), window.quaeroGetCrawlStatus(), window.quaeroStopCrawl(), window.quaeroExtractPage()');

})();
