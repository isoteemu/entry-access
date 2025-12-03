/**
 * QR Code Web Component
 * 
 * Generates QR codes client-side by fetching data from an endpoint.
 * 
 * Usage:
 *   <qr-code src="/api/qr-data" width="256" height="256" show-link="true"></qr-code>
 * 
 * Attributes:
 *   - src: Endpoint to fetch QR code data (JSON with 'url' or 'content' field)
 *   - width: QR code width (default: 256)
 *   - height: QR code height (default: 256)
 *   - color: QR code color (default: #000000)
 *   - background: Background color (default: #ffffff)
 *   - ecl: Error correction level: L, M, Q, H (default: M)
 *   - device-id: Include device ID in request (true/false)
 *   - show-link: Display QR target URL as clickable link (true/false)
 * 
 * Auto-refresh: Automatically refreshes based on expires_at in JSON response
 */

class QRCodeElement extends HTMLElement {
    constructor() {
        super();
        this.attachShadow({ mode: 'open' });
        this._refreshTimer = null;
        this._abortController = null;
        this._currentContent = null;
        this._expiresAt = null;
    }

    static get observedAttributes() {
        return ['src', 'width', 'height', 'color', 'background', 'ecl', 'device-id', 'show-link'];
    }

    connectedCallback() {
        this.render();
        this.loadAndGenerateQR();
    }

    disconnectedCallback() {
        this._clearTimer();
        if (this._abortController) {
            this._abortController.abort();
            this._abortController = null;
        }
    }

    attributeChangedCallback(name, oldValue, newValue) {
        if (oldValue !== newValue) {
            if (name === 'src') {
                this.loadAndGenerateQR();
            } else if (this._currentContent) {
                this.generateQR(this._currentContent);
            }
        }
    }

    _clearTimer() {
        if (this._refreshTimer) {
            clearTimeout(this._refreshTimer);
            this._refreshTimer = null;
        }
    }

    _scheduleRefresh(expiresAt) {
        this._clearTimer();
        
        if (!expiresAt) return;
        
        const now = Date.now();
        const expiryTime = new Date(expiresAt).getTime();
        const timeUntilExpiry = expiryTime - now;
        
        if (timeUntilExpiry <= 0) {
            // Already expired, refresh immediately
            this.loadAndGenerateQR();
            return;
        }
        
        // Refresh at 50% of the lifetime
        const refreshDelay = Math.max(timeUntilExpiry / 2, 1000); // Minimum 1 second

        this._refreshTimer = setTimeout(() => {
            this.loadAndGenerateQR();
        }, refreshDelay);
    }

    render() {
        this.shadowRoot.innerHTML = `
            <style>
                :host {
                    display: inline-block;
                    position: relative;
                }
                
                #container {
                    width: 100%;
                    height: 100%;
                    display: flex;
                    align-items: center;
                    justify-content: center;
                    background: var(--qr-background, #f9fafb);
                }
                
                #qr-display {
                    max-width: 100%;
                    max-height: 100%;
                }
                
                #qr-display svg {
                    width: 100%;
                    height: 100%;
                }
                
                #loading {
                    display: flex;
                    align-items: center;
                    justify-content: center;
                    padding: 2rem;
                }
                
                #error {
                    display: flex;
                    align-items: center;
                    justify-content: center;
                    color: #dc2626;
                    padding: 1rem;
                    text-align: center;
                    font-size: 0.875rem;
                }
                
                #link-display {
                    margin-top: 0.5rem;
                    text-align: center;
                    font-size: 0.875rem;
                    word-break: break-all;
                    text-overflow: ellipsis;
                    overflow: hidden;
                    max-width: 100%;
                }
                
                #link-display a {
                    color: #3b82f6;
                    text-decoration: none;
                }
                
                #link-display a:hover {
                    text-decoration: underline;
                }
                
                .hidden {
                    display: none !important;
                }
                
                .spinner {
                    border: 3px solid #e5e7eb;
                    border-top: 3px solid #3b82f6;
                    border-radius: 50%;
                    width: 40px;
                    height: 40px;
                    animation: spin 1s linear infinite;
                }
                
                @keyframes spin {
                    0% { transform: rotate(0deg); }
                    100% { transform: rotate(360deg); }
                }
            </style>
            
            <div id="container">
                <div id="loading">
                    <div class="spinner"></div>
                </div>
                <div id="qr-display" class="hidden"></div>
                <div id="error" class="hidden"></div>
            </div>
            <div id="link-display" class="hidden"></div>
        `;
    }

    async loadAndGenerateQR() {
        const src = this.getAttribute('src');
        if (!src) {
            this.showError('No data source specified');
            return;
        }
        // Append cache busting parameter
        const url = new URL(src, window.location.href);
        url.searchParams.set('cb', Date.now().toString());
        
        // Include device_id if attribute is set to "true"
        const includeDeviceId = this.getAttribute('device-id') == 'true';
        if (includeDeviceId) {
            const deviceId = localStorage.getItem('device_id');
            if (deviceId) {
                url.searchParams.set('device_id', deviceId);
            }
        }

        const loadingEl = this.shadowRoot.getElementById('loading');
        const qrDisplayEl = this.shadowRoot.getElementById('qr-display');
        const errorEl = this.shadowRoot.getElementById('error');

        loadingEl.classList.remove('hidden');
        qrDisplayEl.classList.add('hidden');
        errorEl.classList.add('hidden');

        try {
            if (this._abortController) {
                this._abortController.abort();
            }
            this._abortController = new AbortController();

            const response = await fetch(url.toString(), {
                signal: this._abortController.signal
            });

            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }

            const contentType = response.headers.get('content-type');
            let qrContent;
            let expiresAt = null;

            if (contentType && contentType.includes('application/json')) {
                const data = await response.json();
                qrContent = data.url || data.content || data.data;
                if (!qrContent) {
                    throw new Error('Invalid JSON response: missing url/content/data field');
                }
                // Check for expiration in various formats
                expiresAt = data.expires_at || data.expiresAt || data.exp || data.expiration;
            } else if (contentType && contentType.includes('text/')) {
                qrContent = await response.text();
            } else {
                throw new Error('Unsupported content type: ' + contentType);
            }

            this._currentContent = qrContent;
            this._expiresAt = expiresAt;
            await this.generateQR(qrContent);

            loadingEl.classList.add('hidden');
            qrDisplayEl.classList.remove('hidden');

            // Schedule refresh based on expiration
            if (expiresAt) {
                this._scheduleRefresh(expiresAt);
            }

            this.dispatchEvent(new CustomEvent('qr-loaded', {
                detail: { content: qrContent, expiresAt: expiresAt }
            }));

        } catch (error) {
            if (error.name === 'AbortError') {
                return;
            }
            
            console.error('QR Code generation error:', error);
            this.showError(error.message);

            this.dispatchEvent(new CustomEvent('qr-error', {
                detail: { error: error.message }
            }));
        }
    }

    async generateQR(content) {
        if (typeof QRCode === 'undefined') {
            throw new Error('QRCode library not loaded');
        }

        const width = parseInt(this.getAttribute('width') || '256', 10);
        const height = parseInt(this.getAttribute('height') || '256', 10);
        const color = this.getAttribute('color') || '#000000';
        const background = this.getAttribute('background') || '#ffffff';
        const ecl = this.getAttribute('ecl') || 'M';

        const qr = new QRCode({
            content: content,
            width: width,
            height: height,
            color: color,
            background: background,
            ecl: ecl,
            padding: 0,
            join: true
        });

        const svg = qr.svg({ container: 'svg-viewbox' });
        const qrDisplayEl = this.shadowRoot.getElementById('qr-display');
        qrDisplayEl.innerHTML = svg;
        
        // Show link if attribute is set
        const showLink = this.getAttribute('show-link') === 'true';
        const linkDisplayEl = this.shadowRoot.getElementById('link-display');
        
        if (showLink) {
            linkDisplayEl.innerHTML = `<a href="${content}" target="_blank" rel="noopener noreferrer">${content}</a>`;
            linkDisplayEl.classList.remove('hidden');
        } else {
            linkDisplayEl.classList.add('hidden');
        }
    }

    showError(message) {
        const loadingEl = this.shadowRoot.getElementById('loading');
        const qrDisplayEl = this.shadowRoot.getElementById('qr-display');
        const errorEl = this.shadowRoot.getElementById('error');

        loadingEl.classList.add('hidden');
        qrDisplayEl.classList.add('hidden');
        errorEl.classList.remove('hidden');
        errorEl.textContent = `Error: ${message}`;
    }

    refresh() {
        this.loadAndGenerateQR();
    }

    getContent() {
        return this._currentContent;
    }
}

if ('customElements' in window) {
    customElements.define('qr-code', QRCodeElement);
}

export default QRCodeElement;
