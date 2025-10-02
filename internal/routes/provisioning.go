package routes

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	. "entry-access-control/internal/config"
	. "entry-access-control/internal/utils"

	. "entry-access-control/internal/jwt"

	"github.com/gin-gonic/gin"
	"github.com/skip2/go-qrcode"
)

func genProvisioningJWT(deviceID string, clientIP string) (string, error) {
	claim := NewDeviceProvisionClaim(deviceID, clientIP)
	return GenerateJWT(claim)
}

func checkProvisioning(c *gin.Context) (error, bool) {
	slog.Debug("Provisioning not implemented yet!")
	return nil, true
}

func ProvisioningApi(r *gin.RouterGroup) {

	// Device provisioning route
	r.GET("/device", func(c *gin.Context) {
		// Cache buster
		cacheBuster := strconv.FormatInt(time.Now().UTC().Unix(), 16)

		qr_url := UrlFor(c, "/api/provision/qr?cb="+cacheBuster)

		// Render page
		c.HTML(http.StatusOK, "provisioning.html.tmpl", gin.H{"QRCodeURL": qr_url})
	})

	// QR code generation route
	// Expects device_id as query parameter
	// Example: /api/provision/qr?device_id=DEVICE123
	r.GET("qr", func(c *gin.Context) {
		// Generate provisioning QR image

		// For provisioning url, we need device id and client IP
		// Device ID is provided as query parameter, client IP is taken from request
		// Client IP is used to restrict provisioning to specific IP.
		// Note: In production behind proxy, make sure to set GIN_TRUSTED_PROXIES env variable accordingly
		// or use c.Request.Header.Get("X-Forwarded-For") to get real client IP

		deviceID := c.Query("device_id")
		clientIP := c.ClientIP()

		if deviceID == "" || clientIP == "" {
			slog.Warn("Missing device_id or client_ip")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing device_id or client_ip"})
			return
		}

		token, err := genProvisioningJWT(deviceID, clientIP)
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
		c.Header("Cache-Control", fmt.Sprintf("max-age=%d", Cfg.TokenTTL))
		c.Data(http.StatusOK, "image/png", qrCode)
	})
}
