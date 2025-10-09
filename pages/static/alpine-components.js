// Alpine.js components for Quaero
// Provides reactive data components for parser status, auth details, and service logs

document.addEventListener('alpine:init', () => {
  // Parser Status Component
  Alpine.data('parserStatus', () => ({
    items: [],
    loading: true,
    error: null,

    async init() {
      await this.fetchStatus();
      // No polling - only fetch on init, manual refresh, or WebSocket event
      this.subscribeToWebSocket();
    },

    async fetchStatus() {
      try {
        const response = await fetch('/api/status/parser');
        if (!response.ok) throw new Error('Failed to fetch parser status');

        const data = await response.json();
        this.items = this._formatParserData(data);
        this.loading = false;
        this.error = null;
      } catch (err) {
        console.error('[ParserStatus] Error fetching status:', err);
        // Don't clear existing data on fetch failure
        this.error = err.message;
        this.loading = false;
        // Keep existing this.items data
      }
    },

    _formatParserData(data) {
      return [
        {
          component: 'JIRA PROJECTS',
          status: data.jiraProjects?.count || 0,
          lastUpdated: this._formatTime(data.jiraProjects?.lastUpdated),
          details: data.jiraProjects?.details || 'No projects found'
        },
        {
          component: 'JIRA ISSUES',
          status: data.jiraIssues?.count || 0,
          lastUpdated: this._formatTime(data.jiraIssues?.lastUpdated),
          details: data.jiraIssues?.details || 'No issues found'
        },
        {
          component: 'CONFLUENCE SPACES',
          status: data.confluenceSpaces?.count || 0,
          lastUpdated: this._formatTime(data.confluenceSpaces?.lastUpdated),
          details: data.confluenceSpaces?.details || 'No spaces found'
        },
        {
          component: 'CONFLUENCE PAGES',
          status: data.confluencePages?.count || 0,
          lastUpdated: this._formatTime(data.confluencePages?.lastUpdated),
          details: data.confluencePages?.details || 'No pages found'
        }
      ];
    },

    _formatTime(timestamp) {
      if (!timestamp || timestamp === 0) return 'Never';
      const date = new Date(timestamp * 1000);
      return date.toLocaleTimeString('en-US', { hour12: false });
    },

    subscribeToWebSocket() {
      if (typeof WebSocketManager !== 'undefined') {
        WebSocketManager.subscribe('parser', (data) => {
          console.log('[ParserStatus] WebSocket update received:', data);
          this.fetchStatus();
        });
      }
    },

    refresh() {
      this.loading = true;
      this.fetchStatus();
    }
  }));

  // Auth Details Component
  Alpine.data('authDetails', () => ({
    services: [],
    loading: true,
    error: null,

    async init() {
      await this.fetchAuth();
      // No polling - only fetch on init, manual refresh, or WebSocket event
      this.subscribeToWebSocket();
    },

    async fetchAuth() {
      try {
        const response = await fetch('/api/auth/details');
        if (!response.ok) throw new Error('Failed to fetch auth details');

        const data = await response.json();
        this.services = this._formatAuthData(data);
        this.loading = false;
        this.error = null;
      } catch (err) {
        console.error('[AuthDetails] Error fetching auth:', err);
        // Don't clear existing data on fetch failure
        this.error = err.message;
        this.loading = false;
        // Keep existing this.services data
      }
    },

    _formatAuthData(data) {
      // API returns { services: [{name, status, user}] }
      if (data && data.services && Array.isArray(data.services)) {
        return data.services.map(service => ({
          name: service.name,
          authenticated: service.status === 'authenticated',
          user: service.user || '-'
        }));
      }

      // Fallback for old format
      const isAuthenticated = data && data.authenticated;
      const baseURL = data?.baseURL || '-';

      return [
        {
          name: 'Jira',
          authenticated: isAuthenticated,
          user: baseURL
        },
        {
          name: 'Confluence',
          authenticated: isAuthenticated,
          user: baseURL
        }
      ];
    },

    startPolling() {
      setInterval(() => this.fetchAuth(), 30000);
    },

    subscribeToWebSocket() {
      if (typeof WebSocketManager !== 'undefined') {
        WebSocketManager.subscribe('auth', (data) => {
          console.log('[AuthDetails] WebSocket update received:', data);
          this.fetchAuth();
        });
      }
    },

    refresh() {
      this.loading = true;
      this.fetchAuth();
    },

    getStatusClass(authenticated) {
      return authenticated ? 'tag is-success' : 'tag is-warning';
    },

    getStatusText(authenticated) {
      return authenticated ? 'Authenticated' : 'Not Authenticated';
    }
  }));

  // Service Logs Component
  Alpine.data('serviceLogs', () => ({
    logs: [],
    maxLogs: 100,
    autoScroll: true,

    init() {
      this.loadRecentLogs();
      this.subscribeToWebSocket();
    },

    async loadRecentLogs() {
      try {
        const response = await fetch('/api/logs/recent');
        if (!response.ok) return;

        const data = await response.json();
        if (data.logs && Array.isArray(data.logs)) {
          this.logs = data.logs.map(log => this._parseLogEntry(log));
          // Scroll to bottom after loading recent logs
          this.$nextTick(() => {
            const container = this.$refs.logContainer;
            if (container) {
              container.scrollTop = container.scrollHeight;
            }
          });
        }
      } catch (err) {
        console.error('[ServiceLogs] Error loading recent logs:', err);
      }
    },

    subscribeToWebSocket() {
      if (typeof WebSocketManager !== 'undefined') {
        WebSocketManager.subscribe('log', (data) => {
          this.addLog(data);
        });
        console.log('[ServiceLogs] WebSocket subscription established');
      } else {
        console.error('[ServiceLogs] WebSocketManager not loaded');
      }
    },

    addLog(logData) {
      const logEntry = this._parseLogEntry(logData);
      this.logs.push(logEntry);

      if (this.logs.length > this.maxLogs) {
        this.logs = this.logs.slice(-this.maxLogs);
      }

      if (this.autoScroll) {
        this.$nextTick(() => {
          const container = this.$refs.logContainer;
          if (container) {
            container.scrollTop = container.scrollHeight;
            // Force scroll to ensure it happens
            requestAnimationFrame(() => {
              container.scrollTop = container.scrollHeight;
            });
          }
        });
      }
    },

    _parseLogEntry(logData) {
      const timestamp = logData.timestamp || logData.time || new Date().toISOString();
      const level = (logData.level || 'INFO').toUpperCase();
      const message = logData.message || logData.msg || '';

      return {
        timestamp: this._formatLogTime(timestamp),
        level: level,
        levelClass: this._getLevelClass(level),
        message: message
      };
    },

    _formatLogTime(timestamp) {
      // If timestamp is already formatted as HH:MM:SS, return as-is
      if (typeof timestamp === 'string' && /^\d{2}:\d{2}:\d{2}$/.test(timestamp)) {
        return timestamp;
      }

      // Otherwise try to parse as date
      const date = new Date(timestamp);
      if (isNaN(date.getTime())) {
        return timestamp; // Return original if can't parse
      }

      return date.toLocaleTimeString('en-US', {
        hour12: false,
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit'
      });
    },

    _getLevelClass(level) {
      const levelMap = {
        'ERROR': 'log-level-error',
        'WARN': 'log-level-warn',
        'WARNING': 'log-level-warn',
        'INFO': 'log-level-info',
        'DEBUG': 'log-level-debug'
      };
      return levelMap[level] || 'log-level-info';
    },

    clearLogs() {
      this.logs = [];
    },

    refresh() {
      this.loadRecentLogs();
    },

    toggleAutoScroll() {
      this.autoScroll = !this.autoScroll;
      if (this.autoScroll) {
        // When re-enabling auto-scroll, scroll to latest logs immediately
        this.$nextTick(() => {
          const container = this.$refs.logContainer;
          if (container) container.scrollTop = container.scrollHeight;
        });
      }
    }
  }));

  // Snackbar Notification Component
  Alpine.data('snackbar', () => ({
    visible: false,
    message: '',
    type: 'info',
    timeout: null,

    show(message, type = 'info', duration = 3000) {
      this.message = message;
      this.type = type;
      this.visible = true;

      if (this.timeout) clearTimeout(this.timeout);

      this.timeout = setTimeout(() => {
        this.hide();
      }, duration);
    },

    hide() {
      this.visible = false;
      if (this.timeout) {
        clearTimeout(this.timeout);
        this.timeout = null;
      }
    },

    getClass() {
      const typeMap = {
        'success': 'is-success',
        'error': 'is-danger',
        'warning': 'is-warning',
        'info': 'is-info'
      };
      return typeMap[this.type] || 'is-info';
    }
  }));
});

// Global notification function for backwards compatibility
window.showNotification = function(message, type = 'info') {
  const snackbarEl = document.querySelector('[x-data*="snackbar"]');
  if (snackbarEl && snackbarEl._x_dataStack) {
    snackbarEl._x_dataStack[0].show(message, type);
  }
};
