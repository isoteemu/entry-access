import { PingMonitor } from "./ping.js";
import { ErrorHandler } from "./error.js";

import { loadConfig } from "./app.js";

let config = {};

// Load configuration
const config_promise = loadConfig().then(cfg => {
    config = cfg;  // Store config globally

    // Store support contact info
    if (config.SupportContact) {
        localStorage.setItem('support_contact', config.SupportContact);
        console.log("Support contact info cached");
    }
}).catch(err => {
    console.error("Failed to load config:", err);
});

document.addEventListener('DOMContentLoaded', async function() {
    await config_promise; // Ensure config is loaded before proceeding

    console.log('DOM ready, running()');
    run();
});

function run() {
    // Initialize your application
    console.log('App starting...');

    // TODO: Fix the error handler to use latest support info from config
    let errorOptions = {
        supportQRUrl: '/dist/assets/support_qr.png',
        supportContact: localStorage.getItem('support_contact') || config.SupportURL || 'Technical Support',
        autoShow: true // Automatically show overlay when errors are added
    };

    // Initialize the error handler
    const errorHandler = new ErrorHandler(errorOptions);

    window.errorHandler = errorHandler;

    const pingMonitor = new PingMonitor("/api/v1/health", 3);
    // TODO: Temporarily disable ping monitoring for testing
    // pingMonitor.start();
    const pingErrorID = "ping_error";
    const networkErrorID = "network_error";

    // Listen for online/offline events
    window.addEventListener('online', () => {
        console.log("Network is online");
        pingMonitor.start();

        errorHandler.removeError(networkErrorID);
    });
    window.addEventListener('offline', () => {
        console.log("Network is offline");
        pingMonitor.stop();

        errorHandler.addError(networkErrorID, {
            title: `Network Error`,
            message: `Network connection lost.`,
            type: 'error'
        });
    });

    pingMonitor.onStatusChange(({ oldStatus, newStatus, failures }) => {
        console.log(`Status changed: ${oldStatus} → ${newStatus} (failures: ${failures})`);
        switch (newStatus) {
            case 'ok':
                console.log('Server is healthy');
                errorHandler.removeError(pingErrorID);
                break;
            case 'warn':
                console.log('Server is experiencing issues');
                break;
            case 'error':
                console.log('Adding ping error to error handler');
                errorHandler.addError(pingErrorID, {
                    title: `Communication Error`,
                    message: `Communication failed with backend after ${failures} attempts.`,
                    type: 'error'
                });
                break;
        }
    });

    const appReadyEvent = new CustomEvent('app:ready', {
        // 'bubbles: true' allows the event to propagate up the DOM tree (useful for elements)
        bubbles: true, 
        
        // 'cancelable: true' allows the event to be cancelled via event.preventDefault()
        cancelable: false,
        detail: { config }
    });
    document.dispatchEvent(appReadyEvent);
}
