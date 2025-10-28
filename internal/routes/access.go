package routes

import (
	access "entry-access-control/internal/access"
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

const ERR_CODES = g.H{
	"AUTH_500": "Internal server error during authentication",
}

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

		if time.Now().Unix() > exp {
			slog.Debug("Entry token expired, creating a new one", "exp", exp, "entryID", entryID)
			createToggle = true
		}
	} else {
		panic("Unexpected token state")
	}

	if createToggle {
		// Notice: To avoid shadowing, not `token, err := ...`
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

func userExists(c *gin.Context, userID string) (bool, error) {
	accessListIface, exists := c.Get("AccessList")
	if !exists {
		slog.Warn("Access list not found in context")
		return false, fmt.Errorf("access list not found in context")
	}
	accessList, ok := accessListIface.(access.AccessList)
	if !ok {
		return false, fmt.Errorf("invalid access list type in context")
	}

	_, err := accessList.Find(userID)
	if err != nil {
		return false, err
	}
	return true, nil
}

func errorPage(c *gin.Context, code int, message string) {
	// Check if message is in error codes
	_message, exists := ERR_CODES[message]
	if exists {
		message = _message.(string)
	}
	// TODO: Implement error page
	c.JSON(code, gin.H{
		"error": message,
	})
}

func EntryRoute(r *gin.RouterGroup) {

	r.GET("/qr", func(c *gin.Context) {
		if err, _ := checkProvisioning(c); err != nil {
			log.Printf("Provisioning check failed: %v", err)
			c.JSON(http.StatusForbidden, gin.H{"error": "Provisioning check failed"})
			return
		}

		// Redirect if cache buster is not set - just to be sure
		if c.Query("cb") == "" {
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
		url := UrlFor(c, "/entry/"+token)

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

	// TODO: Integrate token check, just to show sensible message.
	r.GET("/success", func(c *gin.Context) {
		c.HTML(http.StatusOK, "access_granted.html.tmpl", H(c, gin.H{
			"SupportURL": Cfg.SupportURL,
		}))
	})

	// Router to decide if authentication is needed, or directly grant access
	r.GET("/:token", func(c *gin.Context) {
		if err, _ := checkProvisioning(c); err != nil {
			log.Printf("Provisioning check failed: %v", err)
			c.JSON(http.StatusForbidden, gin.H{"error": "Provisioning check failed"})
			return
		}

		token := c.Param("token")

		// Verify token
		claim, err := DecodeEntryJWT(token)
		if err != nil {
			slog.Debug("Invalid entry token", "error", err)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid entry token"})
			return
		}

		slog.Info("Entry token used", "entryID", claim.EntryID)
		// TODO: Rotate QR code

		// Check if user is logged in
		userID, err := verifyAuth(c)
		if err != nil {
			slog.Error("Failed to verify auth token", "error", err)
			errorPage(c, http.StatusUnauthorized, "Failed to verify auth token")
		}

		exists, err := userExists(c, userID)
		if err != nil || !exists {
			slog.Warn("User has authenticated, but not found in access list", "userID", userID, "error", err, "exists", exists)
			// Destroy the token to avoid reuse
			AuthLogout(c)
			return
		}
		slog.Debug("User authenticated and found in access list", "userID", userID)

		// TODO: Check for access permissions

		c.JSON(http.StatusOK, gin.H{"token": token})
	})
}
