# Quaero Chrome Extension

This Chrome extension captures authentication data from your active Jira/Confluence session and sends it to the Quaero service.

## Installation

1. Build the project using `.\scripts\build.ps1`
2. The extension will be automatically copied to `bin/quaero-chrome-extension`
3. Open Chrome and navigate to `chrome://extensions/`
4. Enable "Developer mode" in the top right
5. Click "Load unpacked"
6. Select the `bin/quaero-chrome-extension` directory

## LLM Setup (Offline Mode)

**Important**: Quaero requires the `llama-server` binary and model files for offline mode functionality.

For complete installation instructions, see the main `README.md` "LLM Setup (Offline Mode)" section.

**Quick Summary:**
- Binary location: `./llama/llama-server.exe` (Windows) or `./llama/llama-server` (Unix), or in system PATH
- Models location: `./models/` directory
- Default server port: 8080 (change in extension settings if customized)

## Usage

1. Start the Quaero service (default: `http://localhost:8080`)
2. Navigate to your Jira or Confluence instance and log in
3. Click the Quaero extension icon in Chrome toolbar
4. The popup will show current status and server connectivity
5. Click "Capture Authentication" to send credentials to Quaero
6. Use the Quaero web UI to manage sources and start crawling

## Features

- **Dropdown Popup Interface**: Compact popup UI with all essential features
- **Authentication Capture**: Extracts cookies and tokens from Atlassian sites
- **Server Status**: Real-time check of Quaero server connection
- **Settings**: Configurable server URL (default: http://localhost:8080, change if you customize the port)
- **Version Display**: Shows both extension and server version
- **Last Capture Tracking**: Displays when authentication was last captured
- **Domain Validation**: Ensures you're on a Jira/Confluence page before capturing

## API Endpoints

The extension uses the following Quaero API endpoints:

- `POST /api/auth` - Capture and store authentication credentials
- `GET /api/version` - Get server version information

## Security

- Authentication data is only sent to localhost (default: `localhost:8080`)
- No data is sent to external servers
- All communication is local to your machine
- Domain validation prevents accidental capture on wrong sites

## Files

- `manifest.json` - Extension manifest (Manifest V3)
- `background.js` - Service worker for background tasks
- `popup.html` - Main popup UI
- `popup.js` - Popup logic and API communication
- `content.js` - Content script (minimal)
- `icons/` - Extension icons

## Removed Features

This version has been simplified from the previous side panel implementation:
- Removed side panel UI (now uses standard popup)
- Removed WebSocket real-time updates (status checked on demand)
- Simplified to focus on core authentication capture functionality
