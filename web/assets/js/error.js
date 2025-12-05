/**
 * General Error Handling and Display System
 * Manages multiple errors with overlay display and support QR code
 */
export class ErrorHandler {

    static instance = null;

    constructor(options = {}) {
        this.errors = new Map();
        this.isVisible = false;
        this.overlay = null;
        
        // Configuration
        this.config = {
            supportQRUrl: options.supportQRUrl || '/dist/assets/support_qr.png',
            supportContact: options.supportContact || 'Contact Support',
            supportDescription: options.supportDescription || 'If you continue to experience issues, please contact our support team using the QR code below.',
            autoShow: options.autoShow !== false, // Default to true
            zIndex: options.zIndex || 9999,
            ...options
        };
        
        // Initialize overlay
        this.createOverlay();
        
        // Bind methods
        this.show = this.show.bind(this);
        this.hide = this.hide.bind(this);
        this.toggle = this.toggle.bind(this);
    }
    
    /**
     * Add or update an error
     * @param {string} id - Unique identifier for the error
     * @param {object} errorData - Error information
     */
    addError(id, errorData) {
        const error = {
            id,
            title: errorData.title || 'Error',
            message: errorData.message || 'An unexpected error occurred.',
            type: errorData.type || 'error', // error, warning, info
            timestamp: new Date(),
            active: true,
            ...errorData
        };
        
        this.errors.set(id, error);
        
        if (this.config.autoShow) {
            this.show();
        }
        
        this.updateDisplay();
        
        // Emit custom event
        this.dispatchEvent('errorAdded', { error });
    }
    
    /**
     * Remove an error
     * @param {string} id - Error identifier to remove
     */
    removeError(id) {
        const error = this.errors.get(id);
        if (error) {
            this.errors.delete(id);
            this.updateDisplay();
            
            // Hide overlay if no active errors
            if (this.getActiveErrors().length === 0) {
                this.hide();
            }
            
            this.dispatchEvent('errorRemoved', { error });
        }
    }
    
    /**
     * Toggle error active state
     * @param {string} id - Error identifier
     * @param {boolean} active - Optional explicit state
     */
    toggleError(id, active = null) {
        const error = this.errors.get(id);
        if (error) {
            error.active = active !== null ? active : !error.active;
            this.updateDisplay();
            
            this.dispatchEvent('errorToggled', { error });
        }
    }
    
    /**
     * Get all active errors
     * @returns {Array} Array of active errors
     */
    getActiveErrors() {
        return Array.from(this.errors.values()).filter(error => error.active);
    }
    
    /**
     * Clear all errors
     */
    clearAll() {
        this.errors.clear();
        this.hide();
        this.dispatchEvent('errorsCleared', {});
    }
    
    /**
     * Show the error overlay
     */
    show() {
        if (!this.isVisible) {
            this.isVisible = true;
            this.overlay.style.display = 'flex';
            document.body.style.overflow = 'hidden';
            
            // Animate in
            requestAnimationFrame(() => {
                this.overlay.classList.add('opacity-100');
                this.overlay.classList.remove('opacity-0');
            });
            
            this.dispatchEvent('overlayShown', {});
        }
    }
    
    /**
     * Hide the error overlay
     */
    hide() {
        if (this.isVisible) {
            this.isVisible = false;
            
            // Animate out
            this.overlay.classList.add('opacity-0');
            this.overlay.classList.remove('opacity-100');
            
            setTimeout(() => {
                this.overlay.style.display = 'none';
                document.body.style.overflow = '';
            }, 300);
            
            this.dispatchEvent('overlayHidden', {});
        }
    }
    
    /**
     * Toggle overlay visibility
     */
    toggle() {
        if (this.isVisible) {
            this.hide();
        } else {
            this.show();
        }
    }
    
    /**
     * Create the overlay DOM structure
     */
    createOverlay() {
        this.overlay = document.createElement('div');
        this.overlay.className = `fixed inset-0 bg-red-600 transition-opacity duration-300 opacity-0`;
        this.overlay.style.zIndex = this.config.zIndex;
        this.overlay.style.display = 'none';
        
        // Create content directly in overlay
        this.overlay.innerHTML = this.getOverlayHTML();
        
        document.body.appendChild(this.overlay);
        
        // Add event listeners
        this.addEventListeners();
    }
    
    /**
     * Generate the overlay HTML content
     */
    getOverlayHTML() {
        return `
            <div class="container mx-auto px-4 py-8 text-white min-h-screen flex flex-col">
                <!-- Header -->
                <div class="flex items-center justify-center mb-8">
                    <div class="text-6xl mr-6">☹</div>
                    <div>
                        <h1 class="text-4xl font-light mb-2">Device ran into a problems.</h1>
                        <!-- <p class="text-xl opacity-90">Device ran into problems.</p> -->
                    </div>
                </div>
                
                <!-- Two-column layout -->
                <div class="flex-1 flex flex-col lg:flex-row lg:divide-x lg:divide-red-500">
                    
                    <!-- Left Section: QR Code -->
                    <div class="w-full lg:w-1/2 lg:pr-8 pb-8 lg:pb-0">
                        <div class="space-y-6">
                            <p class="text-lg opacity-90">
                                To get more information about this issue and possible fixes, please refer to the support options below.
                            </p>
                            
                            <!-- QR Code -->
                            <div class="flex justify-center">
                                <div class="bg-white p-4">
                                    <img src="${this.config.supportQRUrl}" alt="Support QR Code: ${this.config.supportQRUrl}" class="w-48 h-48" id="support-qr-image">
                                </div>
                            </div>

                            <p class="text-sm opacity-75">
                                You can contact support at: ${this.config.supportContact}
                            </p>
                        </div>
                    </div>
                    
                    <!-- Right Section: Error Information -->
                    <div class="w-full lg:w-1/2 lg:pl-8 pt-8 lg:pt-0">
                        <div class="space-y-6">
                            <!-- Primary Error Details -->
                            <div id="primary-error-content">
                                <!-- Primary error will be inserted here -->
                            </div>
                            
                            <!-- Additional Errors -->
                            <div id="additional-errors-content" class="space-y-2">
                                <!-- Additional errors will be inserted here -->
                            </div>
                        </div>
                    </div>
                </div>
                
                <!-- Footer -->
                <div class="mt-8 pt-4 border-t border-red-500 opacity-75">
                    <div class="flex justify-between items-center text-sm">
                        <span id="error-progress">0% complete</span>
                        <!-- Removed stop code from footer -->
                    </div>
                </div>
            </div>
        `;
    }
    
    /**
     * Add event listeners to overlay elements
     */
    addEventListeners() {
        // Click outside to close (disabled for Windows-style error)
        // this.overlay.addEventListener('click', (e) => {
        //     if (e.target === this.overlay) {
        //         this.hide();
        //     }
        // });
        
        // Escape key to close (disabled for Windows-style error)
        // document.addEventListener('keydown', (e) => {
        //     if (e.key === 'Escape' && this.isVisible) {
        //         this.hide();
        //     }
        // });
    }
    
    /**
     * Update the error display
     */
    updateDisplay() {
        const primaryErrorContent = this.overlay.querySelector('#primary-error-content');
        const additionalErrorsContent = this.overlay.querySelector('#additional-errors-content');
        const errorProgress = this.overlay.querySelector('#error-progress');
        // Removed: const primaryErrorCode = this.overlay.querySelector('#primary-error-code');
        
        if (!primaryErrorContent) return;
        
        const activeErrors = this.getActiveErrors();
        
        // Update progress (simulate progress based on error count)
        if (errorProgress) {
            const progress = Math.min(100, activeErrors.length * 20);
            errorProgress.textContent = `${progress}% complete`;
        }
        
        if (activeErrors.length === 0) {
            primaryErrorContent.innerHTML = `
                <div class="text-white opacity-75">
                    <h3 class="text-xl font-medium mb-2">No active errors</h3>
                    <p>System is operating normally</p>
                </div>
            `;
            additionalErrorsContent.innerHTML = '';
            // Removed: if (primaryErrorCode) { primaryErrorCode.textContent = 'SYSTEM_OK'; }
            return;
        }
        
        // Show primary error (first error)
        const primaryError = activeErrors[0];
        primaryErrorContent.innerHTML = `
            <div class="text-white">
                <h3 class="text-xl font-medium mb-2">${primaryError.title}</h3>
                <p class="text-lg opacity-90 mb-4">${primaryError.message}</p>
                ${primaryError.code ? `<p class="text-sm opacity-75 font-mono mb-2">Stop code: ${primaryError.code}</p>` : `<p class="text-sm opacity-75 font-mono mb-2">Stop code: ${primaryError.id.toUpperCase()}</p>`}
                <p class="text-sm opacity-75">Time: ${this.formatTimestamp(primaryError.timestamp)}</p>
            </div>
        `;
        
        // Removed: if (primaryErrorCode) { primaryErrorCode.textContent = primaryError.code || primaryError.id.toUpperCase(); }
        
        // Show additional errors as a list of IDs
        if (activeErrors.length > 1) {
            const additionalErrorIds = activeErrors.slice(1).map(error => error.id);
            additionalErrorsContent.innerHTML = `
                <div class="text-white opacity-75">
                    <h4 class="text-lg font-medium mb-2">Additional Error IDs:</h4>
                    <div class="font-mono text-sm space-y-1">
                        ${additionalErrorIds.map(id => `<div>• ${id.toUpperCase()}</div>`).join('')}
                    </div>
                </div>
            `;
        } else {
            additionalErrorsContent.innerHTML = '';
        }
    }
    
    /**
     * Format timestamp for display
     */
    formatTimestamp(timestamp) {
        return timestamp.toLocaleString();
    }
    
    /**
     * Dispatch custom event
     */
    dispatchEvent(eventType, detail) {
        const event = new CustomEvent(`errorHandler:${eventType}`, { detail });
        document.dispatchEvent(event);
    }
    
    /**
     * Destroy the error handler and clean up
     */
    destroy() {
        if (this.overlay && this.overlay.parentNode) {
            this.overlay.parentNode.removeChild(this.overlay);
        }
        this.errors.clear();
        document.body.style.overflow = '';
    }

    /**
     * Get singleton instance
     * @param {object} options - Configuration options
     * @returns {ErrorHandler} Singleton instance
     */
    static getInstance(options = {}) {
        if (!ErrorHandler.instance) {
            ErrorHandler.instance = new ErrorHandler(options);
        }
        return ErrorHandler.instance;
    }
}

// Create singleton instance
export const instance = new ErrorHandler();

// Export for module usage

export default instance;


// Make available globally for inline event handlers
if (typeof window !== 'undefined') {
    window.ErrorHandler = instance;
}
