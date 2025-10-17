// Alpine.js components for Quaero
// Provides reactive data components for parser status, auth details, and service logs

document.addEventListener('alpine:init', () => {
  // Service Logs Component
  Alpine.data('serviceLogs', () => ({
    logs: [],
    maxLogs: 200,
    autoScroll: true,
    logIdCounter: 0,

    init() {
      console.log('[ServiceLogs] Initializing component');
      this.loadRecentLogs();
      this.subscribeToWebSocket();
    },

    async loadRecentLogs() {
      console.log('[ServiceLogs] Loading recent logs...');
      try {
        const response = await fetch('/api/logs/recent');
        console.log('[ServiceLogs] API response status:', response.status);
        if (!response.ok) {
          console.warn('[ServiceLogs] API returned non-OK status:', response.status);
          return;
        }

        const data = await response.json();
        console.log('[ServiceLogs] Received data:', data);
        if (data.logs && Array.isArray(data.logs)) {
          console.log('[ServiceLogs] Processing', data.logs.length, 'log entries');
          this.logs = data.logs.map(log => {
            const entry = this._parseLogEntry(log);
            entry.id = ++this.logIdCounter;
            return entry;
          });
          console.log('[ServiceLogs] Logs array now contains', this.logs.length, 'entries');
          // Scroll to bottom after loading recent logs
          this.$nextTick(() => {
            const container = this.$refs.logContainer;
            if (container) {
              container.scrollTop = container.scrollHeight;
            }
          });
        } else {
          console.warn('[ServiceLogs] No logs in response or invalid format');
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
      logEntry.id = ++this.logIdCounter;
      this.logs.push(logEntry);

      // Use splice to remove from beginning - more memory efficient than slice
      if (this.logs.length > this.maxLogs) {
        const removeCount = this.logs.length - this.maxLogs;
        this.logs.splice(0, removeCount);
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
        'ERROR': 'terminal-error',
        'WARN': 'terminal-warning',
        'WARNING': 'terminal-warning',
        'INFO': 'terminal-info',
        'DEBUG': 'terminal-time'
      };
      return levelMap[level] || 'terminal-info';
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

  // Application Status Component
  Alpine.data('appStatus', () => ({
    state: 'Idle',
    metadata: {},
    timestamp: null,

    init() {
      this.fetchStatus();
      this.subscribeToWebSocket();
    },

    async fetchStatus() {
      try {
        const response = await fetch('/api/status');
        if (!response.ok) throw new Error('Failed to fetch status');

        const data = await response.json();
        this.state = data.state || 'Idle';
        this.metadata = data.metadata || {};
        this.timestamp = data.timestamp ? new Date(data.timestamp) : new Date();
      } catch (err) {
        console.error('[AppStatus] Error fetching status:', err);
        this.state = 'Unknown';
      }
    },

    subscribeToWebSocket() {
      if (typeof WebSocketManager !== 'undefined') {
        WebSocketManager.subscribe('app_status', (data) => {
          console.log('[AppStatus] WebSocket update received:', data);
          this.state = data.state || 'Idle';
          this.metadata = data.metadata || {};
          this.timestamp = data.timestamp ? new Date(data.timestamp) : new Date();
        });
        console.log('[AppStatus] WebSocket subscription established');
      }
    },

    getStatusColor(state) {
      const colorMap = {
        'Idle': 'label-primary',
        'Crawling': 'label-warning',
        'Offline': 'label-error',
        'Unknown': 'label'
      };
      return colorMap[state] || 'label';
    },

    formatTimestamp(timestamp) {
      if (!timestamp) return 'Never';
      const date = new Date(timestamp);
      return date.toLocaleString();
    }
  }));

  // Source Management Component
  Alpine.data('sourceManagement', () => ({
    sources: [],
    authentications: [],
    currentSource: null,
    showCreateModal: false,
    showEditModal: false,
    loading: true,
    modalTriggerElement: null,

    init() {
      this.loadSources();
      this.loadAuthentications();
      this.resetCurrentSource();
    },

    async loadSources() {
      try {
        const response = await fetch('/api/sources');
        if (!response.ok) throw new Error('Failed to fetch sources');

        const data = await response.json();
        this.sources = Array.isArray(data) ? data : [];
        this.loading = false;
      } catch (err) {
        console.error('[SourceManagement] Error loading sources:', err);
        this.loading = false;
        window.showNotification('Failed to load sources: ' + err.message, 'error');
      }
    },

    async loadAuthentications() {
      try {
        const response = await fetch('/api/auth/list');
        if (!response.ok) throw new Error('Failed to fetch authentications');
        const data = await response.json();
        this.authentications = Array.isArray(data) ? data : [];
      } catch (err) {
        console.error('[SourceManagement] Error loading authentications:', err);
        this.authentications = [];
      }
    },

    resetCurrentSource() {
      this.currentSource = {
        name: '',
        type: 'jira',
        base_url: '',
        auth_id: '',
        auth_domain: '',
        enabled: true,
        crawl_config: {
          max_depth: 3,
          follow_links: true,
          concurrency: 2,
          detail_level: 'full'
        },
        filters: {}
      };
    },

    editSource(source, event) {
      this.modalTriggerElement = event?.target || document.activeElement;
      this.currentSource = JSON.parse(JSON.stringify(source));
      this.showEditModal = true;
      document.body.classList.add('modal-open');
      // Reload authentications in case they changed
      this.loadAuthentications();

      // Move focus to modal after it renders
      this.$nextTick(() => {
        const modal = document.querySelector('.modal.active .modal-container');
        if (modal) {
          const firstFocusable = modal.querySelector('input, select, textarea, button');
          if (firstFocusable) firstFocusable.focus();
        }
      });
    },

    openCreateModal(event) {
      this.modalTriggerElement = event?.target || document.activeElement;
      this.resetCurrentSource();
      this.showCreateModal = true;
      document.body.classList.add('modal-open');
      // Load authentications when opening modal
      this.loadAuthentications();

      // Move focus to modal after it renders
      this.$nextTick(() => {
        const modal = document.querySelector('.modal.active .modal-container');
        if (modal) {
          const firstFocusable = modal.querySelector('input, select, textarea, button');
          if (firstFocusable) firstFocusable.focus();
        }
      });
    },

    async saveSource() {
      try {
        const isEdit = this.showEditModal;
        const url = isEdit ? `/api/sources/${this.currentSource.id}` : '/api/sources';
        const method = isEdit ? 'PUT' : 'POST';

        const response = await fetch(url, {
          method: method,
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(this.currentSource)
        });

        if (!response.ok) {
          const error = await response.json();
          throw new Error(error.error || 'Failed to save source');
        }

        window.showNotification(`Source ${isEdit ? 'updated' : 'created'} successfully`, 'success');
        await this.loadSources();
        this.closeModal();
      } catch (err) {
        console.error('[SourceManagement] Error saving source:', err);
        window.showNotification('Failed to save source: ' + err.message, 'error');
      }
    },

    async deleteSource(sourceId) {
      if (!confirm('Are you sure you want to delete this source? This will also remove all associated data.')) {
        return;
      }

      try {
        const response = await fetch(`/api/sources/${sourceId}`, {
          method: 'DELETE'
        });

        if (!response.ok) {
          throw new Error('Failed to delete source');
        }

        window.showNotification('Source deleted successfully', 'success');
        await this.loadSources();
      } catch (err) {
        console.error('[SourceManagement] Error deleting source:', err);
        window.showNotification('Failed to delete source: ' + err.message, 'error');
      }
    },

    closeModal() {
      this.showCreateModal = false;
      this.showEditModal = false;
      document.body.classList.remove('modal-open');
      this.resetCurrentSource();

      // Restore focus to trigger element
      if (this.modalTriggerElement) {
        this.$nextTick(() => {
          this.modalTriggerElement.focus();
          this.modalTriggerElement = null;
        });
      }
    },

    formatDate(dateStr) {
      if (!dateStr) return 'N/A';
      const date = new Date(dateStr);
      return date.toLocaleString();
    }
  }));
});

// Global notification function using custom toast system
window.showNotification = function(message, type = 'info') {
  // Type to class mapping
  const typeMap = {
    'info': 'toast-info',
    'success': 'toast-success',
    'warning': 'toast-warning',
    'error': 'toast-error',
    'danger': 'toast-error'
  };
  const toastClass = typeMap[type] || 'toast-info';

  try {
    // Get or create toast container
    let container = document.getElementById('toast-container');
    if (!container) {
      container = document.createElement('div');
      container.id = 'toast-container';
      container.className = 'toast-container';
      container.setAttribute('aria-live', 'polite');
      container.setAttribute('aria-atomic', 'false');
      document.body.appendChild(container);
    }

    // Create toast element
    const toast = document.createElement('div');
    toast.className = 'toast-item ' + toastClass;

    // Set ARIA role and aria-live based on type
    const isError = type === 'error' || type === 'danger';
    toast.setAttribute('role', isError ? 'alert' : 'status');
    toast.setAttribute('aria-live', isError ? 'assertive' : 'polite');
    toast.setAttribute('aria-atomic', 'true');

    // Add icon based on type
    const icons = {
      'success': 'fa-check-circle',
      'error': 'fa-exclamation-circle',
      'warning': 'fa-exclamation-triangle',
      'info': 'fa-info-circle'
    };
    const iconClass = icons[type] || icons['info'];

    toast.innerHTML = `
      <i class="fas ${iconClass}" style="margin-right: 0.5rem;"></i>
      <span>${message}</span>
      <button class="toast-close-btn" aria-label="Close notification" title="Close">
        <i class="fas fa-times"></i>
      </button>
    `;

    // Add close button event listener
    const closeBtn = toast.querySelector('.toast-close-btn');
    closeBtn.addEventListener('click', () => {
      toast.classList.add('toast-removing');
      setTimeout(() => {
        if (toast.parentNode) {
          toast.remove();
        }
      }, 300);
    });

    // Append to container
    container.appendChild(toast);

    // Limit to 5 toasts
    const toasts = container.querySelectorAll('.toast-item');
    if (toasts.length > 5) {
      toasts[0].remove();
    }

    // Auto-dismiss after 3000ms
    setTimeout(() => {
      toast.classList.add('toast-removing');
      setTimeout(() => {
        if (toast.parentNode) {
          toast.remove();
        }
      }, 300); // Allow animation to complete
    }, 3000);
  } catch (error) {
    // Fallback to console if DOM manipulation fails
    console.warn('Toast notification failed, falling back to console');
    console.log(`[${type.toUpperCase()}] ${message}`);
  }
};
