/**
 * Settings Page Components for Quaero
 *
 * This file contains all Alpine.js components specifically for the settings page,
 * extracted from common.js to improve code organization and maintainability.
 *
 * Components:
 * - settingsNavigation: Two-column layout navigation component
 * - settingsStatus: Service status and configuration display
 * - settingsConfig: Configuration details viewer
 * - authCookies: Authentication cookies management
 * - authApiKeys: API keys management
 * - settingsDanger: Dangerous operations (document deletion)
 *
 * @version 3.0.0
 * @date 2025-11-13
 * @since 3.0.0 - Enhanced modularity and reusability
 */

// Ensure dependencies are available
if (typeof window.showNotification === 'undefined') {
    console.warn('Settings Components: window.showNotification function not found. Notifications may not work properly.');
}

document.addEventListener('alpine:init', () => {
    // Enhanced debug logging with namespace
    const logger = {
        debug: (component, message, ...args) => window.debugLog?.(`Settings:${component}`, message, ...args),
        error: (component, message, error) => window.debugError?.(`Settings:${component}`, message, error)
    };

    logger.debug('Bootstrap', 'Settings components initialization started');

    // === GLOBAL COMPONENT STATE CACHE ===
    // Prevents duplicate API calls when components are re-initialized by Alpine
    // when accordion content is re-rendered via x-html
    const componentStateCache = {
        authCookies: { hasLoaded: false, data: [] },
        authApiKeys: { hasLoaded: false, data: [] },
        config: { hasLoaded: false, data: null }
    };

    // === UTILITY MIXINS AND HELPERS ===

    /**
     * Base Component Mixin - provides common functionality for all components
     */
    const BaseComponentMixin = () => ({
        // Common state management
        isLoading: false,
        error: null,
        lastUpdated: null,

        // Common methods
        init() {
            if (this.setupComponent) {
                this.setupComponent();
            }
        },

        /**
         * Prevent concurrent requests
         */
        get isRequestInProgress() {
            return this.isLoading === true;
        },

        /**
         * Generic API request handler with error handling
         */
        async makeRequest(url, options = {}) {
            if (this.isRequestInProgress) {
                logger.debug(this.$options.name || 'Component', 'Request already in progress, skipping');
                return null;
            }

            this.isLoading = true;
            this.error = null;

            try {
                const response = await fetch(url, {
                    headers: {
                        'Content-Type': 'application/json',
                        ...options.headers
                    },
                    ...options
                });

                if (!response.ok) {
                    throw new Error(`Request failed: ${response.status} ${response.statusText}`);
                }

                return await response.json();
            } catch (error) {
                this.error = error.message;
                logger.error(this.$options.name || 'Component', 'Request failed', error);
                throw error;
            } finally {
                this.isLoading = false;
            }
        },

        /**
         * Show notification with error handling
         */
        notify(message, type = 'info') {
            if (window.showNotification) {
                window.showNotification(message, type);
            } else {
                console[type === 'error' ? 'error' : 'log'](`[${type.toUpperCase()}] ${message}`);
            }
        },

        /**
         * Enhanced confirmation dialog
         */
        confirmDialog(title, message, details = '') {
            const fullMessage = `${title}\n\n${message}${details ? '\n\n' + details : ''}`;
            return confirm(fullMessage);
        },

        /**
         * Generic refresh method - should be overridden by components
         */
        async refresh() {
            logger.debug(this.$options.name || 'Component', 'Refresh called but not implemented');
        }
    });

    /**
     * Data Validation Mixin - provides common validation utilities
     */
    const DataValidationMixin = () => ({
        /**
         * Sanitize string values to prevent XSS
         */
        sanitizeString(value) {
            if (typeof value !== 'string') return '';
            return value.replace(/[<>"'`]/g, '');
        },

        /**
         * Validate port number
         */
        validatePort(port) {
            const numPort = parseInt(port, 10);
            return (Number.isInteger(numPort) && numPort > 0 && numPort <= 65535) ? numPort : 0;
        },

        /**
         * Check if a config key contains sensitive information
         */
        isSensitiveKey(key) {
            const sensitiveKeys = [
                'password', 'secret', 'key', 'token', 'auth',
                'credential', 'private', 'api_key', 'access_token'
            ];
            return sensitiveKeys.some(sensitive => key.toLowerCase().includes(sensitive));
        },

        /**
         * Sanitize and validate configuration data recursively
         */
        sanitizeConfigData(config) {
            if (!config || typeof config !== 'object') {
                return null;
            }

            // Create a safe copy with filtered sensitive data
            const sanitized = {};
            for (const [key, value] of Object.entries(config)) {
                // Skip sensitive keys
                if (this.isSensitiveKey(key)) {
                    sanitized[key] = '[REDACTED]';
                    continue;
                }

                // Recursively sanitize nested objects
                if (typeof value === 'object' && value !== null) {
                    sanitized[key] = this.sanitizeConfigData(value);
                } else if (typeof value === 'string') {
                    sanitized[key] = this.sanitizeString(value);
                } else {
                    sanitized[key] = value;
                }
            }

            return sanitized;
        },

        /**
         * Validate authentication data structure
         */
        isValidAuthItem(auth) {
            return auth && typeof auth === 'object';
        },

        /**
         * Sanitize authentication data
         */
        sanitizeAuthData(auth) {
            if (!this.isValidAuthItem(auth)) {
                return null;
            }

            return {
                id: this.sanitizeString(auth.id),
                name: this.sanitizeString(auth.name || auth.site_domain || 'Unknown'),
                siteDomain: this.sanitizeString(auth.site_domain || ''),
                authType: this.sanitizeString(auth.auth_type || 'unknown'),
                createdAt: auth.created_at,
                updatedAt: auth.updated_at
            };
        }
    });

    /**
     * Form Management Mixin - provides common form handling
     */
    const FormManagementMixin = () => ({
        // Form state
        formData: {},
        isSaving: false,
        validationErrors: {},

        /**
         * Reset form to default state
         */
        resetForm(defaultData = {}) {
            this.formData = { ...defaultData };
            this.validationErrors = {};
            this.isSaving = false;
        },

        /**
         * Validate form data
         */
        validateForm(rules = {}) {
            const errors = {};

            for (const [field, rule] of Object.entries(rules)) {
                const value = this.formData[field];

                if (rule.required && (!value || value.toString().trim() === '')) {
                    errors[field] = `${field} is required`;
                    continue;
                }

                if (value && rule.pattern && !rule.pattern.test(value)) {
                    errors[field] = rule.message || `${field} format is invalid`;
                }
            }

            this.validationErrors = errors;
            return Object.keys(errors).length === 0;
        },

        /**
         * Generic form submission handler
         */
        async submitForm(url, method = 'POST', validationRules = {}) {
            if (!this.validateForm(validationRules)) {
                this.notify('Please fix validation errors', 'error');
                return false;
            }

            this.isSaving = true;

            try {
                const response = await this.makeRequest(url, {
                    method,
                    body: JSON.stringify(this.formData)
                });

                this.notify('Form submitted successfully', 'success');
                return response;
            } catch (error) {
                this.notify(`Form submission failed: ${error.message}`, 'error');
                return false;
            } finally {
                this.isSaving = false;
            }
        }
    });

    // === COMPONENT DEFINITIONS WITH ENHANCED MODULARITY ===

    // Settings Navigation Component (Two-Column Layout)
    Alpine.data('settingsNavigation', () => ({
        content: {},
        loading: {},
        loadedSections: new Set(),
        activeSection: 'auth-apikeys',
        defaultSection: 'auth-apikeys',
        validSections: ['auth-apikeys', 'auth-cookies', 'config', 'danger', 'status', 'logs'],

        init() {
            window.debugLog('SettingsNavigation', 'Initializing component');
            // Parse URL parameter 'a' to get active section (single value)
            const urlSection = this.getActiveSection();
            window.debugLog('SettingsNavigation', 'Active section from URL:', urlSection);

            // Set active section (from URL or use default), with validation
            this.activeSection = this.validateSectionId(urlSection) || this.defaultSection;

            // Load the active section
            this.selectSection(this.activeSection);
        },

        validateSectionId(sectionId) {
            // Validate section ID against whitelist
            if (!sectionId || !this.validSections.includes(sectionId)) {
                window.debugLog('SettingsNavigation', `Invalid section ID: ${sectionId}, falling back to default`);
                return null;
            }
            return sectionId;
        },

        selectSection(sectionId) {
            // Validate section ID before using
            const validSectionId = this.validateSectionId(sectionId);
            if (!validSectionId) {
                window.debugLog('SettingsNavigation', `Invalid section: ${sectionId}, using default`);
                sectionId = this.defaultSection;
            }

            window.debugLog('SettingsNavigation', `selectSection called: ${sectionId}`);

            // Set as active section
            this.activeSection = sectionId;

            // Determine the partial URL based on section ID
            const partialUrl = `/settings/${sectionId}.html`;

            // Load content if not already loaded
            this.loadContent(sectionId, partialUrl);

            // Update URL
            this.updateUrl(sectionId);
        },

        async loadContent(sectionId, partialUrl) {
            // If already loaded, just return (use cache)
            if (this.loadedSections.has(sectionId)) {
                window.debugLog('SettingsNavigation', `Section ${sectionId} already loaded, using cache`);
                return;
            }

            // Set loading state
            this.loading[sectionId] = true;

            try {
                window.debugLog('SettingsNavigation', `Fetching partial: ${partialUrl}`);
                const response = await fetch(partialUrl);

                if (!response.ok) {
                    throw new Error(`HTTP error! status: ${response.status}`);
                }

                const html = await response.text();
                window.debugLog('SettingsNavigation', `Loaded ${html.length} bytes for ${sectionId}`);

                // Store content and mark as loaded
                this.content[sectionId] = html;
                this.loadedSections.add(sectionId);
                this.loading[sectionId] = false;

            } catch (error) {
                window.debugError('SettingsNavigation', `Error loading ${sectionId}:`, error);
                this.loading[sectionId] = false;

                if (typeof window.showNotification === 'function') {
                    window.showNotification(`Failed to load ${sectionId}: ${error.message}`, 'error');
                }
            }
        },

        updateUrl(sectionId) {
            const url = new URL(window.location);
            const params = new URLSearchParams(url.search);

            // Set 'a' parameter to single active section
            params.set('a', sectionId);

            // Update URL without page reload
            url.search = params.toString();
            window.history.replaceState({}, '', url);

            window.debugLog('SettingsNavigation', `URL updated: ${url.search}`);
        },

        getActiveSection() {
            const params = new URLSearchParams(window.location.search);
            const sectionParam = params.get('a');

            if (!sectionParam || sectionParam.trim() === '') {
                return null;
            }

            // Return the single section ID
            return sectionParam.trim();
        }
    }));

    // Service Status Component - Enhanced with mixin patterns
    Alpine.data('settingsStatus', () => ({
        // Component-specific state
        isOnline: false,

        // Data properties with sensible defaults
        version: 'unknown',
        build: 'unknown',
        port: 0,
        host: '',

        // Mix in base functionality
        ...BaseComponentMixin(),

        // Mix in data validation
        ...DataValidationMixin(),

        /**
         * Component-specific setup
         */
        setupComponent() {
            this.loadConfig();
        },

        /**
         * Component-specific configuration loading
         */
        async loadConfig() {
            try {
                logger.debug('Status', 'Loading service configuration');

                const data = await this.makeRequest('/api/config');

                if (data) {
                    // Enhanced data validation and sanitization
                    this.isOnline = true;
                    this.version = this.sanitizeString(data.version) || 'unknown';
                    this.build = this.sanitizeString(data.build) || 'unknown';
                    this.port = this.validatePort(data.port) || 0;
                    this.host = this.sanitizeString(data.host) || '';
                    this.lastUpdated = new Date();

                    logger.debug('Status', 'Configuration loaded successfully', {
                        version: this.version,
                        build: this.build,
                        port: this.port
                    });
                }

            } catch (error) {
                this.isOnline = false;
                // Graceful degradation - maintain default values
                this.version = 'unknown';
                this.build = 'unknown';
                this.port = 0;
                this.host = '';

                this.notify(`Failed to load service status: ${error.message}`, 'error');
            }
        },

        /**
         * Manual refresh configuration
         */
        async refresh() {
            await this.loadConfig();
        }
    }));

    // Configuration Details Component - Simplified to display config as JSON
    Alpine.data('settingsConfig', () => ({
        config: null,
        isLoading: false,

        init() {
            // Check global cache to prevent duplicate API calls
            if (componentStateCache.config.hasLoaded) {
                logger.debug('Config', 'Loading from cache, skipping API call');
                this.config = componentStateCache.config.data;
                this.isLoading = false;
                return;
            }

            // First load - fetch from API
            this.loadConfig();
        },

        async loadConfig() {
            if (this.isLoading) return;

            this.isLoading = true;

            try {
                logger.debug('Config', 'Fetching configuration from /api/config');

                const response = await fetch('/api/config');
                if (!response.ok) {
                    throw new Error(`HTTP error! status: ${response.status}`);
                }

                const data = await response.json();

                // Store the entire response data for display
                this.config = data;

                // Store in global cache
                componentStateCache.config.hasLoaded = true;
                componentStateCache.config.data = data;

                logger.debug('Config', 'Configuration loaded successfully', {
                    keys: Object.keys(data)
                });
            } catch (error) {
                logger.error('Config', 'Failed to load configuration', error);
                this.config = null;

                // Show notification if available
                if (window.showNotification) {
                    window.showNotification('Failed to load configuration', 'error');
                }
            } finally {
                this.isLoading = false;
            }
        },

        formatConfig(cfg) {
            if (!cfg) {
                return 'No configuration loaded';
            }

            try {
                return JSON.stringify(cfg, null, 2);
            } catch (error) {
                logger.error('Config', 'Failed to format configuration', error);
                return 'Error formatting configuration';
            }
        }
    }));

    // Authentication Cookies Component - Enhanced with better error handling
    Alpine.data('authCookies', () => ({
        // State management
        authentications: [],
        isLoading: false,
        deleting: null,
        error: null,
        lastUpdated: null,

        /**
         * Component initialization
         */
        init() {
            // Check global cache to prevent duplicate API calls when component re-initializes
            if (componentStateCache.authCookies.hasLoaded) {
                logger.debug('AuthCookies', 'Loading from cache, skipping API call');
                this.authentications = componentStateCache.authCookies.data;
                return;
            }

            // First load - fetch from API
            this.loadAuthentications();
        },

        /**
         * Enhanced authentication loading with better filtering
         */
        async loadAuthentications() {
            if (this.isLoading) return; // Prevent concurrent requests

            this.isLoading = true;
            this.error = null;

            try {
                logger.debug('AuthCookies', 'Loading authentication data');

                const response = await fetch('/api/auth/list');
                if (!response.ok) {
                    throw new Error(`Authentication request failed: ${response.status} ${response.statusText}`);
                }

                const data = await response.json();

                // Enhanced filtering with validation
                if (Array.isArray(data)) {
                    this.authentications = data
                        .filter(auth => this.isValidCookieAuth(auth))
                        .map(auth => this.sanitizeAuthData(auth));
                } else {
                    this.authentications = [];
                }

                this.lastUpdated = new Date();

                // Store in global cache to prevent duplicate API calls on re-initialization
                componentStateCache.authCookies.hasLoaded = true;
                componentStateCache.authCookies.data = this.authentications;

                logger.debug('AuthCookies', 'Authentication data loaded successfully', {
                    count: this.authentications.length
                });

            } catch (error) {
                this.error = error.message;
                logger.error('AuthCookies', 'Failed to load authentications', error);
                this.authentications = [];

                // Show user-friendly notification
                if (window.showNotification) {
                    window.showNotification('Failed to load authentications', 'error');
                }
            } finally {
                this.isLoading = false;
            }
        },

        /**
         * Validate if authentication item is a cookie-based auth
         */
        isValidCookieAuth(auth) {
            // Include if it's not an API key OR if auth_type is missing
            return auth.auth_type !== 'api_key' || !('auth_type' in auth);
        },

        /**
         * Sanitize authentication data
         */
        sanitizeAuthData(auth) {
            return {
                id: this.sanitizeString(auth.id),
                name: this.sanitizeString(auth.name || auth.site_domain || 'Unknown'),
                siteDomain: this.sanitizeString(auth.site_domain || ''),
                authType: this.sanitizeString(auth.auth_type || 'unknown'),
                createdAt: auth.created_at,
                updatedAt: auth.updated_at
            };
        },

        /**
         * Sanitize string values to prevent XSS
         */
        sanitizeString(value) {
            if (typeof value !== 'string') return '';
            return value.replace(/[<>"'`]/g, '');
        },

        /**
         * Enhanced authentication deletion with better UX
         */
        async deleteAuthentication(id, siteDomain) {
            const authItem = this.authentications.find(auth => auth.id === id);
            const displayName = siteDomain || authItem?.name || 'this authentication';

            // Enhanced confirmation dialog
            const confirmed = confirm(
                `⚠️ Delete Authentication\n\n` +
                `Are you sure you want to delete authentication for "${displayName}"?\n\n` +
                `This action cannot be undone and any sources using this authentication will need to be updated.`
            );

            if (!confirmed) return;

            this.deleting = id;
            this.error = null;

            try {
                logger.debug('AuthCookies', 'Deleting authentication', { id, siteDomain: displayName });

                const response = await fetch(`/api/auth/${id}`, {
                    method: 'DELETE'
                });

                if (!response.ok) {
                    throw new Error(`Delete request failed: ${response.status} ${response.statusText}`);
                }

                // Remove from local array with proper state update
                this.authentications = this.authentications.filter(auth => auth.id !== id);

                logger.debug('AuthCookies', 'Authentication deleted successfully');

                // Success notification
                if (window.showNotification) {
                    window.showNotification('Authentication deleted successfully', 'success');
                }

            } catch (error) {
                this.error = error.message;
                logger.error('AuthCookies', 'Failed to delete authentication', error);

                // Error notification
                if (window.showNotification) {
                    window.showNotification('Failed to delete authentication: ' + error.message, 'error');
                }
            } finally {
                this.deleting = null;
            }
        },

        /**
         * Manual refresh authentication data
         */
        async refresh() {
            await this.loadAuthentications();
        }
    }));

    // Authentication API Keys Component
    Alpine.data('authApiKeys', () => ({
        apiKeys: [],
        loading: true,
        deleting: null,
        saving: false,
        showCreateModal: false,
        showEditModal: false,
        editingKey: null,
        formData: {
            key: '',
            value: '',
            description: ''
        },

        init() {
            // Check global cache to prevent duplicate API calls when component re-initializes
            if (componentStateCache.authApiKeys.hasLoaded) {
                logger.debug('AuthApiKeys', 'Loading from cache, skipping API call');
                this.apiKeys = componentStateCache.authApiKeys.data;
                this.loading = false;
                return;
            }

            // First load - fetch from API
            this.loadApiKeys();
        },

        async loadApiKeys() {
            this.loading = true;
            try {
                const response = await fetch('/api/kv');
                if (!response.ok) {
                    throw new Error(`HTTP error! status: ${response.status}`);
                }
                const data = await response.json();
                // Store key/value pairs
                if (Array.isArray(data)) {
                    this.apiKeys = data;
                } else {
                    this.apiKeys = [];
                }

                // Store in global cache to prevent duplicate API calls on re-initialization
                componentStateCache.authApiKeys.hasLoaded = true;
                componentStateCache.authApiKeys.data = this.apiKeys;
            } catch (error) {
                console.error('Failed to load API keys:', error);
                window.showNotification('Failed to load API keys', 'error');
            } finally {
                this.loading = false;
            }
        },

        async deleteApiKey(key, displayName) {
            if (!confirm(`Are you sure you want to delete key "${displayName}"?\n\nAny job definitions using this key will fail.`)) {
                return;
            }

            this.deleting = key;
            try {
                const response = await fetch(`/api/kv/${encodeURIComponent(key)}`, {
                    method: 'DELETE'
                });

                if (!response.ok) {
                    throw new Error(`HTTP error! status: ${response.status}`);
                }

                // Remove from local array
                this.apiKeys = this.apiKeys.filter(item => item.key !== key);
                window.showNotification('Key deleted successfully', 'success');
            } catch (error) {
                console.error('Failed to delete key:', error);
                window.showNotification('Failed to delete key', 'error');
            } finally {
                this.deleting = null;
            }
        },

        async editApiKey(apiKey) {
            this.editingKey = apiKey.key;
            this.formData = {
                key: apiKey.key,
                value: '', // Will be fetched from API
                description: apiKey.description || ''
            };

            // Fetch the full (unmasked) value from the API
            try {
                const response = await fetch(`/api/kv/${encodeURIComponent(apiKey.key)}`);
                if (!response.ok) {
                    throw new Error(`HTTP error! status: ${response.status}`);
                }
                const data = await response.json();
                this.formData.value = data.value;
                this.showEditModal = true;
            } catch (error) {
                console.error('Failed to fetch key value:', error);
                window.showNotification('Failed to fetch key value', 'error');
            }
        },

        async saveApiKey() {
            this.saving = true;
            try {
                const url = this.editingKey
                    ? `/api/kv/${encodeURIComponent(this.editingKey)}`
                    : '/api/kv';

                const method = this.editingKey ? 'PUT' : 'POST';

                const response = await fetch(url, {
                    method: method,
                    headers: {
                        'Content-Type': 'application/json'
                    },
                    body: JSON.stringify({
                        key: this.formData.key,
                        value: this.formData.value,
                        description: this.formData.description
                    })
                });

                if (!response.ok) {
                    throw new Error(`HTTP error! status: ${response.status}`);
                }

                window.showNotification(`Key ${this.editingKey ? 'updated' : 'created'} successfully`, 'success');
                this.closeModals();
                await this.loadApiKeys();
            } catch (error) {
                console.error('Failed to save key:', error);
                window.showNotification('Failed to save key', 'error');
            } finally {
                this.saving = false;
            }
        },

        closeModals() {
            this.showCreateModal = false;
            this.showEditModal = false;
            this.editingKey = null;
            this.formData = { key: '', value: '', description: '' };
        },

        getDescription(apiKey) {
            return apiKey.description || '-';
        }
    }));

    // System Logs Component
    Alpine.data('settingsLogs', () => ({
        logFiles: [],
        selectedFile: '',
        logs: [],
        isLoading: false,
        filters: {
            debug: false,
            info: false,
            warn: true,
            error: true
        },

        init() {
            this.loadLogFiles();
        },

        async loadLogFiles() {
            try {
                const response = await fetch('/api/system/logs/files');
                if (!response.ok) throw new Error('Failed to load log files');

                this.logFiles = await response.json();

                // Select first file if available and none selected
                if (this.logFiles.length > 0 && !this.selectedFile) {
                    this.selectedFile = this.logFiles[0].name;
                    this.loadLogs();
                }
            } catch (error) {
                console.error('Error loading log files:', error);
                if (window.showNotification) window.showNotification('Failed to load log files', 'error');
            }
        },

        async loadLogs() {
            if (!this.selectedFile) return;

            this.isLoading = true;
            try {
                // Build query params
                const params = new URLSearchParams({
                    filename: this.selectedFile,
                    limit: 1000
                });

                // Add level filters
                const activeLevels = [];
                if (this.filters.debug) activeLevels.push('debug');
                if (this.filters.info) activeLevels.push('info');
                if (this.filters.warn) activeLevels.push('warn');
                if (this.filters.error) activeLevels.push('error');

                if (activeLevels.length > 0) {
                    params.append('levels', activeLevels.join(','));
                }

                const response = await fetch(`/api/system/logs/content?${params.toString()}`);
                if (!response.ok) throw new Error('Failed to load logs');

                const data = await response.json();
                this.logs = Array.isArray(data) ? data : [];

                // Always scroll to bottom after loading
                this.$nextTick(() => {
                    if (this.$refs.logsContainer) {
                        this.$refs.logsContainer.scrollTop = this.$refs.logsContainer.scrollHeight;
                    }
                });

            } catch (error) {
                console.error('Error loading logs:', error);
                if (window.showNotification) window.showNotification('Failed to load logs', 'error');
            } finally {
                this.isLoading = false;
            }
        },

        toggleFilter(level) {
            this.filters[level] = !this.filters[level];
            this.loadLogs();
        },

        clearLogs() {
            this.logs = [];
        },

        formatSize(bytes) {
            if (bytes === 0) return '0 B';
            const k = 1024;
            const sizes = ['B', 'KB', 'MB', 'GB'];
            const i = Math.floor(Math.log(bytes) / Math.log(k));
            return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
        },

        formatTime(timestamp) {
            if (!timestamp) return '';
            // If timestamp is just time string "15:04:05", return as is
            if (typeof timestamp === 'string' && /^\d{2}:\d{2}:\d{2}$/.test(timestamp)) {
                return timestamp;
            }

            // Otherwise format full date
            try {
                const date = new Date(timestamp);
                if (isNaN(date.getTime())) return timestamp;
                return date.toLocaleTimeString('en-US', {
                    hour12: false,
                    hour: '2-digit',
                    minute: '2-digit',
                    second: '2-digit'
                });
            } catch (e) {
                return timestamp;
            }
        },

        getLevelClass(level) {
            if (!level) return 'text-gray';
            const levelUpper = level.toUpperCase();
            if (levelUpper === 'ERR' || levelUpper === 'ERROR') return 'text-error';
            if (levelUpper === 'WRN' || levelUpper === 'WARN' || levelUpper === 'WARNING') return 'text-warning';
            if (levelUpper === 'INF' || levelUpper === 'INFO') return 'text-primary';
            if (levelUpper === 'DBG' || levelUpper === 'DEBUG') return 'text-gray';
            return 'text-gray';
        }
    }));


    // Settings Danger Zone Component
    Alpine.data('settingsDanger', () => ({
        confirmDeleteAllDocuments() {
            const confirmed = confirm(
                '⚠ WARNING: This will delete ALL documents from the collection database.\n\n' +
                'This will remove all indexed content from Jira and Confluence.\n' +
                'Source data (Jira projects/issues and Confluence spaces/pages) will remain and can be re-synced.\n\n' +
                'Continue?'
            );

            if (!confirmed) return;

            fetch('/api/documents/clear-all', {
                method: 'DELETE'
            })
                .then(response => {
                    if (!response.ok) {
                        return response.json().catch(() => ({ error: 'Failed to clear documents' }));
                    }
                    return response.json();
                })
                .then(result => {
                    window.showNotification(`Success: ${result.message}\n\nDocuments deleted: ${result.documents_affected}`, 'success');
                })
                .catch(error => {
                    console.error('Error clearing documents:', error);
                    window.showNotification('Failed to clear documents: ' + error.message, 'error');
                });
        }
    }));

    logger.debug('Bootstrap', 'Settings components initialization completed');
});
