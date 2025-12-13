// Content script for Quaero extension
// Captures page HTML and metadata when requested

console.log('Quaero content script loaded');

/**
 * Converts all images in the document to base64 data URIs
 * @returns {Promise<string>} HTML string with images converted to base64
 */
async function convertImagesToBase64() {
  // Clone the document to avoid modifying the original
  const docClone = document.cloneNode(true);
  const images = docClone.querySelectorAll('img[src]');

  console.log(`Found ${images.length} images to convert`);

  // Process all images in parallel
  const imagePromises = Array.from(images).map(async (img) => {
    const originalSrc = img.getAttribute('src');

    try {
      // Skip if already a data URI
      if (originalSrc.startsWith('data:')) {
        return;
      }

      // Resolve relative URLs to absolute
      const absoluteSrc = new URL(originalSrc, window.location.href).href;

      // Create a temporary image element in the actual DOM to load the image
      const tempImg = document.createElement('img');
      tempImg.crossOrigin = 'anonymous'; // Try to enable CORS

      // Wait for image to load
      await new Promise((resolve, reject) => {
        tempImg.onload = resolve;
        tempImg.onerror = reject;
        tempImg.src = absoluteSrc;
      });

      // Create canvas and draw image
      const canvas = document.createElement('canvas');
      canvas.width = tempImg.naturalWidth || tempImg.width;
      canvas.height = tempImg.naturalHeight || tempImg.height;

      const ctx = canvas.getContext('2d');
      ctx.drawImage(tempImg, 0, 0);

      // Convert to base64 data URI
      const dataUri = canvas.toDataURL('image/png');

      // Update the cloned image's src
      img.setAttribute('src', dataUri);

      console.log(`Converted image: ${originalSrc.substring(0, 50)}...`);

    } catch (error) {
      // Handle errors gracefully - log warning and skip failed images
      console.warn(`Failed to convert image ${originalSrc}: ${error.message}`);

      // For CORS errors, try fetch with credentials
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
          console.log(`Converted image via fetch: ${originalSrc.substring(0, 50)}...`);
        } else {
          console.warn(`Fetch failed for image: ${originalSrc}, status: ${response.status}`);
        }
      } catch (fetchError) {
        console.warn(`Fetch also failed for image ${originalSrc}: ${fetchError.message}`);
        // Keep original src as fallback
      }
    }
  });

  // Wait for all image conversions to complete
  await Promise.allSettled(imagePromises);

  // Return the modified HTML string
  return docClone.documentElement.outerHTML;
}

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
  } else if (request.action === 'capturePageWithImages') {
    // Handle async image conversion
    (async () => {
      try {
        // Convert images to base64 first
        const html = await convertImagesToBase64();

        // Capture metadata (same as capturePageContent)
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
    })();

    // Return true to indicate async response
    return true;
  }

  // Return true to indicate async response
  return true;
});
