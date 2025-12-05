package routes

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	. "entry-access-control/internal/config"
	. "entry-access-control/internal/jwt"
	"entry-access-control/internal/storage"
	"entry-access-control/internal/utils"

	"github.com/gin-gonic/gin"
)

type registrationResponse struct {
	Status        string `json:"status"`
	DeviceID      string `json:"device_id,omitempty"`
	Message       string `json:"message"`
	Authenticated bool   `json:"authenticated,omitempty"`
}

func genProvisioningJWT(deviceID string, clientIP string) (string, error) {
	claim := NewDeviceProvisionClaim(deviceID, clientIP)
	return GenerateJWT(claim)
}

func getProvisioning(c *gin.Context, deviceID string) (error, storage.Device) {
	if deviceID == "" {
		return ErrDeviceIDRequired, storage.Device{}
	}

	// Get storage provider from context
	err, storageProvider := GetStorageProvider(c)
	if err != nil {
		slog.Error("Failed to get storage provider from context", "error", err)
		return err, storage.Device{}
	}
	ctx := c.Request.Context()

	// Check if device exists in the database
	device, err := storageProvider.GetDevice(ctx, deviceID)
	if err != nil {
		// Device doesn't exist, create it as pending
		slog.Info("New device detected, adding to pending pool", "device_id", deviceID)

		clientIP := c.ClientIP()
		newDevice := storage.Device{
			DeviceID: deviceID,
			ClientIP: clientIP,
			Status:   storage.DeviceStatusPending,
		}

		if err := storageProvider.CreateDevice(ctx, newDevice); err != nil {
			slog.Error("Failed to create device", "device_id", deviceID, "error", err)
			return fmt.Errorf("%w: %v", ErrFailedToCreateDevice, err), newDevice
		}

		return ErrDevicePendingApproval, newDevice
	}

	// Check device status
	switch device.Status {
	case storage.DeviceStatusApproved:
		slog.Debug("Device is approved", "device_id", deviceID)
		return nil, *device
	case storage.DeviceStatusPending:
		slog.Debug("Device is pending approval", "device_id", deviceID)
		return nil, *device
	case storage.DeviceStatusRejected:
		slog.Warn("Device is rejected", "device_id", deviceID)
		return ErrDeviceRejected, *device
	default:
		slog.Error("Unknown device status", "device_id", deviceID, "status", device.Status)
		return fmt.Errorf("%w: %s", ErrDeviceStatusUnknown, device.Status), storage.Device{}
	}
}

// TODO: Implement device registration token generation
// func genDeviceRegistrationToken(deviceID string) (string, error) {
// 	claim := NewDeviceRegistrationClaim(deviceID)
// 	return GenerateJWT(claim)
// }

func ProvisioningApi(r *gin.RouterGroup) {

	// Device provisioning route
	r.GET("/", func(c *gin.Context) {
		// Cache buster
		cacheBuster := strconv.FormatInt(time.Now().UTC().Unix(), 16)

		qr_url := utils.UrlFor(c, r.BasePath()+"/qr.json?cb="+cacheBuster)

		// Render page
		c.HTML(http.StatusOK, "provisioning.html.tmpl", gin.H{"QRCodeURL": qr_url})
	})

	// QR code generation route
	// Expects device_id as query parameter
	// Example: /api/provision/qr.json?device_id=DEVICE123
	r.GET("qr.json", func(c *gin.Context) {
		// Generate provisioning QR image

		// For provisioning url, we need device id and client IP
		// Device ID is provided as query parameter, client IP is taken from request
		// Client IP is used to restrict provisioning to specific IP.
		// Note: In production behind proxy, make sure to set GIN_TRUSTED_PROXIES env variable accordingly

		deviceID := c.Query("device_id")
		clientIP := c.ClientIP()

		if deviceID == "" || clientIP == "" {
			AbortWithHTTPError(c, http.StatusBadRequest, ErrMissingParameter)
			return
		}

		token, err := genProvisioningJWT(deviceID, clientIP)
		if err != nil {
			AbortWithHTTPError(c, http.StatusInternalServerError, err)
			return
		}

		provisioningURL := utils.UrlFor(c, r.BasePath()+"/authorize?"+token)

		// Send cache expiration based on token TTL
		c.Header("Cache-Control", fmt.Sprintf("max-age=%d", Cfg.TokenTTL))

		c.JSON(http.StatusOK, gin.H{
			"url":        provisioningURL,
			"expires_at": time.Now().Add(time.Duration(Cfg.TokenTTL) * time.Second).Format(time.RFC3339),
		})
	})

	r.POST("/register", func(c *gin.Context) {

		var err error
		var deviceID string

		type registrationRequest struct {
			DeviceID string `form:"device_id" json:"device_id"`
		}

		var req registrationRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			slog.Warn("Invalid registration request", "error", err)
			AbortWithHTTPError(c, http.StatusBadRequest, ErrInvalidRequest)
			return
		}
		if req.DeviceID != "" {
			if !utils.VerifyDeviceID(req.DeviceID, []byte(Cfg.Secret)) {
				slog.Warn("Existing device ID verification failed on registration", "device_id", req.DeviceID)
				AbortWithError(c, ErrDeviceIDVerificationFailed)
				return
			}
			deviceID = req.DeviceID
		} else {
			deviceID, err = utils.GenerateDeviceID([]byte(Cfg.Secret))
			if err != nil {
				// Should not happen
				slog.Error("Failed to generate device ID", "error", err)
				AbortWithError(c, ErrInternalServer)
				return
			}
		}

		// Check if device is already registered, creates a new pending device if not found
		err, provisioning := getProvisioning(c, deviceID)
		if err != nil {
			// Device is rejected
			AbortWithError(c, err)
		}

		// Check IP match
		clientIP := c.ClientIP()
		if provisioning.ClientIP != clientIP {
			slog.Warn("Client IP mismatch during device registration", "device_id", deviceID, "expected_ip", provisioning.ClientIP, "actual_ip", clientIP)
			AbortWithError(c, ErrClientIPMismatch)
			return
		}

		// Check if device is approved
		switch provisioning.Status {
		case storage.DeviceStatusApproved:
			// TODO: Check authentication status
			c.JSON(http.StatusOK, registrationResponse{
				Status:        "approved",
				Authenticated: false,
				DeviceID:      deviceID,
				Message:       "Device is already approved",
			})
			return
		case storage.DeviceStatusPending:
			slog.Info("Device registration pending approval", "device_id", deviceID)
			c.JSON(http.StatusAccepted, registrationResponse{
				Status:   "pending",
				DeviceID: deviceID,
				Message:  "Device registration is pending approval",
			})
			return
		case storage.DeviceStatusRejected:
			slog.Warn("Device registration attempt for rejected device", "device_id", deviceID)
			AbortWithError(c, ErrDeviceRejected)
			return
		default:
			// Should not reach here
			AbortWithError(c, fmt.Errorf("unexpected device status during registration"))
			return
		}
	})

	r.GET("/sse/:device_id", func(c *gin.Context) {
		// Not implemented yet
		AbortWithHTTPError(c, http.StatusNotImplemented, fmt.Errorf("SSE endpoint not implemented yet"))
		return
	})
}
