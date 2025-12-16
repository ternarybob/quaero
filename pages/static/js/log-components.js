/**
 * Alpine.js components for SSE log streaming
 * Requires: log-stream.js to be loaded first
 */

/**
 * Queue-specific SSE stream manager
 * Manages multiple SSE connections for jobs/steps in the queue view
 */
const QueueSSEManager = (function() {
    'use strict';

    // Active SSE streams by job ID
    const jobStreams = {};

    /**
     * Get or create an SSE stream for a job
     * @param {string} jobId - Job ID
     * @param {Object} options - Stream options
     * @returns {LogStream}
     */
    function getJobStream(jobId, options = {}) {
        if (!jobStreams[jobId]) {
            if (typeof QuaeroLogs === 'undefined') {
                console.error('[QueueSSEManager] QuaeroLogs not loaded');
                return null;
            }
            jobStreams[jobId] = QuaeroLogs.createJobStream(jobId, {
                limit: options.limit || 100,
                ...options
            });
        }
        return jobStreams[jobId];
    }

    /**
     * Connect to SSE stream for a job
     * @param {string} jobId - Job ID
     * @param {Object} options - Stream options including callbacks
     * @returns {LogStream}
     */
    function connectJob(jobId, options = {}) {
        const stream = getJobStream(jobId, options);
        if (!stream) return null;

        // Bind callbacks if provided
        if (options.onLogs) stream.onLogs = options.onLogs;
        if (options.onStatus) stream.onStatus = options.onStatus;
        if (options.onConnect) stream.onConnect = options.onConnect;
        if (options.onDisconnect) stream.onDisconnect = options.onDisconnect;
        if (options.onError) stream.onError = options.onError;

        // Connect with filters
        stream.connect({
            stepId: options.stepId,
            level: options.level,
            limit: options.limit
        });

        return stream;
    }

    /**
     * Disconnect SSE stream for a job
     * @param {string} jobId - Job ID
     */
    function disconnectJob(jobId) {
        if (jobStreams[jobId]) {
            jobStreams[jobId].disconnect();
            delete jobStreams[jobId];
        }
    }

    /**
     * Update step filter for a job stream
     * @param {string} jobId - Job ID
     * @param {string} stepId - Step ID to filter
     */
    function setStepFilter(jobId, stepId) {
        const stream = jobStreams[jobId];
        if (stream) {
            stream.reconnect({ stepId });
        }
    }

    /**
     * Check if a job has an active SSE stream
     * @param {string} jobId - Job ID
     * @returns {boolean}
     */
    function hasStream(jobId) {
        return !!jobStreams[jobId] && jobStreams[jobId].connected;
    }

    /**
     * Disconnect all streams
     */
    function disconnectAll() {
        Object.keys(jobStreams).forEach(jobId => {
            disconnectJob(jobId);
        });
    }

    return {
        getJobStream,
        connectJob,
        disconnectJob,
        setStepFilter,
        hasStream,
        disconnectAll
    };
})();

// Export to window for global access
window.QueueSSEManager = QueueSSEManager;

document.addEventListener('alpine:init', () => {
    /**
     * Job log viewer component for step logs
     * Usage: <div x-data="sseJobLogViewer('job-uuid')">
     */
    Alpine.data('sseJobLogViewer', (jobId) => ({
        stream: null,
        logs: [],
        totalCount: 0,
        displayedCount: 0,
        jobStatus: null,
        steps: [],
        connected: false,
        autoScroll: true,

        filters: {
            stepId: '',
            level: 'info',
            limit: 100
        },

        init() {
            if (!jobId) {
                console.warn('[sseJobLogViewer] No job ID provided');
                return;
            }

            if (typeof QuaeroLogs === 'undefined') {
                console.error('[sseJobLogViewer] QuaeroLogs not loaded');
                return;
            }

            this.stream = QuaeroLogs.createJobStream(jobId, { limit: this.filters.limit });
            this.bindStreamCallbacks();
            this.stream.connect(this.filters);

            // Watch for filter changes
            this.$watch('filters', (newFilters) => {
                this.logs = [];
                this.stream.reconnect(newFilters);
            }, { deep: true });
        },

        destroy() {
            if (this.stream) {
                this.stream.disconnect();
                this.stream = null;
            }
        },

        bindStreamCallbacks() {
            this.stream.onLogs = (data) => {
                this.logs = this.stream.logs;
                this.totalCount = this.stream.totalCount;
                this.displayedCount = this.stream.displayedCount;
                if (this.autoScroll) {
                    this.$nextTick(() => this.scrollToBottom());
                }
            };

            this.stream.onStatus = (data) => {
                this.jobStatus = data.job;
                this.steps = data.steps || [];
            };

            this.stream.onConnect = () => {
                this.connected = true;
            };

            this.stream.onDisconnect = () => {
                this.connected = false;
            };
        },

        scrollToBottom() {
            const container = this.$refs.logContainer;
            if (!container) return;
            container.scrollTop = container.scrollHeight;
        },

        scrollToBottomIfNeeded() {
            const container = this.$refs.logContainer;
            if (!container) return;
            const isAtBottom = container.scrollHeight - container.scrollTop <= container.clientHeight + 50;
            if (isAtBottom) {
                container.scrollTop = container.scrollHeight;
            }
        },

        setFilter(key, value) {
            this.filters[key] = value;
        },

        clearLogs() {
            this.logs = [];
            if (this.stream) {
                this.stream.clear();
            }
        },

        formatTime(ts) {
            return QuaeroLogs.formatTimestamp(ts);
        },

        levelClass(level) {
            return QuaeroLogs.levelClass(level);
        }
    }));

    /**
     * SSE-based service log viewer component (for service logs panel)
     * Usage: <div x-data="sseServiceLogs">
     */
    Alpine.data('sseServiceLogs', () => ({
        stream: null,
        logs: [],
        totalCount: 0,
        displayedCount: 0,
        serviceStatus: null,
        connected: false,
        autoScroll: true,
        maxLogs: 200,
        logIdCounter: 0,

        filters: {
            level: 'info',
            limit: 100
        },

        init() {
            if (typeof QuaeroLogs === 'undefined') {
                console.error('[sseServiceLogs] QuaeroLogs not loaded');
                return;
            }

            // Check if already initialized this session (prevents duplicate connections)
            if (sessionStorage.getItem('sseServiceLogsInitialized')) {
                console.log('[sseServiceLogs] Already initialized this session, connecting...');
            }
            sessionStorage.setItem('sseServiceLogsInitialized', 'true');

            this.stream = QuaeroLogs.createServiceStream({ limit: this.filters.limit });
            this.bindStreamCallbacks();
            this.stream.connect(this.filters);

            // Watch for filter changes
            this.$watch('filters', (newFilters) => {
                this.logs = [];
                this.stream.reconnect(newFilters);
            }, { deep: true });
        },

        destroy() {
            if (this.stream) {
                this.stream.disconnect();
                this.stream = null;
            }
        },

        bindStreamCallbacks() {
            this.stream.onLogs = (data) => {
                // Process logs with IDs for reactivity
                if (data.logs && Array.isArray(data.logs)) {
                    for (const log of data.logs) {
                        log.id = ++this.logIdCounter;
                        log.levelClass = this.getLevelClass(log.level);
                    }
                }

                this.logs = this.stream.logs.slice(-this.maxLogs);
                this.totalCount = this.stream.totalCount;
                this.displayedCount = this.stream.displayedCount;

                if (this.autoScroll) {
                    this.$nextTick(() => this.scrollToBottom());
                }
            };

            this.stream.onStatus = (data) => {
                this.serviceStatus = data.service;
            };

            this.stream.onConnect = () => {
                this.connected = true;
            };

            this.stream.onDisconnect = () => {
                this.connected = false;
            };
        },

        scrollToBottom() {
            const container = this.$refs.logContainer;
            if (!container) return;
            container.scrollTop = container.scrollHeight;
        },

        toggleAutoScroll() {
            this.autoScroll = !this.autoScroll;
            if (this.autoScroll) {
                this.scrollToBottom();
            }
        },

        refresh() {
            if (this.stream) {
                this.logs = [];
                this.stream.reconnect(this.filters);
            }
        },

        clearLogs() {
            this.logs = [];
            if (this.stream) {
                this.stream.clear();
            }
        },

        getLevelClass(level) {
            const levelMap = {
                'ERR': 'terminal-error',
                'ERROR': 'terminal-error',
                'error': 'terminal-error',
                'WRN': 'terminal-warning',
                'WARN': 'terminal-warning',
                'warn': 'terminal-warning',
                'INF': 'terminal-info',
                'INFO': 'terminal-info',
                'info': 'terminal-info',
                'DBG': 'terminal-debug',
                'DEBUG': 'terminal-debug',
                'debug': 'terminal-debug'
            };
            return levelMap[level] || 'terminal-info';
        },

        formatTime(ts) {
            return QuaeroLogs.formatTimestamp(ts);
        }
    }));
});
