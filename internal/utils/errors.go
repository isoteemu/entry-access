package utils

import "errors"

var (
	// Storage provider errors
	ErrStorageProviderNotFound = errors.New("storage provider not available")
	ErrInvalidStorageProvider  = errors.New("invalid storage provider")
)
