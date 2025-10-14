package app

import (
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	. "entry-access-control/internal/config"
	. "entry-access-control/internal/utils"

	routes "entry-access-control/internal/routes"

	"github.com/gin-gonic/gin"
)

func securityHeaders(c *gin.Context) {
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("X-Frame-Options", "DENY")
	c.Header("X-XSS-Protection", "1; mode=block")

	// Disable caching
	c.Header("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
	c.Next()
}

// Middleware to check if the IP is allowed.
func IPAccessControl(allowedCIDRs []string) gin.HandlerFunc {
	// Parse allowed CIDRs
	var parsedCIDRs []*net.IPNet

	// Allow local networks in debug mode
	if os.Getenv("GIN_MODE") != "release" {
		localhostCIDRs := []string{"127.0.0.1/8", "::1/128"}
		allowedCIDRs = append(allowedCIDRs, localhostCIDRs...)
	}

	for _, cidr := range allowedCIDRs {
		_, net, err := net.ParseCIDR(cidr)
		if err != nil {
			slog.Warn("Invalid CIDR", "cidr", cidr)
			continue
		}
		slog.Debug("Allowed CIDR", "cidr", cidr)
		parsedCIDRs = append(parsedCIDRs, net)
	}

	return func(c *gin.Context) {
		clientIP := net.ParseIP(c.ClientIP())
		if clientIP == nil {
			// Should not happen
			slog.Warn("Invalid client IP", "ip", c.ClientIP())
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Forbidden"})
			return
		}

		for _, cidr := range parsedCIDRs {
			if cidr.Contains(clientIP) {
				c.Next()
				return
			}
		}
		slog.Warn("IP not allowed", "ip", clientIP)
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Forbidden"})
	}
}

func BaseUrlMiddleware(baseurl string) gin.HandlerFunc {

	var urlParts, err = url.Parse(baseurl)
	if err != nil {
		panic("Invalid baseurl in config: " + err.Error())
	}

	return func(c *gin.Context) {
		// Check if the baseurl contains  host and protocol. Use from context if not.
		if urlParts.Scheme == "" {
			// Detect scheme from request
			if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
				urlParts.Scheme = "https"
			} else {
				urlParts.Scheme = "http"
			}
			// Or use the request scheme if available
			if c.Request.URL.Scheme != "" {
				urlParts.Scheme = c.Request.URL.Scheme
			}
		}
		if urlParts.Host == "" {
			// Detect host from request
			urlParts.Host = c.Request.Host
		}
		c.Set("BaseURL", urlParts.String())

		c.Next()
	}
}

func GetBaseURL(c *gin.Context) string {
	return c.MustGet("BaseURL").(string)
}

func HTTPServer() *gin.Engine {
	r := gin.Default()

	r.Static("/assets/", "./web/assets/")
	r.Static("/dist/assets", "./dist/assets") // Serve compiled CSS and fonts

	if Cfg.AllowedNetworks != "" {
		slog.Debug("Enabling IP access control", "allowed_networks", Cfg.AllowedNetworks)
		var allowedCIDRs []string

		for cidr := range strings.SplitSeq(Cfg.AllowedNetworks, ",") {
			// Remove spaces and ignore empty sets
			if cidr := strings.TrimSpace(cidr); cidr != "" {
				allowedCIDRs = append(allowedCIDRs, cidr)
			}
		}

		r.Use(IPAccessControl(allowedCIDRs))
	}

	r.Use(securityHeaders)
	r.Use(BaseUrlMiddleware(Cfg.BaseURL))

	// Inject the HTML renderer into the context for access in handlers
	// This allows rendering templates in sub-packages
	// without passing the renderer explicitly
	// See: RenderTemplate in utils/http.go
	r.Use(func(c *gin.Context) {
		c.Set("HTML", r.HTMLRender)
		c.Next()
	})

	// Load HTML templates
	r.LoadHTMLGlob("web/templates/*")

	// --- Routes ---
	// Serve config for client-side use
	r.GET("/config.json", func(c *gin.Context) {
		// Provide a initial config
		var clientCfg = gin.H{
			"TokenTTL":        Cfg.TokenTTL,
			"TokenExpirySkew": Cfg.TokenExpirySkew,
			"SupportURL":      Cfg.SupportURL,
		}

		c.JSON(http.StatusOK, clientCfg)
	})

	root := r.Group("/")
	routes.Ping(root)

	r.GET("/", func(ctx *gin.Context) {
		var qr_url = UrlFor(ctx, "/qr")
		ctx.HTML(http.StatusOK, "qr.html.tmpl", gin.H{"QRCodeURL": qr_url})
	})

	// Provisioning routes
	rg := r.Group("/api/provision")
	routes.ProvisioningApi(rg)

	// Entry access routes
	rg = r.Group("/entry")
	routes.EntryRoute(rg)

	// Authentication routes
	auth_rg := r.Group("/auth")
	routes.AuthRoutes(auth_rg)

	// Email login routes
	email_rg := auth_rg.Group("/email")
	routes.EmailLoginRoute(email_rg)

	// Debug routes
	routes.RegisterDebugRoutes(r)
	return r
}
