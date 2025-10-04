# Quaero Chrome Extension

This Chrome extension captures authentication data from your active Jira/Confluence session and sends it to the Quaero service.

## Installation

1. Build the project using `.\scripts\build.ps1`
2. Open Chrome and navigate to `chrome://extensions/`
3. Enable "Developer mode" in the top right
4. Click "Load unpacked"
5. Select the `cmd/quaero-chrome-extension` directory

## Usage

1. Start the Quaero service (it runs on `http://localhost:8080`)
2. Navigate to your Jira or Confluence instance and log in
3. Click the Quaero extension icon in Chrome
4. Click "Capture Auth Data"
5. The extension will capture your authentication and send it to the service
6. The service will automatically start scraping

## Features

- **Side Panel Interface**: Modern side panel UI for easy access
- **Authentication Capture**: Extracts cookies and tokens from Atlassian sites
- **Server Status**: Real-time monitoring of Quaero server connection
- **Settings**: Configurable server URL

## Security

- Authentication data is only sent to `localhost:8080`
- No data is sent to external servers
- All communication is local to your machine

## Files

- `manifest.json` - Extension manifest
- `background.js` - Service worker for auth capture
- `sidepanel.html/js` - Main UI interface
- `popup.html/js` - Quick action popup
- `icons/` - Extension icons
