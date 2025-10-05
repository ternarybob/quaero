// Popup script for Quaero extension

const SERVER_URL = 'http://localhost:8085/api/auth';

document.getElementById('captureBtn').addEventListener('click', async () => {
  const statusDiv = document.getElementById('status');
  const captureBtn = document.getElementById('captureBtn');

  // Disable button during capture
  captureBtn.disabled = true;
  statusDiv.style.display = 'block';
  statusDiv.className = 'info';
  statusDiv.textContent = 'Capturing authentication data...';

  try {
    // Request auth data from background script
    const response = await chrome.runtime.sendMessage({ action: 'captureAuth' });

    if (!response.success) {
      throw new Error(response.error || 'Failed to capture auth data');
    }

    // Send auth data to server
    statusDiv.textContent = 'Sending to Quaero...';

    const serverResponse = await fetch(SERVER_URL, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(response.data)
    });

    if (!serverResponse.ok) {
      throw new Error(`Server error: ${serverResponse.status}`);
    }

    const result = await serverResponse.json();

    // Show success
    statusDiv.className = 'success';
    statusDiv.textContent = `✓ ${result.message || 'Authentication captured successfully!'}`;

    // Auto-hide after 3 seconds
    setTimeout(() => {
      statusDiv.style.display = 'none';
    }, 3000);

  } catch (error) {
    console.error('Error:', error);
    statusDiv.className = 'error';
    statusDiv.textContent = `✗ Error: ${error.message}`;
  } finally {
    captureBtn.disabled = false;
  }
});

// Show initial status
document.addEventListener('DOMContentLoaded', () => {
  const statusDiv = document.getElementById('status');
  statusDiv.style.display = 'block';
  statusDiv.className = 'info';
  statusDiv.textContent = 'Make sure Quaero is running on localhost:8085';

  setTimeout(() => {
    statusDiv.style.display = 'none';
  }, 3000);
});
