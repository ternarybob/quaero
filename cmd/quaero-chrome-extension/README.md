# Quaero Chrome Extension

This Chrome extension captures authentication data from your active Jira/Confluence session and sends it to the Quaero service.

## Installation

1. Build the project using `.\scripts\build.ps1`
2. The extension will be automatically copied to `bin/quaero-chrome-extension`
3. Open Chrome and navigate to `chrome://extensions/`
4. Enable "Developer mode" in the top right
5. Click "Load unpacked"
6. Select the `bin/quaero-chrome-extension` directory

## Usage

1. Start the Quaero service (default: `http://localhost:8085`)
2. Navigate to your Jira or Confluence instance and log in
3. Click the Quaero extension icon in Chrome
4. Click "Capture Authentication"
5. The extension will capture your authentication and send it to the service
6. Use the Quaero web UI to manage sources and start crawling

## Features

- **Side Panel Interface**: Modern side panel UI for easy access
- **Authentication Capture**: Extracts cookies and tokens from Atlassian sites
- **WebSocket Status**: Real-time monitoring of Quaero server connection
- **Settings**: Configurable server URL (default: http://localhost:8085)
- **Version Display**: Shows both extension and server version

## API Endpoints

The extension uses the following Quaero API endpoints:

- `POST /api/auth` - Capture and store authentication credentials
- `GET /api/auth/status` - Check authentication status
- `GET /api/version` - Get server version information
- `GET /api/status` - Get application status
- `WS /ws` - WebSocket connection for real-time updates

## Security

- Authentication data is only sent to localhost (default: `localhost:8085`)
- No data is sent to external servers
- All communication is local to your machine
- Uses secure WebSocket (WSS) for HTTPS connections

## Files

- `manifest.json` - Extension manifest
- `background.js` - Service worker for auth capture
- `sidepanel.html/js` - Main UI interface
- `popup.html/js` - Quick action popup
- `icons/` - Extension icons
