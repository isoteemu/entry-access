package utils

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
)

// Helper function to generate a URL for a given path
func UrlFor(c *gin.Context, path string) string {
	baseUrl := c.MustGet("BaseURL").(string)

	// Check for "/" prefix in path
	path = strings.TrimPrefix(path, "/")
	// Ensure baseUrl ends with "/"
	if !strings.HasSuffix(baseUrl, "/") {
		baseUrl += "/"
	}

	return fmt.Sprintf("%s%s%s", baseUrl, "", path)
}

// RenderTemplate renders a template with the given name and data, returning the result as a string.
func RenderTemplate(c *gin.Context, tmplName string, data any) (string, error) {
	var buf bytes.Buffer
	// Get the template engine from Gin context
	tmpl := c.MustGet("HTML").(render.HTMLRender)
	err := tmpl.Instance(tmplName, data).(render.HTML).Template.ExecuteTemplate(&buf, tmplName, data)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
