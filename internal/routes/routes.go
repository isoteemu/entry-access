package routes

import (
	"log/slog"

	"github.com/gin-gonic/gin"
)

// Merge into existing gin.H
func H(c *gin.Context, data gin.H) gin.H {
	if data == nil {
		data = gin.H{}
	}
	data["BaseURL"] = c.MustGet("BaseURL").(string)
	data["AppVersion"] = "v0.1.0" // TODO: set app version
	return data
}

// Returns a HTML response with merged data
func HTML(c *gin.Context, code int, name string, data gin.H) {
	if data == nil {
		data = gin.H{}
	}
	data = H(c, data)
	c.HTML(code, name, data)
}

func Ping(r *gin.RouterGroup) {

	// Check current app version

	r.GET("/ping", func(c *gin.Context) {
		msg := c.Query("ping")
		if msg == "" {
			msg = "pong"
		}

		authenticated := false

		userID, err := verifyAuth(c)
		if err != nil {
			slog.Warn("Ping: Invalid or missing auth token", "error", err)
		} else if userID != "" {
			authenticated = true
		}

		c.JSON(200, gin.H{
			"message":       msg,
			"authenticated": authenticated,
		})
	})
}
