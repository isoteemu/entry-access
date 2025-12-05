package routes

import (
	"errors"
	"net/http"

	"entry-access-control/internal/jwt"
)

// HTTPError represents an error with an associated HTTP status code and user message
type HTTPError struct {
	Err        error    // The underlying error
	StatusCode int      // HTTP status code
	Message    string   // User-friendly message
	StopCodes  []string // Optional stop codes for client-side handling
	Internal   bool     // Whether this is an internal error (hide details from user)
}

// ErrorInfo contains error metadata for user-facing errors
type ErrorInfo struct {
	Message   string   // User-friendly message
	StopCodes []string // Optional stop codes for client-side application
}

// Error implements the error interface
func (e *HTTPError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Message
}

// Unwrap returns the underlying error
func (e *HTTPError) Unwrap() error {
	return e.Err
}

// NewHTTPError creates a new HTTPError
func NewHTTPError(statusCode int, err error, message string, stopCodes ...string) *HTTPError {
	return &HTTPError{
		Err:        err,
		StatusCode: statusCode,
		Message:    message,
		StopCodes:  stopCodes,
		Internal:   statusCode >= 500,
	}
}

// Routes-specific errors (that don't conflict with other packages)
var (
	// Authentication errors
	ErrUnauthorized       = errors.New("unauthorized")
	ErrTokenExpired       = errors.New("token expired")
	ErrInvalidCredentials = errors.New("invalid credentials")

	// Authorization errors
	ErrForbidden               = errors.New("forbidden")
	ErrInsufficientPermissions = errors.New("insufficient permissions")

	// Device provisioning errors
	ErrDeviceIDRequired      = errors.New("device_id is required")
	ErrDevicePendingApproval = errors.New("device pending approval")
	ErrDeviceRejected        = errors.New("device rejected")
	ErrDeviceStatusUnknown   = errors.New("unknown device status")
	ErrFailedToCreateDevice  = errors.New("failed to create device")
	ErrDeviceNotFound        = errors.New("device not found")
	ErrClientIPMismatch      = errors.New("client IP mismatch")

	// Validation errors
	ErrInvalidRequest   = errors.New("invalid request")
	ErrMissingParameter = errors.New("missing required parameter")
	ErrInvalidParameter = errors.New("invalid parameter")

	// Internal errors
	ErrInternalServer     = errors.New("internal server error")
	ErrDatabaseError      = errors.New("database error")
	ErrServiceUnavailable = errors.New("service unavailable")

	// Storage provider errors
	ErrStorageProviderNotFound = errors.New("storage provider not found")
	ErrInvalidStorageProvider  = errors.New("invalid storage provider")
)

// errorStatusMap maps errors to HTTP status codes
var errorStatusMap = map[error]int{
	// 400 Bad Request
	ErrInvalidRequest:   http.StatusBadRequest,
	ErrMissingParameter: http.StatusBadRequest,
	ErrInvalidParameter: http.StatusBadRequest,
	ErrDeviceIDRequired: http.StatusBadRequest,

	// 401 Unauthorized
	ErrUnauthorized:       http.StatusUnauthorized,
	jwt.ErrNonValidToken:  http.StatusUnauthorized,
	ErrTokenExpired:       http.StatusUnauthorized,
	ErrInvalidCredentials: http.StatusUnauthorized,
	jwt.ErrInvalidNonce:   http.StatusUnauthorized,

	// 403 Forbidden
	ErrForbidden:               http.StatusForbidden,
	ErrInsufficientPermissions: http.StatusForbidden,
	ErrDeviceRejected:          http.StatusForbidden,
	ErrClientIPMismatch:        http.StatusForbidden,

	// 404 Not Found
	ErrUserNotFound:   http.StatusNotFound,
	ErrDeviceNotFound: http.StatusNotFound,

	// 202 Accepted (for pending operations)
	ErrDevicePendingApproval: http.StatusAccepted,

	// 500 Internal Server Error
	ErrInternalServer:          http.StatusInternalServerError,
	ErrDatabaseError:           http.StatusInternalServerError,
	ErrStorageProviderNotFound: http.StatusInternalServerError,
	ErrInvalidStorageProvider:  http.StatusInternalServerError,
	ErrDeviceStatusUnknown:     http.StatusInternalServerError,
	ErrFailedToCreateDevice:    http.StatusInternalServerError,

	// 503 Service Unavailable
	ErrServiceUnavailable: http.StatusServiceUnavailable,
}

// errorInfoMap maps errors to user-friendly messages and optional stop codes
var errorInfoMap = map[error]ErrorInfo{
	// Authentication
	ErrUnauthorized: {
		Message:   "Authentication required",
		StopCodes: []string{"AUTH_REQUIRED"},
	},
	jwt.ErrNonValidToken: {
		Message:   "Invalid or expired authentication token",
		StopCodes: []string{"AUTH_INVALID_TOKEN"},
	},
	ErrTokenExpired: {
		Message:   "Authentication token has expired",
		StopCodes: []string{"AUTH_TOKEN_EXPIRED"},
	},
	ErrInvalidCredentials: {
		Message:   "Invalid credentials provided",
		StopCodes: []string{"AUTH_INVALID_CREDENTIALS"},
	},
	jwt.ErrInvalidNonce: {
		Message:   "Invalid or reused token",
		StopCodes: []string{"AUTH_INVALID_NONCE"},
	},

	// Authorization
	ErrForbidden: {
		Message:   "Access denied",
		StopCodes: []string{"FORBIDDEN"},
	},
	ErrInsufficientPermissions: {
		Message:   "You don't have permission to perform this action",
		StopCodes: []string{"INSUFFICIENT_PERMISSIONS"},
	},

	// Device provisioning
	ErrDeviceIDRequired: {
		Message:   "Device ID is required",
		StopCodes: []string{"DEVICE_ID_REQUIRED"},
	},
	ErrDevicePendingApproval: {
		Message:   "Device is pending approval",
		StopCodes: []string{"DEVICE_PENDING"},
	},
	ErrDeviceRejected: {
		Message:   "Device access has been rejected",
		StopCodes: []string{"DEVICE_REJECTED"},
	},
	ErrDeviceNotFound: {
		Message:   "Device not found",
		StopCodes: []string{"DEVICE_NOT_FOUND"},
	},
	ErrClientIPMismatch: {
		Message:   "Request from unauthorized IP address",
		StopCodes: []string{"IP_MISMATCH"},
	},

	// Validation
	ErrInvalidRequest: {
		Message:   "Invalid request format",
		StopCodes: []string{"INVALID_REQUEST"},
	},
	ErrMissingParameter: {
		Message:   "Required parameter is missing",
		StopCodes: []string{"MISSING_PARAMETER"},
	},
	ErrInvalidParameter: {
		Message:   "Invalid parameter value",
		StopCodes: []string{"INVALID_PARAMETER"},
	},

	// Internal (no stop codes for internal errors)
	ErrInternalServer: {
		Message: "An internal error occurred",
	},
	ErrDatabaseError: {
		Message: "Database operation failed",
	},
	ErrStorageProviderNotFound: {
		Message: "Storage service is not available",
	},
	ErrInvalidStorageProvider: {
		Message: "Storage service configuration error",
	},
	ErrDeviceStatusUnknown: {
		Message: "Device status is unknown",
	},
	ErrFailedToCreateDevice: {
		Message: "Failed to create device record",
	},
	ErrServiceUnavailable: {
		Message: "Service is temporarily unavailable",
	},
}

// GetErrorStatus returns the HTTP status code for an error
func GetErrorStatus(err error) int {
	// Check if it's already an HTTPError
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.StatusCode
	}

	// Check direct match
	if status, ok := errorStatusMap[err]; ok {
		return status
	}

	// Check if error wraps a known error
	for knownErr, status := range errorStatusMap {
		if errors.Is(err, knownErr) {
			return status
		}
	}

	// Default to 500 Internal Server Error
	return http.StatusInternalServerError
}

// GetErrorInfo returns error information including message and stop codes
func GetErrorInfo(err error) ErrorInfo {
	// Check if it's an HTTPError with custom info
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		return ErrorInfo{
			Message:   httpErr.Message,
			StopCodes: httpErr.StopCodes,
		}
	}

	// Check direct match
	if info, ok := errorInfoMap[err]; ok {
		return info
	}

	// Check if error wraps a known error
	for knownErr, info := range errorInfoMap {
		if errors.Is(err, knownErr) {
			return info
		}
	}

	// For unknown errors, return a generic message for 5xx, specific for others
	status := GetErrorStatus(err)
	if status >= 500 {
		return ErrorInfo{Message: "An internal error occurred"}
	}
	return ErrorInfo{Message: err.Error()}
}

// GetErrorMessage returns a user-friendly message for an error
func GetErrorMessage(err error) string {
	return GetErrorInfo(err).Message
}

// GetErrorStopCodes returns stop codes for an error
func GetErrorStopCodes(err error) []string {
	return GetErrorInfo(err).StopCodes
}
