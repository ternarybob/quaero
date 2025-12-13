/**
 * WebSocket Manager - Singleton with exponential backoff reconnection.
 */

const WebSocketManager = (() => {
    let instance = null;

    const INITIAL_RECONNECT_DELAY = 1000;
    const MAX_RECONNECT_DELAY = 30000;
    const MAX_RECONNECT_ATTEMPTS = 50;

    class WSManager {
        constructor() {
            if (instance) return instance;

            this.ws = null;
            this.reconnectInterval = null;
            this.reconnectAttempts = 0;
            this.subscribers = {};
            this.isConnected = false;
            this.connectTimeout = null;
            this.healthCheckInterval = null;
            this.serverInstanceId = null; // Track server instance to detect restarts

            this.connect();
            this.startHealthCheck();
            instance = this;
        }

        connect() {
            if (this.connectTimeout) {
                clearTimeout(this.connectTimeout);
                this.connectTimeout = null;
            }

            if (this.ws) {
                if (this.ws.readyState === WebSocket.OPEN) return;
                if (this.ws.readyState === WebSocket.CONNECTING) return;
                try { this.ws.close(); } catch (e) {}
            }

            const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
            const wsUrl = protocol + '//' + window.location.host + '/ws';
            console.log('[WSManager] Connecting to', wsUrl);

            try {
                this.ws = new WebSocket(wsUrl);
            } catch (error) {
                this.isConnected = false;
                this.notifyConnectionChange(false);
                this.scheduleReconnect();
                return;
            }

            this.connectTimeout = setTimeout(() => {
                if (this.ws && this.ws.readyState === WebSocket.CONNECTING) {
                    this.ws.close();
                    this.isConnected = false;
                    this.notifyConnectionChange(false);
                    this.scheduleReconnect();
                }
            }, 10000);

            this.ws.onopen = () => {
                console.log('[WSManager] Connected');
                clearTimeout(this.connectTimeout);
                this.connectTimeout = null;
                this.isConnected = true;
                this.reconnectAttempts = 0;
                this.clearReconnectTimer();
                this.notifyConnectionChange(true);
            };

            this.ws.onmessage = (event) => {
                try {
                    const message = JSON.parse(event.data);
                    this.handleMessage(message);
                } catch (e) {}
            };

            this.ws.onerror = () => {
                this.isConnected = false;
                this.notifyConnectionChange(false);
            };

            this.ws.onclose = (event) => {
                console.log('[WSManager] Disconnected, code:', event.code);
                clearTimeout(this.connectTimeout);
                this.connectTimeout = null;
                this.isConnected = false;
                this.notifyConnectionChange(false);
                this.scheduleReconnect();
            };
        }

        handleMessage(message) {
            const { type, payload } = message;
            if (!type) return;

            // Check for server restart via status message with serverInstanceId
            if (type === 'status' && payload && payload.serverInstanceId) {
                const newInstanceId = payload.serverInstanceId;
                if (this.serverInstanceId !== null && this.serverInstanceId !== newInstanceId) {
                    // Server has restarted - notify subscribers to clear state
                    console.log('[WSManager] Server restart detected (instance changed from', this.serverInstanceId, 'to', newInstanceId + ')');
                    if (this.subscribers['_server_restart']) {
                        this.subscribers['_server_restart'].forEach(cb => { try { cb({ oldInstanceId: this.serverInstanceId, newInstanceId }); } catch (e) {} });
                    }
                }
                this.serverInstanceId = newInstanceId;
            }

            if (this.subscribers[type]) {
                this.subscribers[type].forEach(cb => { try { cb(payload); } catch (e) {} });
            }
        }

        // Subscribe to server restart events - callback receives { oldInstanceId, newInstanceId }
        onServerRestart(callback) {
            return this.subscribe('_server_restart', callback);
        }

        subscribe(messageType, callback) {
            if (!this.subscribers[messageType]) this.subscribers[messageType] = [];
            this.subscribers[messageType].push(callback);
            return () => this.unsubscribe(messageType, callback);
        }

        unsubscribe(messageType, callback) {
            if (!this.subscribers[messageType]) return;
            this.subscribers[messageType] = this.subscribers[messageType].filter(cb => cb !== callback);
            if (this.subscribers[messageType].length === 0) delete this.subscribers[messageType];
        }

        onConnectionChange(callback) {
            const unsub = this.subscribe('_connection', callback);
            try { callback(this.isConnected); } catch (e) {}
            return unsub;
        }

        notifyConnectionChange(isConnected) {
            if (this.subscribers['_connection']) {
                this.subscribers['_connection'].forEach(cb => { try { cb(isConnected); } catch (e) {} });
            }
        }

        scheduleReconnect() {
            if (this.reconnectInterval) return;
            if (this.reconnectAttempts >= MAX_RECONNECT_ATTEMPTS) {
                console.warn('[WSManager] Max reconnection attempts reached');
                return;
            }

            const delay = Math.min(INITIAL_RECONNECT_DELAY * Math.pow(2, this.reconnectAttempts), MAX_RECONNECT_DELAY);
            this.reconnectAttempts++;
            console.log('[WSManager] Reconnect in ' + delay + 'ms (attempt ' + this.reconnectAttempts + '/' + MAX_RECONNECT_ATTEMPTS + ')');

            this.reconnectInterval = setTimeout(() => {
                this.reconnectInterval = null;
                this.connect();
            }, delay);
        }

        clearReconnectTimer() {
            if (this.reconnectInterval) {
                clearTimeout(this.reconnectInterval);
                this.reconnectInterval = null;
            }
        }

        startHealthCheck() {
            this.healthCheckInterval = setInterval(() => {
                if (this.getConnectionState() === 'disconnected' && !this.reconnectInterval && this.reconnectAttempts < MAX_RECONNECT_ATTEMPTS) {
                    this.connect();
                }
            }, 5000);
        }

        send(message) {
            if (this.ws && this.ws.readyState === WebSocket.OPEN) {
                this.ws.send(JSON.stringify(message));
            }
        }

        close() {
            this.clearReconnectTimer();
            if (this.ws) { this.ws.close(); this.ws = null; }
        }

        getConnectionStatus() { return this.isConnected; }

        getConnectionState() {
            if (!this.ws) return 'disconnected';
            switch (this.ws.readyState) {
                case WebSocket.CONNECTING: return 'connecting';
                case WebSocket.OPEN: return 'connected';
                default: return 'disconnected';
            }
        }

        resetReconnectAttempts() { this.reconnectAttempts = 0; }
        getReconnectAttempts() { return this.reconnectAttempts; }
    }

    return new WSManager();
})();

window.WebSocketManager = WebSocketManager;
