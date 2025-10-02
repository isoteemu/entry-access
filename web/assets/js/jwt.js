/**
 * Basic JWT parsing module
 * Provides functions to decode and parse JWT tokens
 */

/**
 * Base64 URL decode function
 * @param {string} str - Base64 URL encoded string
 * @returns {string} - Decoded string
 */
function base64UrlDecode(str) {
    // Replace URL-safe characters with standard base64 characters
    str = str.replace(/-/g, '+').replace(/_/g, '/');
    
    // Add padding if needed
    while (str.length % 4) {
        str += '=';
    }
    
    // Decode base64 and return as UTF-8 string
    return decodeURIComponent(escape(atob(str)));
}

/**
 * Parse JWT token and extract header, payload, and signature
 * @param {string} token - JWT token string
 * @returns {object} - Object containing header, payload, and signature
 */
function parseJWT(token) {
    if (!token || typeof token !== 'string') {
        throw new Error('Invalid token: must be a non-empty string');
    }

    const parts = token.split('.');

    if (parts.length !== 3) {
        throw new Error('Invalid JWT: must have exactly 3 parts separated by dots');
    }
    
    try {
        const header = JSON.parse(base64UrlDecode(parts[0]));
        const payload = JSON.parse(base64UrlDecode(parts[1]));
        const signature = parts[2];
        
        return {
            header,
            payload,
            signature,
            raw: {
                header: parts[0],
                payload: parts[1],
                signature: parts[2]
            }
        };
    } catch (error) {
        throw new Error('Invalid JWT: failed to parse token - ' + error.message);
    }
}

/**
 * Check if JWT token is expired
 * @param {object} payload - JWT payload object
 * @returns {boolean} - True if token is expired, false otherwise
 */
function isTokenExpired(payload) {
    if (!payload.exp) {
        return false; // No expiration time set
    }
    
    const currentTime = Math.floor(Date.now() / 1000);
    return payload.exp < currentTime;
}

/**
 * Get token expiration date
 * @param {object} payload - JWT payload object
 * @returns {Date|null} - Expiration date or null if no expiration set
 */
function getTokenExpiration(payload) {
    if (!payload.exp) {
        return null;
    }
    
    return new Date(payload.exp * 1000);
}

/**
 * Extract basic token information
 * @param {string} token - JWT token string
 * @returns {object} - Object with basic token information
 */
function getTokenInfo(token) {
    const parsed = parseJWT(token);
    
    return {
        algorithm: parsed.header.alg,
        type: parsed.header.typ,
        issuer: parsed.payload.iss,
        subject: parsed.payload.sub,
        audience: parsed.payload.aud,
        issuedAt: parsed.payload.iat ? new Date(parsed.payload.iat * 1000) : null,
        expiresAt: getTokenExpiration(parsed.payload),
        isExpired: isTokenExpired(parsed.payload)
    };
}

// ES6 module exports
export {
    parseJWT,
    isTokenExpired,
    getTokenExpiration,
    getTokenInfo,
    base64UrlDecode
};
