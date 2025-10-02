package app

import "errors"

var (
	// Token did not pass validation
	ErrInvalidToken = errors.New("invalid token")
)
