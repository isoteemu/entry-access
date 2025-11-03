package routes

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type errorStruct struct {
	Status  string   `json:"status"`
	Message string   `json:"message,omitempty"`
	Code    []string `json:"code,omitempty"`
}

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

// Returns a HTML response with merged data
func HTML(c *gin.Context, code int, name string, data any) {
	if data == nil {
		data = gin.H{}
	}
	data = H(c, data)
	c.HTML(code, name, data)
}

// General purpose error page renderer
func errorPage(c *gin.Context, code any, message string) {
	// Normalize code to both an int HTTP status and a string representation
	var status int
	var codeStr string

	// Duck type code
	switch v := code.(type) {
	case int:
		status = v
		codeStr = "HTTP_" + strconv.Itoa(v) // e.g., HTTP_404
	case string:
		codeStr = v
		if i, err := strconv.Atoi(v); err == nil {
			slog.Debug("errorPage: code string is numeric", "code", v)
			status = i
			errorPage(c, status, message)
			return
		} else {
			status = http.StatusInternalServerError
		}
	default:
		panic("errorPage called with invalid code type")
	}

	// Check if message is in error codes
	if message == "" {
		if _message, exists := ERR_CODES[codeStr]; exists {
			message = _message
		}
	}

	errstruct := errorStruct{
		Status:  "error",
		Message: message,
		Code:    []string{codeStr},
	}

	// Check the Accept header to determine response type
	accept := c.GetHeader("Accept")
	if accept == "application/json" {
		c.AbortWithStatusJSON(status, errstruct)
	} else {
		slog.Debug("Returning error page HTML", "code", status, "message", message)
		HTML(c, status, "error.html.tmpl", errstruct)
		c.Abort()
	}
}
