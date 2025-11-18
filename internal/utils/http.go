package utils

import (
	"bytes"
	"fmt"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
)

// Helper function to generate a URL for a given path
func UrlFor(c *gin.Context, path string, args ...map[string]any) string {
	baseUrl := c.MustGet("BaseURL").(string)

	// Check for "/" prefix in path
	path = strings.TrimPrefix(path, "/")
	// Ensure baseUrl ends with "/"
	if !strings.HasSuffix(baseUrl, "/") {
		baseUrl += "/"
	}

	// Append query parameters if provided
	if len(args) > 0 {
		params := args[0]
		if len(params) > 0 {
			var queryParts []string
			for key, value := range params {
				queryParts = append(queryParts, fmt.Sprintf(
					"%s=%s",
					url.QueryEscape(key),
					url.QueryEscape(fmt.Sprintf("%v", value))))
			}
			queryString := strings.Join(queryParts, "&")
			path += "?" + queryString
		}
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
