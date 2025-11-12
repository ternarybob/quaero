# Quaero Web Crawler Extension

This Chrome extension captures authentication data and instantly starts crawling any website with Quaero. The extension works generically with any website - authenticated or public - making it easy to index documentation, wikis, issue trackers, and more.

## Installation

1. Build the project using `.\scripts\build.ps1`
2. The extension will be automatically copied to `bin/quaero-chrome-extension`
3. Open Chrome and navigate to `chrome://extensions/`
4. Enable "Developer mode" in the top right
5. Click "Load unpacked"
6. Select the `bin/quaero-chrome-extension` directory

## LLM Setup (Google ADK)

**Important**: Quaero requires a Google Gemini API key for LLM functionality (embeddings and chat).

For complete installation instructions, see the main `README.md` "LLM Setup (Google ADK)" section.

**Quick Summary:**
- API key from: https://aistudio.google.com/app/apikey
- No local model files or binaries required
- Default server port: 8085 (change in extension settings if customized)

## Usage

1. Start the Quaero service (default: `http://localhost:8085`)
2. Navigate to any website you want to crawl (examples: documentation sites, wikis, Jira, Confluence, GitHub)
3. If the site requires authentication, log in normally (handles 2FA, SSO, etc.)
4. Click the Quaero extension icon in Chrome toolbar
5. The side panel will show current status and server connectivity
6. Click "Capture & Crawl" to capture authentication and immediately start crawling
7. The extension will:
   - Capture authentication cookies and tokens from the current site
   - Send authentication data to Quaero
   - Automatically create and execute a crawler job starting from the current page
   - Display a success message with the job ID
8. Monitor crawl progress in the Quaero web UI at `http://localhost:8085`

## Features

- **Capture & Crawl**: One-click authentication capture and instant crawl job creation
- **Side Panel UI**: Clean, persistent side panel interface with real-time status
- **Generic Website Support**: Works with any website - public or authenticated
- **Examples**: Documentation sites, wikis, Jira, Confluence, GitHub, knowledge bases
- **Automatic Job Creation**: Creates and executes crawler job in one step
- **Real-time Status**: WebSocket connection shows server status and last capture time
- **Configurable Server URL**: Default `http://localhost:8085`, customizable in settings
- **Version Display**: Shows both extension and server version
- **Last Capture Tracking**: Displays when authentication was last captured
- **Job Monitoring**: Check progress in Quaero web UI after starting crawl

## API Endpoints

The extension uses the following Quaero API endpoints:

- `POST /api/auth` - Capture and store authentication credentials
- `POST /api/job-definitions/quick-crawl` - Create and execute crawler job
- `GET /api/version` - Get server version information
- `WS /ws` - WebSocket connection for real-time status updates

## Security

- Authentication data is only sent to localhost (default: `localhost:8085`)
- No data is sent to external servers
- All communication is local to your machine
- Generic capture works with any website - you control which sites to crawl
- Crawler jobs are executed locally by Quaero server
- All crawled content stays on your machine

## Files

- `manifest.json` - Extension manifest (Manifest V3)
- `background.js` - Service worker for background tasks
- `popup.html` - Extension action popup (triggers side panel)
- `sidepanel.html` - Main side panel UI with capture and status
- `sidepanel.js` - Side panel logic, API communication, and WebSocket connection
- `content.js` - Content script (minimal)
- `icons/` - Extension icons

## Implementation Details

**Capture & Crawl Workflow:**
1. User clicks "Capture & Crawl" button in side panel
2. Extension captures cookies from current tab using Chrome Cookies API
3. Extension extracts authentication tokens from cookies (generic pattern matching)
4. Extension sends auth data to `POST /api/auth`
5. Extension immediately creates quick-crawl job via `POST /api/job-definitions/quick-crawl`
6. Quick-crawl job includes current page URL and captured cookies
7. Quaero server creates job definition and executes crawler immediately
8. Extension displays success message with job ID
9. User monitors progress in Quaero web UI

**Key Design Decisions:**
- Generic approach works with any website (not platform-specific)
- One-click workflow reduces friction for users
- Side panel provides persistent UI without disrupting workflow
- WebSocket connection for real-time server status
- Quick-crawl creates sensible defaults (depth: 3, max pages: 100)
