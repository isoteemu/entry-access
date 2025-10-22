import {parseJWT} from "./jwt.js";

class DeviceProvisioning {
    constructor() {
        // Key for storing device ID in local storage
        this.DEVICE_ID_KEY = 'device_id';
        this.DEVICE_AUTH_KEY = 'device_auth';
    }

    /**
     * Generates a unique device ID using UUID v4 format
     * @returns {string} A unique device ID
     */
    generateDeviceId() {
        return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function(c) {
            const r = Math.random() * 16 | 0;
            const v = c == 'x' ? r : (r & 0x3 | 0x8);
            return v.toString(16);
        });
    }

    /**
     * Gets the device ID from local storage or creates a new one if it doesn't exist
     * @returns {string} The device ID
     */
    getOrCreateDeviceId() {
        try {
            // Check if device_id already exists in local storage
            let deviceId = localStorage.getItem(this.DEVICE_ID_KEY);
            
            if (!deviceId) {
                // Generate new device ID if it doesn't exist
                deviceId = this.generateDeviceId();
                
                // Store the new device ID in local storage
                localStorage.setItem(this.DEVICE_ID_KEY, deviceId);
                
                console.log('New device ID created and stored:', deviceId);
            } else {
                console.log('Existing device ID found:', deviceId);
            }
            
            return deviceId;
        } catch (error) {
            console.error('Error managing device ID:', error);
            // Panic!
            throw new Error('Failed to manage device ID');
        }
    }

    /**
     * Gets the current device ID from local storage
     * @returns {string|null} The device ID or null if not found
     */
    getDeviceId() {
        try {
            return localStorage.getItem(this.DEVICE_ID_KEY);
        } catch (error) {
            console.error('Error retrieving device ID:', error);
            return null;
        }
    }

    /**
     * Removes the device ID from local storage (useful for testing or reset)
     */
    clearDeviceId() {
        try {
            localStorage.removeItem(this.DEVICE_ID_KEY);
            console.log('Device ID cleared from local storage');
        } catch (error) {
            console.error('Error clearing device ID:', error);
        }
    }

    /**
     * Stores the device authorization JWT in local storage
     * @param {string} jwt - The JWT token received from the server
     */
    storeAuthToken(jwt) {
        try {
            localStorage.setItem(this.DEVICE_AUTH_KEY, jwt);
            console.log('Device authorization token stored');
        } catch (error) {
            console.error('Error storing auth token:', error);
        }
    }

    /**
     * Gets the current device authorization JWT from local storage
     * @returns {string|null} The JWT token or null if not found
     */
    getAuthToken() {
        try {
            return localStorage.getItem(this.DEVICE_AUTH_KEY);
        } catch (error) {
            console.error('Error retrieving auth token:', error);
            return null;
        }
    }

    /**
     * Checks if the device is authorized (has a valid JWT)
     * @returns {boolean} True if device has an auth token
     */
    isAuthorized() {
        const token = this.getAuthToken();
        if (!token) return false;

        try {
            // Parse JWT to check expiration
            const payload = parseJWT(token);
            const now = Math.floor(Date.now() / 1000);
            
            // Check if token is expired
            if (payload.exp && payload.exp < now) {
                console.log('Auth token expired');
                this.clearAuthToken();
                return false;
            }
            
            return true;
        } catch (error) {
            console.error('Error checking auth token:', error);
            this.clearAuthToken();
            return false;
        }
    }

    /**
     * Checks if the auth token needs refresh (less than 1 day until expiration)
     * @returns {boolean} True if token needs refresh
     */
    needsRefresh() {
        const token = this.getAuthToken();
        if (!token) return false;

        try {
            const payload = parseJWT(token);
            const now = Math.floor(Date.now() / 1000);
            const oneDayInSeconds = 24 * 60 * 60;
            
            // Check if token expires within 1 day
            if (payload.exp && (payload.exp - now) < oneDayInSeconds) {
                console.log('Auth token needs refresh');
                return true;
            }
            
            return false;
        } catch (error) {
            console.error('Error checking token refresh need:', error);
            return false;
        }
    }

    /**
     * Removes the auth token from local storage
     */
    clearAuthToken() {
        try {
            localStorage.removeItem(this.DEVICE_AUTH_KEY);
            console.log('Device auth token cleared from local storage');
        } catch (error) {
            console.error('Error clearing auth token:', error);
        }
    }

    /**
     * Polls the server to check if device has been authorized
     * @returns {Promise<boolean>} True if device is now authorized
     */
    async pollForAuthorization() {
        const deviceId = this.getDeviceId();
        if (!deviceId) {
            console.error('No device ID found for polling');
            return false;
        }

        try {
            const response = await fetch('/api/device/status', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({
                    device_id: deviceId
                })
            });

            if (response.ok) {
                const data = await response.json();
                
                if (data.authorized && data.jwt) {
                    this.storeAuthToken(data.jwt);
                    console.log('Device authorized successfully');
                    return true;
                }
            }
            
            return false;
        } catch (error) {
            console.error('Error polling for authorization:', error);
            return false;
        }
    }

    /**
     * Requests a refresh of the current auth token
     * @returns {Promise<boolean>} True if refresh was successful
     */
    async refreshAuthToken() {
        const deviceId = this.getDeviceId();
        const currentToken = this.getAuthToken();
        
        if (!deviceId || !currentToken) {
            console.error('Missing device ID or auth token for refresh');
            return false;
        }

        try {
            const response = await fetch('/api/device/refresh', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${currentToken}`
                },
                body: JSON.stringify({
                    device_id: deviceId
                })
            });

            if (response.ok) {
                const data = await response.json();
                
                if (data.jwt) {
                    this.storeAuthToken(data.jwt);
                    console.log('Auth token refreshed successfully');
                    return true;
                }
            }
            
            return false;
        } catch (error) {
            console.error('Error refreshing auth token:', error);
            return false;
        }
    }

    /**
     * Initializes device provisioning - main entry point
     * @returns {string} The device ID
     */
    initialize() {
        console.log('Initializing device provisioning...');
        const deviceId = this.getOrCreateDeviceId();
        console.log('Device provisioning completed. Device ID:', deviceId);
        return deviceId;
    }
}

/**
 * Loads configuration data with caching
 * Loads config once per page load, caching it in memory and local storage.
 * If load fails, falls back to local storage cache if available.
 */
let configCache = null; // Module-level cache for current page load

async function loadConfig() {
    // Return cached config if already loaded during this page session
    if (configCache !== null) {
        console.log('Returning cached config from memory');
        return configCache;
    }

    const cacheKey = 'app_config_cache_v1';

    try {
        const response = fetch('/config.json', {cache: 'no-store'})
            .then(res => {
                if (!res.ok) {
                    throw new Error(`HTTP error! status: ${res.status}`);
                }
                return res.json();
            })
            .then(configData => {
                // Cache in local storage
                localStorage.setItem(cacheKey, JSON.stringify(configData));
                // Cache in memory for this page load
                configCache = configData;
                console.log('Config loaded and cached');
                return configData;
            });
        
        return await response;
    } catch (error) {
        console.error('Config fetch failed, attempting to load from cache:', error);
        // Fallback to local storage cache
        const cachedConfig = localStorage.getItem(cacheKey);
        if (cachedConfig) {
            try {
                configCache = JSON.parse(cachedConfig);
                console.log('Config loaded from localStorage cache');
                return configCache;
            } catch (parseError) {
                console.error('Failed to parse cached config:', parseError);
                localStorage.removeItem(cacheKey);
                throw parseError;
            }
        } else {
            console.warn('No cached config found.');
            throw error;
        }
    }
}

// TODO: Add version check function
function checkVersion() {
    // Fetch version, and reload if mismatch
}

export {
    DeviceProvisioning,
    loadConfig
};
