import { PingMonitor } from "./ping.js";
import { ErrorHandler } from "./error.js";

let config = {};

const config_promise = fetch('/config.json')
    .then(response => response.json())
    .then(data => {
        config = data;
        console.log('Configuration loaded:', config);
    })
    .catch(error => {
        console.error('Error loading configuration:', error);
    });

document.addEventListener('DOMContentLoaded', async function() {
    await config_promise; // Ensure config is loaded before proceeding

    console.log('DOM ready, running()');
    run();
});

function run() {
    // Initialize your application
    console.log('App starting...');

    // Initialize the error handler
    const errorHandler = new ErrorHandler({
        supportQRUrl: '/dist/assets/support_qr.png',
        supportContact: config.SupportURL || 'Technical Support',
        autoShow: true // Automatically show overlay when errors are added
    });

    window.errorHandler = errorHandler;


    const pingMonitor = new PingMonitor("/ping", 3);
    pingMonitor.start();
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
}
