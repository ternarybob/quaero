#!/usr/bin/env node

/**
 * MCP Proxy for Quaero Document Service
 * Bridges stdio-based MCP clients (like LM Studio) to HTTP-based MCP server
 */

const http = require('http');

const QUAERO_URL = process.env.QUAERO_URL || 'http://localhost:8085';
const DEBUG = process.env.DEBUG === 'true';

function log(...args) {
  if (DEBUG) {
    console.error('[Quaero MCP Proxy]', ...args);
  }
}

log('Starting Quaero MCP proxy');
log('Target URL:', QUAERO_URL);

// Set up stdio for MCP protocol
process.stdin.setEncoding('utf8');
process.stdin.resume();

let buffer = '';

process.stdin.on('data', async (chunk) => {
  buffer += chunk;

  // Process complete JSON-RPC messages (one per line)
  const lines = buffer.split('\n');
  buffer = lines.pop() || ''; // Keep incomplete line in buffer

  for (const line of lines) {
    if (!line.trim()) continue;

    try {
      const request = JSON.parse(line);
      log('Received request:', request.method);

      const response = await forwardToQuaero(request);
      log('Sending response:', response.id);

      // Write response as JSON line
      process.stdout.write(JSON.stringify(response) + '\n');
    } catch (err) {
      log('Error processing request:', err.message);

      // Send error response
      const errorResponse = {
        jsonrpc: '2.0',
        id: null,
        error: {
          code: -32603,
          message: 'Internal error: ' + err.message
        }
      };
      process.stdout.write(JSON.stringify(errorResponse) + '\n');
    }
  }
});

process.stdin.on('end', () => {
  log('stdin closed, exiting');
  process.exit(0);
});

async function forwardToQuaero(request) {
  return new Promise((resolve, reject) => {
    const url = new URL(QUAERO_URL);
    const data = JSON.stringify(request);

    const options = {
      hostname: url.hostname,
      port: url.port || 8085,
      path: '/mcp',
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Content-Length': Buffer.byteLength(data)
      },
      timeout: 30000
    };

    log(`Forwarding to ${url.hostname}:${options.port}${options.path}`);

    const req = http.request(options, (res) => {
      let body = '';

      res.on('data', (chunk) => {
        body += chunk;
      });

      res.on('end', () => {
        try {
          const response = JSON.parse(body);
          resolve(response);
        } catch (err) {
          log('Failed to parse response:', body);
          reject(new Error('Invalid JSON response from server'));
        }
      });
    });

    req.on('error', (err) => {
      log('HTTP request error:', err.message);
      reject(err);
    });

    req.on('timeout', () => {
      req.destroy();
      reject(new Error('Request timeout'));
    });

    req.write(data);
    req.end();
  });
}

// Handle shutdown signals
process.on('SIGINT', () => {
  log('Received SIGINT, shutting down');
  process.exit(0);
});

process.on('SIGTERM', () => {
  log('Received SIGTERM, shutting down');
  process.exit(0);
});

// Handle uncaught errors
process.on('uncaughtException', (err) => {
  log('Uncaught exception:', err);
  process.exit(1);
});

process.on('unhandledRejection', (err) => {
  log('Unhandled rejection:', err);
  process.exit(1);
});

log('Proxy ready, waiting for requests on stdin');
