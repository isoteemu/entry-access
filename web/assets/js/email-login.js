// Load config.json
import { loadConfig } from '/assets/js/app.js';

const config = await loadConfig();

// Show error in server error div
function err(msg) {
    console.error(msg);
    const serverErrorDiv = document.getElementById('server-error');
    const serverErrorContent = document.getElementById('server-error-content');

    let statusMessage = msg || 'An unexpected error occurred.';
    if (config?.SupportURL) {
        statusMessage += ` Please contact support at <a href="${config.SupportURL}" target="_blank">${config.SupportURL}</a>.`;
    }

    if (serverErrorDiv && serverErrorContent) {
        serverErrorContent.innerHTML = statusMessage;
        serverErrorDiv.classList.remove('hidden');
        serverErrorDiv.scrollIntoView({ behavior: 'smooth' });
    }
}

// Using SSE to listen for authentication status updates
function listenForAuthStatus(token) {
    // Polls for authentication token via Server-Sent Events (SSE)
    const url = new URL('sse', window.location);
    url.searchParams.append('token', token);
    url.searchParams.append('cb', Date.now()); // Prevent caching

    console.log("Connecting to SSE URL:", url.toString());

    const eventSource = new EventSource(url);

    eventSource.addEventListener('open', function(event) {
        console.log('SSE connection opened:', event);
    });

    eventSource.addEventListener('error', function(event) {
        console.log('SSE connection error:', event, event.type);
        // Handle different error scenarios
        if (event.type == 'error') {
            err('SSE connection error occurred. Please try again.');
            eventSource.close();
        } else if (event.eventPhase == EventSource.CLOSED) {
            console.log('SSE connection closed by server.');
            eventSource.close();
        } else {
            console.error('SSE error:', event);
        }
    });

    // Event listener for incoming messages
    // SSE messages that start with 'data: ' are received here
    eventSource.onmessage = function(event) {
        console.log('SSE message received:', event.data);
        try {
            // Parse the JSON string sent by the server
            const data = JSON.parse(event.data);

            console.log("EventSource message received:", data);
            if (data.error) {
                err(data.error);
                eventSource.close();
                return;
            }

            if (data.status === 'confirmed') {
                eventSource.close();
                if (data.redirect) {
                    window.location.href = data.redirect;
                } else {
                    err('No redirect URL provided. Please try logging in again.');
                }
            } else if (data.status === "expired") {
                eventSource.close();
                console.log('SSE connection closed by client due to token expiration.');
                err('Login link has expired. Please request a new login link.');
            } else if (data.status === 'pending') {
                // TODO: Implement UI update for pending status
                console.log('Authentication pending...');
            }

        } catch (e) {
            console.error('Error parsing JSON:', e, 'Raw data:', event.data);
        }
    };

    return eventSource;
}

// Helper function to perform fetch with timeout
function postData(url = '', data = {}, options = {}) {
    const controller = new AbortController();

    const defaultOptions = {
        timeout: 4_000,
        method: 'POST',
        headers: {
            'Content-Type': 'application/x-www-form-urlencoded',
        },
        body: new URLSearchParams(data),
        signal: controller.signal,
    };

    options = { ...defaultOptions, ...options };

    if (options.timeout > 0) {
        setTimeout(() => controller.abort(), options.timeout);
    }

    return fetch(url, options)
        .catch(error => {
            if (error.name === 'AbortError') {
                throw new Error('Request timed out');
            }
            throw error;
        });
}

function formDataToObject(formElement) {
    const formData = new FormData(formElement);
    const data = {};
    formData.forEach((value, key) => {
        data[key] = value;
    });
    return data;
}

// Utility functions
const FormUtils = {
    setError(inputElement, message) {
        inputElement.setCustomValidity(message);
        inputElement.reportValidity();
    },

    clearError(inputElement) {
        inputElement.setCustomValidity('');
    },

    /**
     * Parse error response and return user-friendly message.
     * Handles specific status codes like 429 and 500 with custom messages.
     * Attempts to extract error message from JSON response if available.
     */
    async parseErrorResponse(response) {
        const contentType = response.headers.get('content-type') || '';
        let customMessage = null;

        // First try to parse JSON error message if available
        if (contentType.includes('application/json')) {
            try {
                const data = await response.json();
                if (data?.error) {
                    customMessage = data.error;
                }
            } catch (error) {
                // Fall through to status-based handling
                console.warn("Failed to parse JSON error response", error);
            }
        }

        // Handle specific status codes with custom message or fallback
        if (response.status === 429) {
            return customMessage || 'Too many requests. Please wait before trying again.';
        } else if (response.status === 500) {
            let statusMessage = customMessage || 'Server error occurred.';

            // Try to get support URL from config
            try {
                // TODO: Use loadConfig()
                const configResponse = await fetch('config.json');
                const config = await configResponse.json();
                if (config?.SupportURL) {
                    return `${statusMessage} Please contact support at <a href="${config.SupportURL}" target="_blank">${config.SupportURL}</a>.`;
                }
            } catch (error) {
                // Config fetch failed, use generic message
                console.warn("Failed to fetch config for support URL", error);
            }

            return `${statusMessage} Please try again later or contact support.`;
        }

        return customMessage;
    },

    startCooldown(button, seconds = 45, finalText = 'Resend email') {
        let countdown = seconds;
        button.disabled = true;
        
        const interval = setInterval(() => {
            button.textContent = `Wait ${countdown--}s...`;
            if (countdown <= 0) {
                clearInterval(interval);
                button.disabled = false;
                button.textContent = finalText;
            }
        }, 1000);
    }
};

// Run in anonymous async function
(async () => {
    // DOM elements
    const emailForm = document.getElementById('email-form');
    const otpForm = document.getElementById('otp-form');
    const emailInput = document.getElementById('email');
    const otpInput = document.getElementById('otp');
    const otpDiv = document.getElementById('otp-container');

    // New: server-side error div
    const serverErrorDiv = document.getElementById('server-error');

    // Set timezone
    const timezoneInput = document.getElementById('timezone');
    if (timezoneInput) {
        const timeZone = Intl.DateTimeFormat().resolvedOptions().timeZone;
        if (timeZone) {
            timezoneInput.value = timeZone;
        }
    }

    // Clear validation errors on input
    emailInput.addEventListener('input', () => FormUtils.clearError(emailInput));
    otpInput.addEventListener('input', () => FormUtils.clearError(otpInput));

    // Handle email form submission
    emailForm.addEventListener('submit', async function(e) {
        e.preventDefault();

        const button = this.querySelector('button[type="submit"]');
        const buttonText = button.textContent;

        // Clear previous state
        FormUtils.clearError(emailInput);
        if (serverErrorDiv) serverErrorDiv.classList.add('hidden'); // hide any server error when retrying

        // Disable button and show loading state
        button.textContent = 'Sending...';
        button.disabled = true;

        try {
            let formData = formDataToObject(emailForm);
            const response = await postData(this.action, formData);
            if (response.ok) {
                // Success
                otpDiv.classList.remove('hidden');
                FormUtils.startCooldown(button);

                const data = await response.json();
                console.log("Response data:", data);

                // Check that otpclaim exists
                if (!data?.otpclaim) {
                    throw new Error("No otpclaim received");
                }
                otpForm.querySelector('input[name="otpclaim"]').value = data.otpclaim;
                otpInput.focus();
                window.location.hash = 'OTP';
                // Start listening for auth status via SSE
                listenForAuthStatus(data.otpclaim);
            } else {
                // Handle error response
                const errorMsg = await FormUtils.parseErrorResponse(response) || 'Failed to send login link. Please try again.';
                FormUtils.setError(emailInput, errorMsg);
                button.textContent = buttonText;
                button.disabled = false;
            }
        } catch (error) {
            const errorMessage = error.message || 'Failed to send login link. Please try again.';
            console.error("Error:", error);
            button.textContent = buttonText;
            button.disabled = false;
        }
    });

    // OTP form handler
    otpForm.addEventListener('submit', async function(e) {
        e.preventDefault();

        const button = this.querySelector('button[type="submit"]');
        const otp = otpInput.value;
        
        // Clear previous state
        FormUtils.clearError(otpInput);
        if (serverErrorDiv) serverErrorDiv.classList.add('hidden'); // hide server error when retrying
        button.disabled = true;
        button.textContent = 'Verifying...';

        try {
            const data = formDataToObject(this);

            const response = await postData(this.action, data);

            if (response.ok) {
                const responseData = await response.json();
                if (responseData?.redirect) {
                    window.location.href = responseData.redirect;
                } else {
                    throw new Error("No redirect URL received");
                }
            } else {
                // Parse and show error via validation
                const customError = await FormUtils.parseErrorResponse(response) || 
                    'Failed to verify OTP. Please try again.';
                
                FormUtils.setError(otpInput, customError);
                button.disabled = false;
                button.textContent = 'Verify and proceed';
            }
        } catch (error) {
            const errorMessage = error.message || 'Failed to verify OTP. Please try again.';
            FormUtils.setError(otpInput, errorMessage);
            button.disabled = false;
            button.textContent = 'Verify and proceed';
        }
    });
    // Handle page load focus
    document.addEventListener('DOMContentLoaded', function() {
        if (location.hash === '#OTP') {
            otpForm.classList.remove('hidden');
            otpInput.focus();
        } else {
            emailInput.focus();
        }
    });

    // Check for history changes to detect app state
    window.addEventListener('hashchange', function() {
        if (location.hash === '#OTP') {
            otpForm.classList.remove('hidden');
            otpInput.focus();
        } else {
            emailInput.focus();
        }
    });

})();
