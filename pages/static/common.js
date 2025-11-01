// Alpine.js components for Quaero
// Provides reactive data components for parser status, auth details, and service logs

// Global debug flag - read from server config (injected by template)
// Can be overridden in browser console: window.QUAERO_DEBUG = false
window.QUAERO_DEBUG = typeof window.QUAERO_CLIENT_DEBUG !== 'undefined' ? window.QUAERO_CLIENT_DEBUG : false;

// Debug logger helper
window.debugLog = function(component, message, ...args) {
  if (window.QUAERO_DEBUG) {
    const timestamp = new Date().toISOString().split('T')[1].split('.')[0];
    console.log(`[${timestamp}] [${component}]`, message, ...args);
  }
};

window.debugError = function(component, message, error) {
  const timestamp = new Date().toISOString().split('T')[1].split('.')[0];
  console.error(`[${timestamp}] [${component}]`, message, error);
  if (error && error.stack) {
    console.error(`[${timestamp}] [${component}] Stack:`, error.stack);
  }
};

document.addEventListener('alpine:init', () => {
  window.debugLog('Common', 'Alpine.js init event started');
  // Service Logs Component
  Alpine.data('serviceLogs', () => ({
    logs: [],
    maxLogs: 200,
    autoScroll: true,
    logIdCounter: 0,

    // Architecture Note: Log Filtering
    // - Server filters logs before broadcasting (WebSocketWriter with min_level and exclude_patterns)
    // - Client displays all received logs without filtering
    // - This maintains clean separation: server controls filtering, client is display layer
    // - See: internal/handlers/websocket_writer.go for server-side filtering logic

    init() {
      window.debugLog('ServiceLogs', 'Initializing component');
      this.loadRecentLogs();
      this.subscribeToWebSocket();
    },

    async loadRecentLogs() {
      window.debugLog('ServiceLogs', 'Loading recent logs...');
      try {
        const response = await fetch('/api/logs/recent');
        window.debugLog('ServiceLogs', 'API response status:', response.status);
        if (!response.ok) {
          window.debugLog('ServiceLogs', 'API returned non-OK status:', response.status);
          return;
        }

        const data = await response.json();
        window.debugLog('ServiceLogs', 'Received data:', data);
        if (data.logs && Array.isArray(data.logs)) {
          window.debugLog('ServiceLogs', 'Processing', data.logs.length, 'log entries');
          this.logs = data.logs.map(log => {
            const entry = this._parseLogEntry(log);
            entry.id = ++this.logIdCounter;
            return entry;
          });
          window.debugLog('ServiceLogs', 'Logs array now contains', this.logs.length, 'entries');
          // Scroll to bottom after loading recent logs
          this.$nextTick(() => {
            const container = this.$refs.logContainer;
            if (container) {
              container.scrollTop = container.scrollHeight;
            }
          });
        } else {
          window.debugLog('ServiceLogs', 'No logs in response or invalid format');
        }
      } catch (err) {
        window.debugError('ServiceLogs', 'Error loading recent logs:', err);
      }
    },

    subscribeToWebSocket() {
      if (typeof WebSocketManager !== 'undefined') {
        WebSocketManager.subscribe('log', (data) => {
          this.addLog(data);
        });
        window.debugLog('ServiceLogs', 'WebSocket subscription established');
      } else {
        window.debugError('ServiceLogs', 'WebSocketManager not loaded', new Error('WebSocketManager undefined'));
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
      window.debugLog('AppStatus', 'Initializing component');
      this.fetchStatus();
      this.subscribeToWebSocket();
    },

    async fetchStatus() {
      window.debugLog('AppStatus', 'Fetching status from /api/status');
      try {
        const response = await fetch('/api/status');
        window.debugLog('AppStatus', 'Response status:', response.status);
        if (!response.ok) throw new Error('Failed to fetch status');

        const data = await response.json();
        window.debugLog('AppStatus', 'Status data received:', data);
        this.state = data.state || 'Idle';
        this.metadata = data.metadata || {};
        this.timestamp = data.timestamp ? new Date(data.timestamp) : new Date();
      } catch (err) {
        window.debugError('AppStatus', 'Error fetching status:', err);
        this.state = 'Unknown';
      }
    },

    subscribeToWebSocket() {
      if (typeof WebSocketManager !== 'undefined') {
        WebSocketManager.subscribe('app_status', (data) => {
          window.debugLog('AppStatus', 'WebSocket update received:', data);
          this.state = data.state || 'Idle';
          this.metadata = data.metadata || {};
          this.timestamp = data.timestamp ? new Date(data.timestamp) : new Date();
        });
        window.debugLog('AppStatus', 'WebSocket subscription established');
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

  // Job Type Utility Functions
  // These functions provide consistent styling and display for job types across the UI
  function getJobTypeBadgeClass(jobType) {
    const mapping = {
      'pre_validation': 'label-warning',   // Orange
      'crawler_url': 'label-info',         // Blue
      'post_summary': 'label-primary',     // Purple
      'parent': 'label-success',           // Green
    };
    return mapping[jobType] || 'label';  // Gray default
  }

  function getJobTypeIcon(jobType) {
    const mapping = {
      'pre_validation': 'fa-check-circle',
      'crawler_url': 'fa-link',
      'post_summary': 'fa-file-alt',
      'parent': 'fa-folder',
    };
    return mapping[jobType] || 'fa-question-circle';  // Default
  }

  function getJobTypeDisplayName(jobType) {
    const mapping = {
      'pre_validation': 'Pre-Validation',
      'crawler_url': 'URL Crawl',
      'post_summary': 'Post-Summary',
      'parent': 'Parent Job',
    };
    return mapping[jobType] || 'Unknown Type';
  }

  // Export to window for use in queue.html Alpine.js components
  window.getJobTypeBadgeClass = getJobTypeBadgeClass;
  window.getJobTypeIcon = getJobTypeIcon;
  window.getJobTypeDisplayName = getJobTypeDisplayName;

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
      window.debugLog('SourceManagement', 'Initializing component');
      this.loadSources();
      this.loadAuthentications();
      this.resetCurrentSource();
    },

    async loadSources() {
      window.debugLog('SourceManagement', 'Loading sources from /api/sources');
      try {
        const response = await fetch('/api/sources');
        window.debugLog('SourceManagement', 'Response status:', response.status);
        if (!response.ok) throw new Error('Failed to fetch sources');

        const data = await response.json();
        window.debugLog('SourceManagement', 'Sources data received:', data);
        this.sources = Array.isArray(data) ? data : [];
        window.debugLog('SourceManagement', 'Sources array:', this.sources, 'Count:', this.sources.length);
        this.loading = false;
      } catch (err) {
        window.debugError('SourceManagement', 'Error loading sources:', err);
        this.loading = false;
        window.showNotification('Failed to load sources: ' + err.message, 'error');
      }
    },

    async loadAuthentications() {
      window.debugLog('SourceManagement', 'Loading authentications from /api/auth/list');
      try {
        const response = await fetch('/api/auth/list');
        window.debugLog('SourceManagement', 'Auth response status:', response.status);
        if (!response.ok) throw new Error('Failed to fetch authentications');
        const data = await response.json();
        window.debugLog('SourceManagement', 'Authentications received:', data);
        this.authentications = Array.isArray(data) ? data : [];
      } catch (err) {
        window.debugError('SourceManagement', 'Error loading authentications:', err);
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
          concurrency: 2
        },
        filters: {
          include_patterns: '',
          exclude_patterns: ''
        }
      };
    },

    editSource(source, event) {
      this.modalTriggerElement = event?.target || document.activeElement;
      this.currentSource = JSON.parse(JSON.stringify(source));

      // Ensure filters field exists and contains strings (not objects)
      if (!this.currentSource.filters) {
        this.currentSource.filters = { include_patterns: '', exclude_patterns: '' };
      } else {
        // Convert filter object to strings if needed
        if (typeof this.currentSource.filters.include_patterns !== 'string') {
          this.currentSource.filters.include_patterns = '';
        }
        if (typeof this.currentSource.filters.exclude_patterns !== 'string') {
          this.currentSource.filters.exclude_patterns = '';
        }
      }

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
      window.debugLog('SourceManagement', 'Saving source:', this.currentSource);
      try {
        const isEdit = this.showEditModal;
        const url = isEdit ? `/api/sources/${this.currentSource.id}` : '/api/sources';
        const method = isEdit ? 'PUT' : 'POST';

        // Process filters before sending to backend
        const sourceToSave = JSON.parse(JSON.stringify(this.currentSource));

        // Ensure filters are properly formatted (empty strings become null for backend)
        if (sourceToSave.filters) {
          if (!sourceToSave.filters.include_patterns || sourceToSave.filters.include_patterns.trim() === '') {
            sourceToSave.filters.include_patterns = '';
          }
          if (!sourceToSave.filters.exclude_patterns || sourceToSave.filters.exclude_patterns.trim() === '') {
            sourceToSave.filters.exclude_patterns = '';
          }
        }

        window.debugLog('SourceManagement', `${method} ${url}`, 'Processed source:', sourceToSave);
        const response = await fetch(url, {
          method: method,
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(sourceToSave)
        });

        window.debugLog('SourceManagement', 'Save response status:', response.status);
        if (!response.ok) {
          const error = await response.json();
          throw new Error(error.error || 'Failed to save source');
        }

        window.showNotification(`Source ${isEdit ? 'updated' : 'created'} successfully`, 'success');
        await this.loadSources();
        this.closeModal();
      } catch (err) {
        window.debugError('SourceManagement', 'Error saving source:', err);
        window.showNotification('Failed to save source: ' + err.message, 'error');
      }
    },

    async deleteSource(sourceId) {
      if (!confirm('Are you sure you want to delete this source? This will also remove all associated data.')) {
        return;
      }

      window.debugLog('SourceManagement', 'Deleting source:', sourceId);
      try {
        const response = await fetch(`/api/sources/${sourceId}`, {
          method: 'DELETE'
        });

        window.debugLog('SourceManagement', 'Delete response status:', response.status);
        if (!response.ok) {
          throw new Error('Failed to delete source');
        }

        window.showNotification('Source deleted successfully', 'success');
        await this.loadSources();
      } catch (err) {
        window.debugError('SourceManagement', 'Error deleting source:', err);
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
    },

    formatFilterDisplay(filters) {
      if (!filters) return 'None';

      const parts = [];
      if (filters.include_patterns && filters.include_patterns.trim()) {
        const count = filters.include_patterns.split(',').filter(p => p.trim()).length;
        parts.push(`Include: ${count}`);
      }
      if (filters.exclude_patterns && filters.exclude_patterns.trim()) {
        const count = filters.exclude_patterns.split(',').filter(p => p.trim()).length;
        parts.push(`Exclude: ${count}`);
      }

      return parts.length > 0 ? parts.join(', ') : 'None';
    }
  }));

  // Job Definitions Management Component
  Alpine.data('jobDefinitionsManagement', () => ({
    jobDefinitions: [],
    sources: [],
    currentJobDefinition: null,
    showCreateModal: false,
    showEditModal: false,
    loading: true,
    modalTriggerElement: null,

    init() {
      window.debugLog('JobDefinitionsManagement', 'Initializing component');
      this.loadJobDefinitions();
      this.loadSources();
      this.resetCurrentJobDefinition();
    },

    async loadJobDefinitions() {
      window.debugLog('JobDefinitionsManagement', 'Loading job definitions from /api/job-definitions');
      try {
        const response = await fetch('/api/job-definitions');
        window.debugLog('JobDefinitionsManagement', 'Response status:', response.status);
        if (!response.ok) throw new Error('Failed to fetch job definitions');

        const data = await response.json();
        window.debugLog('JobDefinitionsManagement', 'Job definitions received:', data);

        // API returns { job_definitions: [...], total_count: N }
        if (data && data.job_definitions) {
          this.jobDefinitions = Array.isArray(data.job_definitions) ? data.job_definitions : [];
        } else if (Array.isArray(data)) {
          // Fallback for direct array response
          this.jobDefinitions = data;
        } else {
          this.jobDefinitions = [];
        }

        // Backward compatibility: ensure all job definitions have post_jobs field
        this.jobDefinitions = this.jobDefinitions.map(jd => ({ ...jd, post_jobs: jd.post_jobs || [] }));

        this.loading = false;
      } catch (err) {
        window.debugError('JobDefinitionsManagement', 'Error loading job definitions:', err);
        this.loading = false;
        window.showNotification('Failed to load job definitions: ' + err.message, 'error');
      }
    },

    async loadSources() {
      window.debugLog('JobDefinitionsManagement', 'Loading sources from /api/sources');
      try {
        const response = await fetch('/api/sources');
        if (!response.ok) throw new Error('Failed to fetch sources');
        const data = await response.json();
        this.sources = Array.isArray(data) ? data : [];
      } catch (err) {
        window.debugError('JobDefinitionsManagement', 'Error loading sources:', err);
        this.sources = [];
      }
    },

    resetCurrentJobDefinition() {
      this.currentJobDefinition = {
        id: this.generateID(),
        name: '',
        type: 'crawler',
        description: '',
        sources: [],
        steps: this.getDefaultSteps('crawler'),
        schedule: '',  // Optional: empty for on-demand jobs
        timeout: '',   // Optional: duration string like "10m", "1h", "30s"
        enabled: true,
        auto_start: false,
        config: {},
        post_jobs: []
      };
    },

    get availablePostJobs() {
      // Filter out the current job being edited to prevent self-reference
      return this.jobDefinitions.filter(jobDef => jobDef.id !== this.currentJobDefinition.id);
    },

    generateID() {
      // Generate a random ID (UUID-like format)
      return 'job_' + Date.now() + '_' + Math.random().toString(36).substr(2, 9);
    },

    getDefaultSteps(jobType) {
      // Return default steps based on job type
      // Actions must match registered actions in backend:
      // - crawler: crawl, transform, embed
      // - summarizer: scan, summarize, extract_keywords
      switch (jobType) {
        case 'crawler':
          return [
            {
              name: 'crawl_sources',
              action: 'crawl',
              config: {
                wait_for_completion: true  // CRITICAL: Wait for crawl to finish before transform
              },
              on_error: 'fail'
            },
            {
              name: 'transform_to_documents',
              action: 'transform',
              config: {},
              on_error: 'fail'
            },
            {
              name: 'generate_embeddings',
              action: 'embed',
              config: {},
              on_error: 'continue'
            }
          ];
        case 'summarizer':
          return [
            {
              name: 'scan_documents',
              action: 'scan',
              config: {},
              on_error: 'fail'
            },
            {
              name: 'summarize_content',
              action: 'summarize',
              config: {},
              on_error: 'continue'
            }
          ];
        case 'custom':
          return [
            {
              name: 'custom_step',
              action: 'crawl',
              config: {},
              on_error: 'fail'
            }
          ];
        default:
          return [];
      }
    },

    updateJobTypeSteps(jobType) {
      // Update steps when job type changes
      this.currentJobDefinition.steps = this.getDefaultSteps(jobType);
    },

    async openCreateModal(event) {
      this.modalTriggerElement = event?.target || document.activeElement;
      this.resetCurrentJobDefinition();
      this.showCreateModal = true;
      document.body.classList.add('modal-open');
      await this.loadJobDefinitions();
      this.loadSources();

      this.$nextTick(() => {
        const modal = document.querySelector('.modal.active .modal-container');
        if (modal) {
          const firstFocusable = modal.querySelector('input, select, textarea, button');
          if (firstFocusable) firstFocusable.focus();
        }
      });
    },

    async editJobDefinition(jobDef, event) {
      this.modalTriggerElement = event?.target || document.activeElement;
      this.currentJobDefinition = JSON.parse(JSON.stringify(jobDef));
      // Defensive initialization for backward compatibility with old job definitions
      if (!this.currentJobDefinition.post_jobs) {
        this.currentJobDefinition.post_jobs = [];
      }
      this.showEditModal = true;
      document.body.classList.add('modal-open');
      await this.loadJobDefinitions();
      this.loadSources();

      this.$nextTick(() => {
        const modal = document.querySelector('.modal.active .modal-container');
        if (modal) {
          const firstFocusable = modal.querySelector('input, select, textarea, button');
          if (firstFocusable) firstFocusable.focus();
        }
      });
    },

    detectPostJobCycle() {
      // Build directed graph from all job definitions with the pending post_jobs applied
      const graph = new Map();

      // Add all existing job definitions to the graph
      for (const jobDef of this.jobDefinitions) {
        const postJobs = jobDef.post_jobs || [];
        // If this is the current job being edited, use the pending post_jobs
        if (jobDef.id === this.currentJobDefinition.id) {
          graph.set(jobDef.id, this.currentJobDefinition.post_jobs || []);
        } else {
          graph.set(jobDef.id, postJobs);
        }
      }

      // If this is a new job (not in jobDefinitions yet), add it to the graph
      const isNewJob = !this.jobDefinitions.some(jd => jd.id === this.currentJobDefinition.id);
      if (isNewJob) {
        graph.set(this.currentJobDefinition.id, this.currentJobDefinition.post_jobs || []);
      }

      // DFS to detect if current job is reachable from any of its post_jobs
      const currentJobId = this.currentJobDefinition.id;
      const postJobs = this.currentJobDefinition.post_jobs || [];

      if (postJobs.length === 0) {
        return false; // No post-jobs, no cycle possible
      }

      // Check if current job is reachable from any of its post_jobs
      const visited = new Set();
      const recursionStack = new Set();

      const dfs = (nodeId) => {
        if (recursionStack.has(nodeId)) {
          return true; // Cycle detected
        }
        if (visited.has(nodeId)) {
          return false; // Already visited this path
        }

        visited.add(nodeId);
        recursionStack.add(nodeId);

        const neighbors = graph.get(nodeId) || [];
        for (const neighborId of neighbors) {
          if (neighborId === currentJobId) {
            return true; // Found path back to current job - cycle detected
          }
          if (dfs(neighborId)) {
            return true;
          }
        }

        recursionStack.delete(nodeId);
        return false;
      };

      // Start DFS from each post-job of the current job
      for (const postJobId of postJobs) {
        visited.clear();
        recursionStack.clear();
        if (dfs(postJobId)) {
          return true; // Cycle detected
        }
      }

      return false; // No cycle detected
    },

    async saveJobDefinition() {
      window.debugLog('JobDefinitionsManagement', 'Saving job definition:', this.currentJobDefinition);

      // Check for cycles in post-job graph before saving
      if (this.detectPostJobCycle()) {
        window.showNotification('Cannot save: Post-job configuration creates a cycle. A job cannot trigger itself indirectly through post-jobs.', 'error');
        return;
      }

      try {
        const isEdit = this.showEditModal;
        const url = isEdit ? `/api/job-definitions/${this.currentJobDefinition.id}` : '/api/job-definitions';
        const method = isEdit ? 'PUT' : 'POST';

        const jobDefToSave = JSON.parse(JSON.stringify(this.currentJobDefinition));

        window.debugLog('JobDefinitionsManagement', `${method} ${url}`, 'Job definition:', jobDefToSave);
        const response = await fetch(url, {
          method: method,
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(jobDefToSave)
        });

        window.debugLog('JobDefinitionsManagement', 'Save response status:', response.status);
        if (!response.ok) {
          const error = await response.json();
          throw new Error(error.error || 'Failed to save job definition');
        }

        window.showNotification(`Job definition ${isEdit ? 'updated' : 'created'} successfully`, 'success');
        await this.loadJobDefinitions();
        this.closeModal();
      } catch (err) {
        window.debugError('JobDefinitionsManagement', 'Error saving job definition:', err);
        window.showNotification('Failed to save job definition: ' + err.message, 'error');
      }
    },

    async deleteJobDefinition(jobDefId) {
      if (!confirm('Are you sure you want to delete this job definition?')) {
        return;
      }

      window.debugLog('JobDefinitionsManagement', 'Deleting job definition:', jobDefId);
      try {
        const response = await fetch(`/api/job-definitions/${jobDefId}`, {
          method: 'DELETE'
        });

        window.debugLog('JobDefinitionsManagement', 'Delete response status:', response.status);
        if (!response.ok) {
          throw new Error('Failed to delete job definition');
        }

        window.showNotification('Job definition deleted successfully', 'success');
        await this.loadJobDefinitions();
      } catch (err) {
        window.debugError('JobDefinitionsManagement', 'Error deleting job definition:', err);
        window.showNotification('Failed to delete job definition: ' + err.message, 'error');
      }
    },

    async executeJobDefinition(jobDefId, jobDefName) {
      if (!confirm(`Are you sure you want to execute "${jobDefName}"?`)) {
        return;
      }

      window.debugLog('JobDefinitionsManagement', 'Executing job definition:', jobDefId);
      try {
        const response = await fetch(`/api/job-definitions/${jobDefId}/execute`, {
          method: 'POST'
        });

        window.debugLog('JobDefinitionsManagement', 'Execute response status:', response.status);
        if (!response.ok) {
          const error = await response.json();
          throw new Error(error.error || 'Failed to execute job definition');
        }

        const result = await response.json();
        window.showNotification(`Job started successfully! Job ID: ${result.job_id}`, 'success');
      } catch (err) {
        window.debugError('JobDefinitionsManagement', 'Error executing job definition:', err);
        window.showNotification('Failed to execute job: ' + err.message, 'error');
      }
    },

    closeModal() {
      this.showCreateModal = false;
      this.showEditModal = false;
      document.body.classList.remove('modal-open');
      this.resetCurrentJobDefinition();

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
    },

    formatSourcesList(sources) {
      if (!sources || sources.length === 0) return 'None';
      return `${sources.length} source${sources.length !== 1 ? 's' : ''}`;
    },

    formatPostJobsList(postJobs) {
      if (!postJobs || postJobs.length === 0) return 'None';
      return `${postJobs.length} post-job${postJobs.length !== 1 ? 's' : ''}`;
    },

    getPostJobsTooltip(postJobIds) {
      if (!postJobIds || postJobIds.length === 0) return 'No post-jobs configured';
      const names = postJobIds.map(postJobId => {
        const jobDef = this.jobDefinitions.find(jd => jd.id === postJobId);
        return jobDef ? jobDef.name : postJobId + ' (deleted)';
      });
      return 'Post-jobs:\n' + names.join('\n');
    }
  }));

  // Queue Statistics Component (for queue.html)
  Alpine.data('queueStats', () => ({
    stats: {
      pending_messages: 0,
      in_flight_messages: 0,
      total_messages: 0,
      concurrency: 0,
      queue_name: 'crawler'
    },
    connectionStatus: false,

    init() {
      window.debugLog('QueueStats', 'Initializing component');
      this.subscribeToWebSocket();
      this.checkConnectionStatus();
    },

    subscribeToWebSocket() {
      if (typeof WebSocketManager !== 'undefined') {
        WebSocketManager.subscribe('queue_stats', (data) => {
          window.debugLog('QueueStats', 'Queue stats update received:', data);
          this.stats = {
            pending_messages: data.pending_messages || 0,
            in_flight_messages: data.in_flight_messages || 0,
            total_messages: data.total_messages || 0,
            concurrency: data.concurrency || 0,
            queue_name: data.queue_name || 'crawler'
          };
          this.connectionStatus = true;
        });
        window.debugLog('QueueStats', 'WebSocket subscription established');
        this.connectionStatus = true;
      } else {
        window.debugError('QueueStats', 'WebSocketManager not loaded', new Error('WebSocketManager undefined'));
        this.connectionStatus = false;
      }
    },

    checkConnectionStatus() {
      // Periodically check WebSocket connection status
      setInterval(() => {
        if (typeof WebSocketManager !== 'undefined') {
          this.connectionStatus = WebSocketManager.getConnectionStatus();
        } else {
          this.connectionStatus = false;
        }
      }, 5000);
    }
  }));

  // Queue Status Overview Component (for jobs.html)
  Alpine.data('queueStatusOverview', () => ({
    stats: {
      pending_messages: 0,
      in_flight_messages: 0,
      concurrency: 0
    },

    init() {
      window.debugLog('QueueStatusOverview', 'Initializing component');
      this.subscribeToWebSocket();
    },

    subscribeToWebSocket() {
      if (typeof WebSocketManager !== 'undefined') {
        WebSocketManager.subscribe('queue_stats', (data) => {
          window.debugLog('QueueStatusOverview', 'Queue stats update received:', data);
          this.stats = {
            pending_messages: data.pending_messages || 0,
            in_flight_messages: data.in_flight_messages || 0,
            concurrency: data.concurrency || 0
          };
        });
        window.debugLog('QueueStatusOverview', 'WebSocket subscription established');
      } else {
        window.debugError('QueueStatusOverview', 'WebSocketManager not loaded', new Error('WebSocketManager undefined'));
      }
    }
  }));

  // Job Spawn Notifications Component (optional enhancement)
  Alpine.data('jobSpawnNotifications', () => ({
    notifications: [],
    maxNotifications: 50,

    init() {
      window.debugLog('JobSpawnNotifications', 'Initializing component');
      this.subscribeToWebSocket();
    },

    subscribeToWebSocket() {
      if (typeof WebSocketManager !== 'undefined') {
        WebSocketManager.subscribe('job_spawn', (data) => {
          window.debugLog('JobSpawnNotifications', 'Job spawn event received:', data);
          this.addNotification({
            parent_job_id: data.parent_job_id || '',
            child_job_id: data.child_job_id || '',
            job_type: data.job_type || 'unknown',
            url: data.url || '',
            depth: data.depth || 0,
            timestamp: data.timestamp || new Date().toISOString()
          });
        });
        window.debugLog('JobSpawnNotifications', 'WebSocket subscription established');
      } else {
        window.debugError('JobSpawnNotifications', 'WebSocketManager not loaded', new Error('WebSocketManager undefined'));
      }
    },

    addNotification(notification) {
      this.notifications.unshift(notification);

      // Limit to maxNotifications
      if (this.notifications.length > this.maxNotifications) {
        this.notifications = this.notifications.slice(0, this.maxNotifications);
      }
    },

    formatTimestamp(timestamp) {
      if (!timestamp) return '';
      const date = new Date(timestamp);
      return date.toLocaleTimeString('en-US', {
        hour12: false,
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit'
      });
    },

    clearNotifications() {
      this.notifications = [];
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
