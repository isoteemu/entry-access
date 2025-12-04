
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
