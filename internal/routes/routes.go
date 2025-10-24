package routes

import (
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
