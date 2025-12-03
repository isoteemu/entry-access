class PingMonitor {
    constructor(serverUrl = '/api/v1/health', failureThreshold = 3) {
        this.serverUrl = serverUrl;
        this.failureThreshold = failureThreshold;
        this.consecutiveFailures = 0;
        this.status = 'ok';
        this.isRunning = false;
        this.interval = 2000;
        this.intervalId = null;
        this.listeners = [];
    }

    // Add event listener for status changes
    onStatusChange(callback) {
        this.listeners.push(callback);
    }

    // Notify all listeners of status change
    notifyStatusChange(oldStatus, newStatus) {
        this.listeners.forEach(callback => {
            callback({ oldStatus, newStatus, failures: this.consecutiveFailures });
        });
    }

    // Perform a single ping
    async ping() {
        const pingMsg = Math.random().toString(36).substring(2);
        const url = this.serverUrl + `?ping=${pingMsg}`;

        // Create AbortController for timeout
        const controller = new AbortController();
        const timeoutId = setTimeout(() => controller.abort(), 1000);

        try {
            const response = await fetch(url, {
                method: 'GET',
                signal: controller.signal,
                headers: {
                    'Cache-Control': 'no-cache'
                }
            });
            
            clearTimeout(timeoutId);
            
            if (response.ok) {
                this.handleSuccess();
            } else {
                console.error('Ping failed:', response.status);
                this.handleFailure(`HTTP ${response.status}: ${response.statusText}`);
            }
        } catch (error) {
            clearTimeout(timeoutId);
            console.error('Ping error:', error);
            if (error.name === 'AbortError') {
                this.handleFailure('Request timeout');
            } else {
                this.handleFailure(error.message);
            }
        }
    }

    // Handle successful ping
    handleSuccess() {
        const oldStatus = this.status;
        this.consecutiveFailures = 0;
        this.status = 'ok';
        
        if (oldStatus !== 'ok') {
            console.log('Server connection restored');
            this.notifyStatusChange(oldStatus, this.status);
        }
    }

    // Handle failed ping
    handleFailure(errorMessage) {
        const oldStatus = this.status;
        this.consecutiveFailures++;

        console.warn(`Ping failed (${this.consecutiveFailures}/${this.failureThreshold}): ${errorMessage}`);

        if (this.consecutiveFailures === 1) {
            this.status = 'warn';
        } else if (this.consecutiveFailures >= this.failureThreshold) {
            this.status = 'error';
        }

        if (oldStatus !== this.status) {
            this.notifyStatusChange(oldStatus, this.status);
        }
    }

    // Start pinging every second
    start() {
        if (this.isRunning) {
            console.warn('Ping monitor is already running');
            return;
        }

        console.log(`Starting ping monitor for ${this.serverUrl}`);
        this.isRunning = true;

        // Initial ping
        this.ping();

        // Set up interval for subsequent pings
        this.intervalId = setInterval(() => {
            this.ping();
        }, this.interval);
    }

    // Stop pinging
    stop() {
        if (!this.isRunning) {
            console.warn('Ping monitor is not running');
            return;
        }

        console.log('Stopping ping monitor');
        this.isRunning = false;
        
        if (this.intervalId) {
            clearInterval(this.intervalId);
            this.intervalId = null;
        }
    }

    // Get current status
    getStatus() {
        return {
            status: this.status,
            consecutiveFailures: this.consecutiveFailures,
            isRunning: this.isRunning
        };
    }
}

export { PingMonitor };