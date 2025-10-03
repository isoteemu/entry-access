// Authentication middlware
// Checks for a valid authentication token in the request header
// If valid, sets the user information in the context
// If invalid, returns 401 Unauthorized
package routes

import (
	. "entry-access-control/internal/config"
	. "entry-access-control/internal/jwt"
	. "entry-access-control/internal/utils"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

const authCookieName = "auth_token"

const AUTH_FAIL_STATUS = 401 // HTTP status code for authentication failure

// Get authentication TTL in seconds
func authTTL() uint {
	// Convert days to seconds
	return Cfg.UserAuthTTL * 24 * 60 * 60 // in seconds
}

// Set authentication cookie
// The cookie is set to expire when the token expires
func setAuthCookie(c *gin.Context, token string) {

	ttl := authTTL()
	secure := c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https"

	// Convert to int for SetCookie

	c.SetCookie(
		authCookieName,
		token,
		int(ttl),
		"/",
		"",
		secure, // Secure
		true,
	)
}

func verifyAuth(c *gin.Context) (string, error) {
	// Get auth token from cookie
	token, err := c.Cookie(authCookieName)
	if err != nil {
		return "", err
	}
	// Decode token
	claims, err := DecodeAuthJWT(token)
	if err != nil {
		return "", err
	}
	return claims.UserID, nil
}

func renewAuth(c *gin.Context, userId string, forceRenew bool) error {

	// Fetch old token to invalidate it
	oldToken, err := c.Cookie(authCookieName)
	if err == nil {
		// Decode old token to get its ID
		oldClaims, err := DecodeAuthJWT(oldToken)
		if err == nil {
			nonce := oldClaims.ID
			expiration := oldClaims.ExpiresAt.Time

			// Log odd behavior, where the user ID in the token does not match the expected user ID
			// This could indicate token tampering attempt, but also benign issues like user ID change
			if oldClaims.UserID != userId {
				slog.Warn("renewAuth: User ID mismatch in token", "tokenUserID", oldClaims.UserID, "expectedUserID", userId)
				return nil
			}

			renewAge := time.Duration(authTTL()/2) * time.Second
			if forceRenew || time.Until(expiration) < renewAge {
				slog.Info("Renewing auth token for user", "userID", userId)

				// Invalidate old token by consuming its nonce
				NonceStore.Consume(c.Request.Context(), nonce)

				forceRenew = true
			}
		}
	} else {
		slog.Warn("renewAuth: No existing auth token found", "error", err)
		c.AbortWithError(AUTH_FAIL_STATUS, err)
	}

	if !forceRenew {
		// Early stop: No need to renew
		return nil
	}

	// Generate new auth token
	claim := NewAuthClaims(userId)
	token, err := GenerateJWT(claim)
	if err != nil {
		return err
	}
	// Set auth cookie
	setAuthCookie(c, token)
	return nil
}

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check for existing token
		uid, err := verifyAuth(c)
		if err != nil {
			slog.Warn("AuthMiddleware: Invalid or missing auth token", "error", err)
			c.AbortWithStatus(AUTH_FAIL_STATUS)
			return
		}

		// Set user ID in context
		c.Set("userID", uid)
		c.Next()
	}
}

func AuthRoutes(r *gin.RouterGroup) {
	// Route to renew authentication token
	r.GET("/renew", AuthMiddleware(), func(c *gin.Context) {
		// Get user ID from context
		uid, exists := c.Get("userID")
		if !exists {
			slog.Warn("AuthRoutes: User ID not found in context")
			c.AbortWithStatus(AUTH_FAIL_STATUS)
			return
		}
		userIdStr, ok := uid.(string)
		if !ok {
			slog.Warn("AuthRoutes: User ID in context is not a string")
			c.AbortWithStatus(AUTH_FAIL_STATUS)
			return
		}

		err := renewAuth(c, userIdStr, true)
		if err != nil {
			slog.Error("AuthRoutes: Failed to renew auth token", "error", err)
			c.AbortWithStatus(500)
			return
		}
		c.Status(200)
	})

	// Route to check authentication status
	r.GET("/status", AuthMiddleware(), func(c *gin.Context) {
		// If we reach here, the token is valid
		c.JSON(200, gin.H{"status": "authenticated", "userID": c.GetString("userID")})
	})
}
