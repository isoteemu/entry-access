// Authentication middlware
// Checks for a valid authentication token in the request header
// If valid, sets the user information in the context
// If invalid, returns 401 Unauthorized
package routes

import (
	. "entry-access-control/internal/config"
	. "entry-access-control/internal/jwt"
	"entry-access-control/internal/nonce"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

const AUTH_COOKIE_NAME = "auth_token"

const AUTH_FAIL_STATUS = http.StatusUnauthorized // HTTP status code for authentication failure

var (
	ErrUserNotFound  = errors.New("user not found in context")
	ErrUserNotString = errors.New("user ID in context is not a string")
)

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
		AUTH_COOKIE_NAME,
		token,
		int(ttl),
		"/",
		"",
		secure, // Secure
		true,
	)
}

func GetUser(c *gin.Context) (string, error) {
	// Get user ID from context
	uid, exists := c.Get("userID")
	if !exists {
		return "", ErrUserNotFound
	}
	userIdStr, ok := uid.(string)
	if !ok {
		slog.Warn("GetUser: User ID in context is not a string")
		return "", ErrUserNotString
	}
	return userIdStr, nil
}

func NewAuth(c *gin.Context, userId string) error {
	// Create new auth token
	claim := NewAuthClaims(userId)
	token, err := GenerateJWT(claim)
	if err != nil {
		return err
	}
	// Set auth cookie
	setAuthCookie(c, token)
	return nil
}

func verifyAuth(c *gin.Context) (string, error) {
	// Get auth token from cookie
	token, err := c.Cookie(AUTH_COOKIE_NAME)
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
	oldToken, err := c.Cookie(AUTH_COOKIE_NAME)
	if err == nil {
		// Decode old token to get its ID
		oldClaims, err := DecodeAuthJWT(oldToken)
		if err == nil {
			nonceValue := oldClaims.ID
			expiration := oldClaims.ExpiresAt.Time

			// Log odd behavior, where the user ID in the token does not match the expected user ID
			// This could indicate token tampering attempt, but also benign issues like user ID change
			if oldClaims.UserID != userId {
				slog.Warn("renewAuth: User ID mismatch in token", "tokenUserID", oldClaims.UserID, "expectedUserID", userId)
				return nil
			}

			// If MustRenew is set, we must renew the token
			if oldClaims.MustRenew {
				slog.Debug("renewAuth: Token marked for mandatory renewal", "userID", userId)
				forceRenew = true
			}

			renewAge := time.Duration(authTTL()/2) * time.Second
			if forceRenew || time.Until(expiration) < renewAge {
				slog.Debug("Renewing auth token for user", "userID", userId)

				// Invalidate old token by consuming its nonce
				nonce.Store.Consume(c.Request.Context(), nonceValue)

				forceRenew = true
			}
		}
	} else if !forceRenew {
		slog.Warn("renewAuth: No existing auth token found", "error", err)
		c.AbortWithError(AUTH_FAIL_STATUS, err)
	}

	if !forceRenew {
		// Early stop: No need to renew
		slog.Debug("renewAuth: No need to renew auth token", "userID", userId)
		return nil
	}

	// Create new auth token
	NewAuth(c, userId)
	return nil
}

func AuthLogout(c *gin.Context) {

	// Consume the nonce to invalidate the token
	token, err := c.Cookie(AUTH_COOKIE_NAME)

	if err != nil {
		slog.Warn("AuthLogout: No auth token found to consume nonce", "error", err)
	} else {
		claims, err := DecodeAuthJWT(token)
		if err == nil {
			nonce.Store.Consume(c.Request.Context(), claims.ID)
		}
	}

	// Clear auth cookie by setting it to expire in the past
	c.SetCookie(
		AUTH_COOKIE_NAME,
		"",
		-1,
		"/",
		"",
		false,
		true,
	)
}

// RequireAuth creates middleware that requires authentication.
// Redirects to login page if not authenticated.
func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check for existing token
		uid, exists := c.Get("userID")
		if !exists || uid == "" {
			slog.Warn("RequireAuth: No user ID found in context")
			loginPage := loginUrl(c)
			c.Redirect(http.StatusFound, loginPage)
			c.Abort()
			return
		}
		userIdStr, ok := uid.(string)
		if !ok {
			errorPage(c, http.StatusInternalServerError, "Invalid user context")
			return
		}
		slog.Debug("RequireAuth: Authenticated user", "userID", userIdStr)
		c.Next()
	}
}

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check for existing token
		uid, err := verifyAuth(c)
		if err != nil {
			slog.Warn("AuthMiddleware: Invalid or missing auth token", "error", err)
			c.AbortWithStatusJSON(AUTH_FAIL_STATUS, gin.H{
				"error": "unauthorized",
			})
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

	r.POST("/logout", AuthMiddleware(), func(c *gin.Context) {
		AuthLogout(c)
		c.Redirect(303, "/")
	})
}
