package app

import (
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	. "entry-access-control/internal/config"
	. "entry-access-control/internal/utils"

	routes "entry-access-control/internal/routes"

	"github.com/gin-contrib/multitemplate"
	"github.com/gin-gonic/gin"
)

const API_V1_PREFIX = "/api/v1"

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

// Multitemplate renderer to support layouts
// Copied from gin-contrib/multitemplate/example/advanced/example.go
func createRenderer(templateDir string) multitemplate.Renderer {
	r := multitemplate.NewRenderer()

	layouts, err := filepath.Glob(templateDir + "/layouts/*.html.tmpl")
	if err != nil {
		panic(err.Error())
	}

	includes, err := filepath.Glob(templateDir + "/*.html.tmpl")
	if err != nil {
		panic(err.Error())
	}

	// Generate our templates map from our layouts/ and includes/ directories
	for _, include := range includes {
		layoutCopy := make([]string, len(layouts))
		copy(layoutCopy, layouts)
		files := append(layoutCopy, include)
		slog.Debug("Loading template", "template", include, "with_layouts", layoutCopy)
		r.AddFromFiles(filepath.Base(include), files...)
	}
	return r
}

func HTTPServer() *gin.Engine {
	r := gin.New()

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

	/*
		// Initialize logger
		logger := slog.Default().WithGroup("http").
			With("gin_mode", gin.Mode())

		r.Use(sloggin.NewWithConfig(logger, sloggin.Config{
			HandleGinDebug: true,
		}))
		r.Use(gin.Recovery())
	*/

	// Inject the HTML renderer into the context for access in handlers
	// This allows rendering templates in sub-packages
	// without passing the renderer explicitly
	// See: RenderTemplate in utils/http.go
	r.Use(func(c *gin.Context) {
		c.Set("HTML", r.HTMLRender)
		c.Next()
	})

	// Load HTML templates
	r.HTMLRender = createRenderer("web/templates")

	return r
}

func RegisterRoutes(r *gin.Engine) *gin.Engine {
	// --- Routes ---
	// Serve config for client-side use
	r.GET("/config.json", func(c *gin.Context) {
		// Provide a initial config
		SupportQRURL := UrlFor(c, "dist/assets/support_qr.png")
		var clientCfg = gin.H{
			"TokenTTL":        Cfg.TokenTTL,
			"TokenExpirySkew": Cfg.TokenExpirySkew,
			"SupportURL":      Cfg.SupportURL,
			"SupportQRURL":    SupportQRURL,
		}

		c.JSON(http.StatusOK, clientCfg)
	})

	r.GET("/", func(ctx *gin.Context) {
		var qr_url = UrlFor(ctx, "/qr")
		ctx.HTML(http.StatusOK, "qr.html.tmpl", gin.H{"QRCodeURL": qr_url})
	})

	apirg := r.Group(API_V1_PREFIX)
	routes.Health(apirg)

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
	routes.EmailLoginRoute(auth_rg)

	return r
}
