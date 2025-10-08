package routes

import (
	. "entry-access-control/internal/jwt"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	. "entry-access-control/internal/config"
	. "entry-access-control/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/skip2/go-qrcode"
)

func genEntryToken(entryID string) (string, error) {
	claim := NewEntryClaim(entryID)
	return GenerateJWT(claim)
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
			slog.Debug("Invalid token format, expected 3 parts", "token", token)
			return "", fmt.Errorf("invalid token format")
		}
		// Validate the token (assuming the token is a JWT)
		claims, err := DecodeEntryJWT(token)
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

func EntryRoute(r *gin.RouterGroup) {

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

		// TODO: Extract from device provisioning data
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

	r.GET("/e/:token", func(c *gin.Context) {
		if err, _ := checkProvisioning(c); err != nil {
			log.Printf("Provisioning check failed: %v", err)
			c.JSON(http.StatusForbidden, gin.H{"error": "Provisioning check failed"})
			return
		}

		token := c.Param("token")

		c.JSON(http.StatusOK, gin.H{"token": token})
	})

}
