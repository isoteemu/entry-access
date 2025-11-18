package routes

import (
	"log/slog"
	"net/http"

	"entry-access-control/internal/access"

	"github.com/gin-gonic/gin"
)

// RequirePermission creates middleware that checks for specific permission.
func RequirePermission(resource, action string, opts ...map[string]interface{}) gin.HandlerFunc {
	return func(c *gin.Context) {

		userID, err := GetUser(c)
		if err != nil && err != ErrUserNotFound {
			errorPage(c, http.StatusInternalServerError, "Internal server error: "+err.Error())
			return
		}

		rbac := c.MustGet("rbac").(*access.RBAC)
		if !rbac.Can(userID, resource, action) {
			slog.Warn("Permission denied",
				"userID", userID,
				"resource", resource,
				"action", action)

			// Check if authenticated, redirect to login if not
			if userID == "" {
				slog.Warn("Unauthenticated permission attempt",
					"resource", resource,
					"action", action)

				loginPage := loginUrl(c)
				c.Redirect(http.StatusFound, loginPage)
				c.Abort()
				return
			} else {
				slog.Warn("Permission denied for authenticated user",
					"userID", userID,
					"resource", resource,
					"action", action)
			}

			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "permission denied",
				"details": map[string]string{
					"resource": resource,
					"action":   action,
				},
			})
			return
		}

		slog.Debug("Permission granted",
			"userID", userID,
			"resource", resource,
			"action", action)

		c.Next()
	}
}
