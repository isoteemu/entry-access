package utils

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"

	"github.com/gin-gonic/gin"
)

// Helper function to generate a URL for a given path
func UrlFor(c *gin.Context, path string) string {
	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	// Check for "/" prefix in path
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return fmt.Sprintf("%s://%s%s", scheme, c.Request.Host, path)
}

// GetBaseURL automatically detects the base URL from the request
func GetBaseURL(c *gin.Context, configBaseURL string) string {
	// If BaseURL is explicitly configured, use it
	if configBaseURL != "" {
		return configBaseURL
	}

	// Auto-detect from request
	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}

	return fmt.Sprintf("%s://%s", scheme, c.Request.Host)
}

func RenderTemplate(c *gin.Context, tmplName string, data any) (string, error) {
	var buf bytes.Buffer
	tmpl := c.MustGet("html").(*template.Template)
	err := tmpl.ExecuteTemplate(&buf, tmplName, data)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
