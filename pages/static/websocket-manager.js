/**
 * WebSocket Manager - Singleton pattern for managing a single WebSocket connection
 * across multiple components on the page.
 * 
 * Usage:
 *   WebSocketManager.subscribe('status', (data) => { ... });
 *   WebSocketManager.subscribe('log', (data) => { ... });
 *   WebSocketManager.unsubscribe('status', handler);
 */

const WebSocketManager = (() => {
    let instance = null;
    
    class WSManager {
        constructor() {
            if (instance) {
                return instance;
            }
            
            this.ws = null;
            this.reconnectInterval = null;
            this.reconnectDelay = 3000;
            this.subscribers = {};
            this.isConnected = false;
            this.connectTimeout = null;
            this.healthCheckInterval = null;
            
            // Auto-connect on instantiation
            this.connect();
            
            // Start health check to monitor connection state
            this.startHealthCheck();
            
            instance = this;
        }
        
        /**
         * Establish WebSocket connection
         */
        connect() {
            // Clear any existing connection timeout
            if (this.connectTimeout) {
                clearTimeout(this.connectTimeout);
                this.connectTimeout = null;
            }
            
            // If already connected or connecting, close existing connection first
            if (this.ws) {
                if (this.ws.readyState === WebSocket.OPEN) {
                    console.log('[WSManager] Already connected');
                    return;
                }
                if (this.ws.readyState === WebSocket.CONNECTING) {
                    console.log('[WSManager] Already connecting, readyState:', this.ws.readyState);
                    return;
                }
                // Close any existing connection
                try {
                    this.ws.close();
                } catch (e) {
                    console.warn('[WSManager] Error closing existing connection:', e);
                }
            }
            
            const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
            const wsUrl = `${protocol}//${window.location.host}/ws`;
            
            console.log('[WSManager] Connecting to', wsUrl);
            
            try {
                this.ws = new WebSocket(wsUrl);
            } catch (error) {
                console.error('[WSManager] Failed to create WebSocket:', error);
                this.isConnected = false;
                this.notifyConnectionChange(false);
                this.scheduleReconnect();
                return;
            }
            
            // Set timeout for connection attempt (10 seconds)
            this.connectTimeout = setTimeout(() => {
                if (this.ws && this.ws.readyState === WebSocket.CONNECTING) {
                    console.error('[WSManager] Connection timeout - stuck in CONNECTING state');
                    this.ws.close();
                    this.isConnected = false;
                    this.notifyConnectionChange(false);
                    this.scheduleReconnect();
                }
            }, 10000);
            
            this.ws.onopen = () => {
                console.log('[WSManager] ✅ Connected successfully');
                clearTimeout(this.connectTimeout);
                this.connectTimeout = null;
                this.isConnected = true;
                this.clearReconnectTimer();
                this.notifyConnectionChange(true);
            };
            
            this.ws.onmessage = (event) => {
                try {
                    const message = JSON.parse(event.data);
                    console.log('[WSManager] Message received, type:', message.type);
                    this.handleMessage(message);
                } catch (error) {
                    console.error('[WSManager] Failed to parse message:', error, 'Data:', event.data);
                }
            };
            
            this.ws.onerror = (error) => {
                console.error('[WSManager] ❌ WebSocket ERROR:', error);
                console.error('[WSManager] WebSocket readyState:', this.ws ? this.ws.readyState : 'null');
                this.isConnected = false;
                this.notifyConnectionChange(false);
            };
            
            this.ws.onclose = (event) => {
                console.log('[WSManager] Disconnected, code:', event.code, 'reason:', event.reason);
                clearTimeout(this.connectTimeout);
                this.connectTimeout = null;
                this.isConnected = false;
                this.notifyConnectionChange(false);
                this.scheduleReconnect();
            };
        }
        
        /**
         * Handle incoming WebSocket message
         */
        handleMessage(message) {
            const { type, payload } = message;
            
            if (!type) {
                console.warn('[WSManager] Message missing type:', message);
                return;
            }
            
            // Notify all subscribers for this message type
            if (this.subscribers[type]) {
                this.subscribers[type].forEach(callback => {
                    try {
                        callback(payload);
                    } catch (error) {
                        console.error(`[WSManager] Subscriber error for type '${type}':`, error);
                    }
                });
            }
        }
        
        /**
         * Subscribe to a specific message type
         * @param {string} messageType - The message type to listen for (e.g., 'status', 'log', 'auth')
         * @param {function} callback - Function to call when message is received
         * @returns {function} Unsubscribe function
         */
        subscribe(messageType, callback) {
            if (!this.subscribers[messageType]) {
                this.subscribers[messageType] = [];
            }
            
            this.subscribers[messageType].push(callback);
            console.log(`[WSManager] Subscribed to '${messageType}' (${this.subscribers[messageType].length} total)`);
            
            // Return unsubscribe function
            return () => this.unsubscribe(messageType, callback);
        }
        
        /**
         * Unsubscribe from a specific message type
         * @param {string} messageType - The message type
         * @param {function} callback - The callback function to remove
         */
        unsubscribe(messageType, callback) {
            if (!this.subscribers[messageType]) {
                return;
            }
            
            this.subscribers[messageType] = this.subscribers[messageType].filter(cb => cb !== callback);
            console.log(`[WSManager] Unsubscribed from '${messageType}' (${this.subscribers[messageType].length} remaining)`);
            
            // Clean up empty arrays
            if (this.subscribers[messageType].length === 0) {
                delete this.subscribers[messageType];
            }
        }
        
        /**
         * Subscribe to connection state changes
         * @param {function} callback - Called with boolean (true = connected, false = disconnected)
         * @returns {function} Unsubscribe function
         */
        onConnectionChange(callback) {
            const unsubscribe = this.subscribe('_connection', callback);
            
            // Immediately notify subscriber of current connection state
            // This ensures they don't miss the initial connection event
            try {
                callback(this.isConnected);
            } catch (error) {
                console.error('[WSManager] Error in immediate connection callback:', error);
            }
            
            return unsubscribe;
        }
        
        /**
         * Notify connection state change subscribers
         */
        notifyConnectionChange(isConnected) {
            if (this.subscribers['_connection']) {
                this.subscribers['_connection'].forEach(callback => {
                    try {
                        callback(isConnected);
                    } catch (error) {
                        console.error('[WSManager] Connection change subscriber error:', error);
                    }
                });
            }
        }
        
        /**
         * Schedule reconnection attempt
         */
        scheduleReconnect() {
            if (this.reconnectInterval) {
                return; // Already scheduled
            }
            
            console.log(`[WSManager] Will reconnect in ${this.reconnectDelay}ms...`);
            this.reconnectInterval = setTimeout(() => {
                this.reconnectInterval = null;
                console.log('[WSManager] Attempting reconnection...');
                this.connect();
            }, this.reconnectDelay);
        }
        
        /**
         * Clear reconnection timer
         */
        clearReconnectTimer() {
            if (this.reconnectInterval) {
                clearTimeout(this.reconnectInterval);
                this.reconnectInterval = null;
            }
        }
        
        /**
         * Start periodic health check to monitor connection state
         */
        startHealthCheck() {
            // Check connection health every 5 seconds
            this.healthCheckInterval = setInterval(() => {
                const state = this.getConnectionState();
                
                // If stuck in connecting state for too long, the timeout will handle it
                // If disconnected and not reconnecting, attempt reconnection
                if (state === 'disconnected' && !this.reconnectInterval) {
                    console.log('[WSManager] Health check: Disconnected, attempting reconnect');
                    this.connect();
                }
            }, 5000);
        }
        
        /**
         * Send a message through the WebSocket
         * @param {object} message - The message to send
         */
        send(message) {
            if (this.ws && this.ws.readyState === WebSocket.OPEN) {
                this.ws.send(JSON.stringify(message));
            } else {
                console.warn('[WSManager] Cannot send message, not connected');
            }
        }
        
        /**
         * Close the WebSocket connection
         */
        close() {
            this.clearReconnectTimer();
            if (this.ws) {
                this.ws.close();
                this.ws = null;
            }
        }
        
        /**
         * Get connection status
         */
        getConnectionStatus() {
            return this.isConnected;
        }
        
        /**
         * Get detailed connection state
         * @returns {string} 'connected', 'connecting', 'disconnected'
         */
        getConnectionState() {
            if (!this.ws) {
                return 'disconnected';
            }
            
            switch (this.ws.readyState) {
                case WebSocket.CONNECTING:
                    return 'connecting';
                case WebSocket.OPEN:
                    return 'connected';
                case WebSocket.CLOSING:
                case WebSocket.CLOSED:
                default:
                    return 'disconnected';
            }
        }
    }
    
    // Return singleton instance
    return new WSManager();
})();

// Make available globally
window.WebSocketManager = WebSocketManager;