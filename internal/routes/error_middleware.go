package routes

import (
	"log/slog"

	"github.com/gin-gonic/gin"
)

type errorStruct struct {
	Succeed bool     `json:"success"`
	Status  string   `json:"status"`
	Message string   `json:"message,omitempty"`
	Code    []string `json:"code,omitempty"`
}

// ErrorHandler captures errors and returns a consistent JSON error response
// with appropriate HTTP status codes based on the error type
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next() // Process the request first

		// Check if any errors were added to the context
		if len(c.Errors) > 0 {
			// Use the last error (most recent)
			err := c.Errors.Last().Err

			// Get appropriate status code and error info
			statusCode := GetErrorStatus(err)
			errorInfo := GetErrorInfo(err)

			// Log the error with appropriate level based on status code
			if statusCode >= 500 {
				slog.Error("Request failed with server error",
					"error", err,
					"status", statusCode,
					"path", c.Request.URL.Path,
					"method", c.Request.Method,
				)
			} else if statusCode >= 400 {
				slog.Warn("Request failed with client error",
					"error", err,
					"status", statusCode,
					"path", c.Request.URL.Path,
					"method", c.Request.Method,
				)
			}

			// Only send the response if it hasn't been written yet
			if !c.Writer.Written() {
				response := errorStruct{
					Succeed: false,
					Status:  "error",
					Message: errorInfo.Message,
				}

				// // Add stop codes if present
				// if len(errorInfo.StopCodes) > 0 {
				// 	response.Code = errorInfo.StopCodes
				// }
				// Collect all the stop codes from all wrapped errors
				var stopCodes []string
				for _, _err := range c.Errors {
					errInfo := GetErrorInfo(_err.Err)
					stopCodes = append(stopCodes, errInfo.StopCodes...)
				}
				response.Code = stopCodes

				// Check the Accept header to determine response type
				accept := c.GetHeader("Accept")
				if accept == "application/json" {
					c.AbortWithStatusJSON(statusCode, response)
				} else {
					slog.Debug("Returning error page HTML", "code", statusCode, "message", errorInfo.Message)
					HTML(c, statusCode, "error.html.tmpl", response)
					c.Abort()
				}
			}
		}
	}
}

// AbortWithError is a helper function to abort the request with an error
// and add it to the Gin error chain for the ErrorHandler middleware
func AbortWithError(c *gin.Context, err error) {
	statusCode := GetErrorStatus(err)
	c.Error(err)
	c.Abort()
	// Set the status code so gin knows not to send 200
	c.Status(statusCode)
}

// AbortWithHTTPError is a helper to abort with a custom HTTPError
func AbortWithHTTPError(c *gin.Context, statusCode int, err error, message string, stopCodes ...string) {
	httpErr := NewHTTPError(statusCode, err, message, stopCodes...)
	c.Error(httpErr)
	c.Abort()
	c.Status(statusCode)
}
