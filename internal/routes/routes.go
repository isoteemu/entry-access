package routes

import (
	"encoding/json"
	"entry-access-control/internal/utils"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

const LOGIN_URL = "/auth/login"

// Merge into existing gin.H
func H(c *gin.Context, data any) gin.H {

	// Use a concrete gin.H for manipulation
	var h gin.H

	// If not map, create one
	switch d := data.(type) {
	case gin.H:
		h = d
	default:
		h = gin.H{}
		// Convert to map if possible, using JSON marshal/unmarshal as a workaround
		jsonData, _ := json.Marshal(data)

		err := json.Unmarshal(jsonData, &h)
		if err != nil {
			slog.Warn("H: failed to unmarshal data into gin.H", "error", err)
		}

		slog.Debug("H: merged data into new gin.H", "h", h)
	}

	// Add common data
	h["BaseURL"] = c.MustGet("BaseURL").(string)
	h["AppVersion"] = "v0.1.0" // TODO: set app version
	return h
}

func loginUrl(c *gin.Context) string {
	if c.Request.URL.Path == LOGIN_URL {
		AbortWithHTTPError(c, http.StatusBadRequest, ErrInvalidRequest, "Already on login page")
		return ""
	}

	return utils.UrlFor(c, "/auth/login", gin.H{"next": c.Request.URL.RequestURI()})
}

// Returns a HTML response with merged data
func HTML(c *gin.Context, code int, name string, data any) {
	if data == nil {
		data = gin.H{}
	}
	data = H(c, data)
	c.HTML(code, name, data)
}
