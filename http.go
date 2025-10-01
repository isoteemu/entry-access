package main

import (
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	qrcode "github.com/skip2/go-qrcode"

	. "entry-access-control/utils"
)

const QR_IMAGE_SIZE = 512

func checkProvisioning(c *gin.Context) (error, bool) {
	slog.Debug("Provisioning not implemented yet!")
	return nil, true
}

func securityHeaders(c *gin.Context) {
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("X-Frame-Options", "DENY")
	c.Header("X-XSS-Protection", "1; mode=block")

	// CORS: allow all
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

// Entry tokens store. entry_id -> token
var entryTokens = struct {
	sync.RWMutex
	tokens map[string]string
}{
	tokens: make(map[string]string),
}

func getEntryToken(entryID string) (string, error) {
	var createToggle bool = false
	entryTokens.Lock()
	defer entryTokens.Unlock()

	token, exists := entryTokens.tokens[entryID]
	if !exists {
		createToggle = true
	} else if token != "" {
		// Validate that the token is not expired
		parts := strings.Split(token, ".")
		if len(parts) != 3 {
			slog.Debug("Invalid token format", "token", token)
			return "", fmt.Errorf("invalid token format")
		}
		// Validate the token (assuming the token is a JWT)
		claims, err := decodeEntryJWT(token)
		if err != nil {
			return "", fmt.Errorf("invalid token payload")
		}
		// Check expiration (assuming the token is a JWT)
		exp := claims.ExpiresAt.Time.Unix()
		if err != nil {
			return "", fmt.Errorf("invalid token payload")
		}
		if time.Now().Unix() > exp {
			slog.Debug("Entry token expired, creating a new one", "exp", exp, "entryID", entryID)
			createToggle = true
		}
	} else {
		panic("Unexpected token state")
	}

	if createToggle {
		// Notice: To avoid shadowing, no `token, err := ...`
		var err error
		token, err = genEntryToken(entryID)
		slog.Debug("Generated new entry token", "token", token, "entryID", entryID)
		if err != nil {
			return "", err
		}
		entryTokens.tokens[entryID] = token
	}
	return token, nil
}

func HTTPServer() *gin.Engine {
	r := gin.Default()

	r.Static("/assets/", "./assets/")         // Serve CSS
	r.Static("/dist/assets", "./dist/assets") // Serve compiled CSS and fonts

	r.LoadHTMLGlob("templates/*")

	var allowedCIDRs []string
	for cidr := range strings.SplitSeq(cfg.AllowedNetworks, ",") {
		// Remove spaces and ignore empty sets
		if cidr := strings.TrimSpace(cidr); cidr != "" {
			allowedCIDRs = append(allowedCIDRs, cidr)
		}
	}

	r.Use(IPAccessControl(allowedCIDRs))
	r.Use(securityHeaders)

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})

	r.GET("/api/device/provision", func(c *gin.Context) {
		// Cache buster
		cacheBuster := strconv.FormatInt(time.Now().UTC().Unix(), 16)

		qr_url := UrlFor(c, "/api/device/provision/qr?cb="+cacheBuster)

		// Render page
		c.HTML(http.StatusOK, "provisioning.html.tmpl", gin.H{"QRCodeURL": qr_url})
	})

	r.GET("/api/device/provision/qr", func(c *gin.Context) {
		// Generate provisioning QR image

		// For provisioning url, we need device id and client ip

		deviceID := c.Query("device_id")
		clientIP := c.ClientIP()

		if deviceID == "" || clientIP == "" {
			slog.Warn("Missing device_id or client_ip")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing device_id or client_ip"})
			return
		}

		token, err := GenDeviceProvisionToken(deviceID)
		if err != nil {
			slog.Warn("Failed to generate device provision token", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate device provision token"})
			return
		}

		provisioningURL := UrlFor(c, "/api/device/authorize?"+token)

		qrCode, err := qrcode.Encode(provisioningURL, qrcode.Medium, QR_IMAGE_SIZE)
		if err != nil {
			slog.Warn("Failed to generate QR code", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate QR code"})
			return
		}

		// Send cache expiration based on token TTL
		c.Header("Cache-Control", fmt.Sprintf("max-age=%d", cfg.TokenTTL))
		c.Data(http.StatusOK, "image/png", qrCode)
	})

	r.GET("/p/a", func(c *gin.Context) {
		slog.Warn("Authorization check is missing")
	})

	r.GET("/config.json", func(c *gin.Context) {
		// Provide a initial config
		var clientCfg = gin.H{
			"TokenTTL":        cfg.TokenTTL,
			"TokenExpirySkew": cfg.TokenExpirySkew,
		}

		c.JSON(http.StatusOK, clientCfg)
	})

	r.GET("/e/:token", func(c *gin.Context) {
		if err, _ := checkProvisioning(c); err != nil {
			log.Printf("Provisioning check failed: %v", err)
			c.JSON(http.StatusForbidden, gin.H{"error": "Provisioning check failed"})
			return
		}

		token := c.Param("token")

		c.JSON(http.StatusOK, gin.H{"token": token})
	})

	r.GET("/qr", func(c *gin.Context) {
		if err, _ := checkProvisioning(c); err != nil {
			log.Printf("Provisioning check failed: %v", err)
			c.JSON(http.StatusForbidden, gin.H{"error": "Provisioning check failed"})
			return
		}

		// Redirect if cache buster is not set - just to be sure
		if c.Query("c") == "" {
			slog.Debug("Cache buster not set, redirecting")
			c.Redirect(http.StatusFound, "/qr?cb="+strconv.FormatInt(time.Now().UTC().Unix(), 16))
			return
		}

		token, err := getEntryToken("entry1")
		if err != nil {
			slog.Debug("Error getting entry token", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error getting entry token"})
			return
		}

		slog.Debug("Entry token", "token", token)
		// Generate URL

		// Generate URL pointing to self
		url := UrlFor(c, "/e/"+token)

		// We could cache qr code, but it takes milliseconds to generate
		qr, err := qrcode.Encode(url, qrcode.Medium, QR_IMAGE_SIZE)
		if err != nil {
			slog.Debug("Error generating QR code", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error generating QR code"})
			return
		}
		slog.Debug("Generated QR code", "url", url)

		c.Data(http.StatusOK, "image/png", qr)
	})

	r.GET("/", func(ctx *gin.Context) {
		var qr_url = UrlFor(ctx, "/qr")
		ctx.HTML(http.StatusOK, "qr.html.tmpl", gin.H{"QRCodeURL": qr_url})
	})

	return r
}
