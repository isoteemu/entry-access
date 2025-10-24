package routes

import "github.com/gin-gonic/gin"

func Health(r *gin.RouterGroup) {

	// Check current app version

	r.GET("/health", func(c *gin.Context) {
		msg := c.Query("ping")
		if msg == "" {
			msg = "pong"
		}

		c.JSON(200, gin.H{
			"message": msg,
		})
	})
}
