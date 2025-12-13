/**
 * Web frontend entry point
 *
 * This module provides the main Express server for the web frontend,
 * handling HTTP requests and serving static content.
 */

const express = require('express');
const axios = require('axios');
const { validateRequest, formatResponse } = require('./utils');

const app = express();
const PORT = process.env.PORT || 3000;
const BACKEND_URL = process.env.BACKEND_URL || 'http://localhost:8080';

// Middleware
app.use(express.json());
app.use(express.static('public'));

/**
 * Home route - serves the main page
 */
app.get('/', (req, res) => {
  res.send(`
    <html>
      <head><title>Multi-Language Test Project</title></head>
      <body>
        <h1>Welcome to Multi-Language Test Project</h1>
        <p>Frontend server is running on port ${PORT}</p>
        <p>Backend API: ${BACKEND_URL}</p>
      </body>
    </html>
  `);
});

/**
 * Health check endpoint
 */
app.get('/health', (req, res) => {
  res.json({
    status: 'healthy',
    service: 'web-frontend',
    version: '1.0.0',
    timestamp: new Date().toISOString(),
  });
});

/**
 * Proxy endpoint to backend API
 */
app.post('/api/process', async (req, res) => {
  try {
    // Validate incoming request
    if (!validateRequest(req.body)) {
      return res.status(400).json({ error: 'Invalid request format' });
    }

    // Forward to backend
    const response = await axios.post(`${BACKEND_URL}/api/process`, req.body);

    // Format and return response
    const formatted = formatResponse(response.data);
    res.json(formatted);
  } catch (error) {
    console.error('Error processing request:', error.message);
    res.status(500).json({ error: 'Internal server error' });
  }
});

/**
 * Get backend health status
 */
app.get('/api/backend-health', async (req, res) => {
  try {
    const response = await axios.get(`${BACKEND_URL}/api/health`);
    res.json({
      backend: response.data,
      frontend: { status: 'healthy' },
    });
  } catch (error) {
    res.status(503).json({
      backend: { status: 'unavailable', error: error.message },
      frontend: { status: 'healthy' },
    });
  }
});

// Start server
app.listen(PORT, () => {
  console.log(`Web frontend listening on port ${PORT}`);
  console.log(`Backend API configured at ${BACKEND_URL}`);
});

module.exports = app;
