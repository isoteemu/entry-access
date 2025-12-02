document.addEventListener('DOMContentLoaded', async () => {
    const noSleep = new window.NoSleep();
    
    // Need a user interaction to enable NoSleep
    function enableNoSleep() {
        noSleep.enable();
        console.log('NoSleep enabled to prevent device sleep.');
        document.removeEventListener('click', enableNoSleep, false);
    }
    document.addEventListener('click', enableNoSleep, false);
});
