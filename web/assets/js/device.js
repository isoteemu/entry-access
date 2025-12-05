import { instance as errorHandler } from './error.js';

class DeviceProvisioning {
    constructor() {
        // Key for storing device ID in local storage
        this.DEVICE_ID_KEY = 'device_id';
        this.DEVICE_AUTH_KEY = 'device_auth';

        this.errorHandler = errorHandler;
        this.device_id = null;
        this.authenticated = false;
    }

    /**
     * Handles API errors and displays them using the error handler
     * @param {Response} response - Fetch response object
     * @param {string} context - Context description for the error
     */
    async handleApiError(response, context) {
        let errorData = null;
        try {
            errorData = await response.json();
        } catch (e) {
            // Response is not JSON, use simple error
            errorData = {
                message: `${context} failed with status ${response.status}`,
                status: 'error'
            };
        }

        // Check if it's an errorStruct from backend
        if (errorData && errorData.success === false) {
            const errorId = errorData.code && errorData.code.length > 0 
                ? errorData.code.join('_').toLowerCase() 
                : 'provisioning_error';
            
            this.errorHandler.addError(errorId, {
                title: `${context} Failed`,
                message: errorData.message || 'An unexpected error occurred',
                code: errorData.code ? errorData.code.join(', ') : null,
                type: 'error'
            });
            
            throw new Error(errorData.message || `${context} failed`);
        } else {
            // Simple error without errorStruct
            const message = errorData?.message || `${context} failed with status ${response.status}`;
            
            this.errorHandler.addError('device_error', {
                title: `${context} Failed`,
                message: message,
                type: 'error'
            });
            
            throw new Error(message);
        }
    }

    /**
     * Registers device with the server using /register endpoint
     * @param {string|null} existingDeviceId - Optional existing device ID
     * @returns {Promise<Object>} Registration response with device_id and status
     */
    async registerDevice(existingDeviceId = null) {
        try {
            const payload = existingDeviceId ? { device_id: existingDeviceId } : {};

            const response = await fetch('/api/provision/register', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Accept': 'application/json'
                },
                body: JSON.stringify(payload)
            });

            if (!response.ok) {
                await this.handleApiError(response, 'Device registration');
            }

            const data = await response.json();
            
            // Update instance properties
            if (data.device_id) {
                this.device_id = data.device_id;
                localStorage.setItem(this.DEVICE_ID_KEY, data.device_id);
            }
            this.authenticated = data.authenticated || false;

            console.log('Device registration response:', data);
            return data;
        } catch (error) {
            console.error('Error registering device:', error);
            throw error;
        }
    }

    /**
     * Gets the current device ID from instance or local storage
     * @returns {string|null} The device ID or null if not found
     */
    getDeviceId() {
        if (this.device_id) {
            return this.device_id;
        }
        try {
            return localStorage.getItem(this.DEVICE_ID_KEY);
        } catch (error) {
            console.error('Error retrieving device ID:', error);
            return null;
        }
    }

    getAuthenticated() {
        return (this.authenticated === true);
    }

    /**
     * Removes the device ID from local storage and resets instance (useful for testing or reset)
     */
    clearDeviceId() {
        try {
            localStorage.removeItem(this.DEVICE_ID_KEY);
            this.device_id = null;
            this.authenticated = false;
            console.log('Device ID cleared from local storage');
        } catch (error) {
            console.error('Error clearing device ID:', error);
        }
    }

    /**
     * Initializes device provisioning - main entry point
     * Registers device with server and sets device_id and authenticated properties
     * @returns {Promise<string>} The device ID
     */
    async initialize() {
        console.log('Initializing device provisioning...');
        try {
            // Get existing device ID from local storage if it exists
            const existingDeviceId = localStorage.getItem(this.DEVICE_ID_KEY);
            
            // Register with server (creates new or checks existing device)
            const response = await this.registerDevice(existingDeviceId);
            
            // Set properties from response
            this.device_id = response.device_id;
            this.authenticated = response.authenticated || false;
            
            console.log('Device provisioning completed:', {
                device_id: this.device_id,
                authenticated: this.authenticated,
                status: response.status
            });
            
            return this.device_id;
        } catch (error) {
            console.error('Device provisioning failed:', error);
            this.errorHandler.addError('provisioning_failed', {
                title: 'Provisioning Failed',
                message: 'Unable to initialize device provisioning. Please try again.',
                type: 'error'
            });
            throw error;
        }
    }
}

// Prevent device from sleeping using NoSleep.js
// Notice: NoSleep.js requires a user interaction to activate
function initNoSleep() {
    const noSleep = new window.NoSleep();
    
    // Need a user interaction to enable NoSleep
    function enableNoSleep() {
        noSleep.enable();
        console.log('NoSleep enabled to prevent device sleep.');
        document.removeEventListener('click', enableNoSleep, false);
    }
    document.addEventListener('click', enableNoSleep, false);
}

document.addEventListener('DOMContentLoaded', async () => {
    // Check if NoSleep.js is loaded
    if (typeof window.NoSleep === 'undefined') {
        // Load NoSleep.js dynamically
        const script = document.createElement('script');
        script.src = 'dist/vendor/js/nosleep.min.js';
        script.onload = initNoSleep;
        document.head.appendChild(script);
    } else {
        initNoSleep();
        return;
    }
});

export { DeviceProvisioning };
