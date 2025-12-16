/**
 * Quaero SSE Log Streaming Library
 * Global utilities for log streaming across all pages
 */

const QuaeroLogs = (function() {
    'use strict';

    // Default configuration
    const defaults = {
        limit: 100,
        fallbackTimeoutMs: 15000,
        maxBufferSize: 1000,
        reconnectDelayMs: 1000
    };

    /**
     * Create a new log stream connection for job logs
     * Unified endpoint: /api/logs/stream?scope=job&job_id=X&step=Y&level=info
     * @param {string} jobId - Job ID to stream logs for
     * @param {Object} options - Stream options (stepId, level, limit)
     * @returns {LogStream}
     */
    function createJobStream(jobId, options = {}) {
        return new LogStream('/api/logs/stream', {
            ...defaults,
            ...options,
            scope: 'job',
            jobId: jobId
        });
    }

    /**
     * Create a service log stream (for global service logs on all pages)
     * Unified endpoint: /api/logs/stream?scope=service&level=info
     * @param {Object} options - Stream options (level, limit)
     * @returns {LogStream}
     */
    function createServiceStream(options = {}) {
        return new LogStream('/api/logs/stream', {
            ...defaults,
            ...options,
            scope: 'service',
            isServiceLog: true
        });
    }

    /**
     * Build query string from filter options
     * @param {Object} filters
     * @returns {string}
     */
    function buildQueryParams(filters) {
        const params = new URLSearchParams();

        // Scope is required - default to 'service'
        params.set('scope', filters.scope || 'service');

        // Job ID required for scope=job
        if (filters.jobId) params.set('job_id', filters.jobId);
        if (filters.limit) params.set('limit', filters.limit);
        if (filters.stepId) params.set('step', filters.stepId);
        if (filters.level) params.set('level', filters.level);
        if (filters.since) params.set('since', filters.since);

        return params.toString();
    }

    /**
     * Format timestamp for display
     * @param {string} isoTimestamp
     * @param {boolean} includeDate
     * @returns {string}
     */
    function formatTimestamp(isoTimestamp, includeDate = false) {
        if (!isoTimestamp) return '';
        const date = new Date(isoTimestamp);
        if (isNaN(date.getTime())) {
            // If not a valid date, return as-is (might be just time string like "10:30:00")
            return isoTimestamp;
        }
        if (includeDate) {
            return date.toLocaleString();
        }
        return date.toLocaleTimeString();
    }

    /**
     * Get CSS class for log level
     * @param {string} level
     * @returns {string}
     */
    function levelClass(level) {
        const classes = {
            'debug': 'log-level-debug',
            'dbg': 'log-level-debug',
            'info': 'log-level-info',
            'inf': 'log-level-info',
            'warn': 'log-level-warn',
            'wrn': 'log-level-warn',
            'warning': 'log-level-warn',
            'error': 'log-level-error',
            'err': 'log-level-error',
            'fatal': 'log-level-fatal'
        };
        return classes[level?.toLowerCase()] || 'log-level-info';
    }

    /**
     * LogStream class - manages SSE connection and log state
     */
    class LogStream {
        constructor(endpoint, options) {
            this.endpoint = endpoint;
            this.options = options;
            this.eventSource = null;
            this.logs = [];
            this.totalCount = 0;
            this.displayedCount = 0;
            this.status = null;
            this.steps = [];
            this.lastEventTime = Date.now();
            this.fallbackTimer = null;
            this.connected = false;
            this.reconnectAttempts = 0;
            this.maxReconnectAttempts = 10;
            this.reconnectTimer = null;

            // Callbacks
            this.onLogs = null;
            this.onStatus = null;
            this.onError = null;
            this.onConnect = null;
            this.onDisconnect = null;
        }

        /**
         * Connect to SSE stream
         * @param {Object} filters - Optional filter overrides
         */
        connect(filters = {}) {
            if (this.eventSource) {
                this.disconnect();
            }

            // CRITICAL: Include scope and jobId from options - these are required for job log streams
            const queryString = buildQueryParams({
                scope: this.options.scope,
                jobId: this.options.jobId,
                limit: this.options.limit,
                ...filters
            });

            const url = queryString ? `${this.endpoint}?${queryString}` : this.endpoint;

            console.log('[QuaeroLogs] Connecting to SSE:', url);

            try {
                this.eventSource = new EventSource(url);
            } catch (err) {
                console.error('[QuaeroLogs] Failed to create EventSource:', err);
                if (this.onError) this.onError(err);
                this.scheduleReconnect(filters);
                return;
            }

            this.eventSource.addEventListener('logs', (e) => {
                try {
                    const data = JSON.parse(e.data);
                    console.log('[QuaeroLogs] SSE logs event received:', { logsCount: data.logs?.length, meta: data.meta });
                    this.handleLogs(data);
                } catch (err) {
                    console.error('[QuaeroLogs] Error parsing logs event:', err);
                }
            });

            this.eventSource.addEventListener('status', (e) => {
                try {
                    this.handleStatus(JSON.parse(e.data));
                } catch (err) {
                    console.error('[QuaeroLogs] Error parsing status event:', err);
                }
            });

            this.eventSource.addEventListener('ping', () => {
                this.lastEventTime = Date.now();
            });

            this.eventSource.onopen = () => {
                console.log('[QuaeroLogs] SSE connected');
                this.connected = true;
                this.reconnectAttempts = 0;
                this.lastEventTime = Date.now();
                if (this.onConnect) this.onConnect();
            };

            this.eventSource.onerror = (err) => {
                console.error('[QuaeroLogs] SSE error:', err);
                this.connected = false;
                if (this.onError) this.onError(err);
                if (this.onDisconnect) this.onDisconnect();

                // EventSource will auto-reconnect, but we track state
                if (this.eventSource && this.eventSource.readyState === EventSource.CLOSED) {
                    this.scheduleReconnect(filters);
                }
            };

            // Setup fallback timer for stale connections
            this.startFallbackTimer(filters);
        }

        /**
         * Schedule a reconnection attempt
         * @param {Object} filters
         */
        scheduleReconnect(filters) {
            if (this.reconnectTimer) {
                clearTimeout(this.reconnectTimer);
            }

            if (this.reconnectAttempts >= this.maxReconnectAttempts) {
                console.warn('[QuaeroLogs] Max reconnect attempts reached');
                return;
            }

            const delay = Math.min(
                this.options.reconnectDelayMs * Math.pow(2, this.reconnectAttempts),
                30000
            );
            this.reconnectAttempts++;

            console.log(`[QuaeroLogs] Reconnecting in ${delay}ms (attempt ${this.reconnectAttempts})`);

            this.reconnectTimer = setTimeout(() => {
                this.connect(filters);
            }, delay);
        }

        /**
         * Disconnect from SSE stream
         */
        disconnect() {
            if (this.reconnectTimer) {
                clearTimeout(this.reconnectTimer);
                this.reconnectTimer = null;
            }

            if (this.eventSource) {
                this.eventSource.close();
                this.eventSource = null;
            }
            this.connected = false;
            this.stopFallbackTimer();
            if (this.onDisconnect) this.onDisconnect();
        }

        /**
         * Reconnect with new filters
         * @param {Object} filters
         */
        reconnect(filters = {}) {
            this.logs = [];
            this.reconnectAttempts = 0;
            this.connect(filters);
        }

        /**
         * Handle incoming log batch
         * @param {Object} data
         */
        handleLogs(data) {
            this.lastEventTime = Date.now();

            if (!data.logs || !Array.isArray(data.logs)) {
                return;
            }

            // Append new logs
            this.logs.push(...data.logs);

            // Trim buffer if needed
            const maxBuffer = this.options.maxBufferSize || 1000;
            if (this.logs.length > maxBuffer) {
                this.logs = this.logs.slice(-Math.floor(maxBuffer / 2));
            }

            if (data.meta) {
                this.totalCount = data.meta.total_count || this.totalCount;
                this.displayedCount = data.meta.displayed_count || this.logs.length;
            }

            if (this.onLogs) this.onLogs(data);
        }

        /**
         * Handle status update
         * @param {Object} data
         */
        handleStatus(data) {
            this.lastEventTime = Date.now();
            this.status = data.job || data.service;
            this.steps = data.steps || [];

            if (this.onStatus) this.onStatus(data);
        }

        /**
         * Start fallback API polling timer
         * @param {Object} filters
         */
        startFallbackTimer(filters) {
            this.stopFallbackTimer();

            this.fallbackTimer = setInterval(() => {
                if (Date.now() - this.lastEventTime > this.options.fallbackTimeoutMs) {
                    console.log('[QuaeroLogs] No events received, fetching via API');
                    this.fetchViaApi(filters);
                }
            }, this.options.fallbackTimeoutMs);
        }

        /**
         * Stop fallback timer
         */
        stopFallbackTimer() {
            if (this.fallbackTimer) {
                clearInterval(this.fallbackTimer);
                this.fallbackTimer = null;
            }
        }

        /**
         * Fallback API fetch
         * @param {Object} filters
         */
        async fetchViaApi(filters = {}) {
            try {
                const lastId = this.logs.length > 0
                    ? this.logs[this.logs.length - 1].id
                    : '';

                // Convert stream endpoint to regular endpoint
                const apiEndpoint = this.endpoint.replace('/stream', '').replace('/api/jobs/', '/api/logs?scope=job&job_id=');
                const queryString = buildQueryParams({
                    limit: this.options.limit,
                    since: lastId,
                    ...filters
                });

                let url = apiEndpoint;
                if (this.options.isServiceLog) {
                    url = '/api/logs?scope=service';
                }
                if (queryString) {
                    url += (url.includes('?') ? '&' : '?') + queryString;
                }

                const res = await fetch(url);
                if (!res.ok) throw new Error(`HTTP ${res.status}`);

                const data = await res.json();
                if (data.logs) {
                    this.handleLogs({ logs: data.logs, meta: { total_count: data.total_count, displayed_count: data.count } });
                }
            } catch (err) {
                console.error('[QuaeroLogs] API fallback failed:', err);
                if (this.onError) this.onError(err);
            }
        }

        /**
         * Clear log buffer
         */
        clear() {
            this.logs = [];
            this.totalCount = 0;
            this.displayedCount = 0;
        }

        /**
         * Get current state for Alpine.js reactivity
         * @returns {Object}
         */
        getState() {
            return {
                logs: this.logs,
                totalCount: this.totalCount,
                displayedCount: this.displayedCount,
                status: this.status,
                steps: this.steps,
                connected: this.connected
            };
        }
    }

    // Public API
    return {
        createJobStream,
        createServiceStream,
        buildQueryParams,
        formatTimestamp,
        levelClass,
        LogStream
    };
})();

// Export to window for global access
window.QuaeroLogs = QuaeroLogs;
