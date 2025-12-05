package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/crypto/pbkdf2"
)

const (
	// KDF parameters
	kdfIterations = 100000         // PBKDF2 iteration count
	kdfKeyLength  = 32             // Derived key length in bytes (256 bits)
	kdfSaltPrefix = "device-id-v1" // Version prefix for salt domain separation
)

// deriveKey derives a cryptographic key from the secret using PBKDF2
func deriveKey(secret []byte, salt []byte) []byte {
	// Combine constant prefix with salt for domain separation
	fullSalt := append([]byte(kdfSaltPrefix), salt...)
	// Use PBKDF2 with SHA-256 for strong key derivation
	return pbkdf2.Key(secret, fullSalt, kdfIterations, kdfKeyLength, sha256.New)
}

func GenerateDeviceID(secret []byte) (string, error) {

	// Generate UUID
	uuidObj, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}
	uuidStr := uuidObj.String()

	// Create HMAC signature from the first part of UUID
	parts := strings.Split(uuidStr, "-")
	firstParts := strings.Join(parts[:4], "-") // Use first 4 parts for UUID base

	// Derive key from secret using the UUID first parts as salt
	key := deriveKey(secret, []byte(firstParts))

	h := hmac.New(sha256.New, key)
	h.Write([]byte(firstParts))
	signature := hex.EncodeToString(h.Sum(nil))[:12] // 12 chars to fit UUID format

	// Format: first-parts-of-uuid-signature (replace last part with signature)
	return fmt.Sprintf("%s-%s", firstParts, signature), nil
}

func VerifyDeviceID(deviceID string, secret []byte) bool {
	// Validate UUID format
	_, err := uuid.Parse(deviceID)
	if err != nil {
		slog.Error("VerifyDeviceID: failed to parse deviceID as UUID", "deviceID", deviceID, "error", err)
		return false
	}

	// Split deviceID into parts
	parts := strings.Split(deviceID, "-")
	if len(parts) != 5 { // uuid format (4 parts) + signature (1 part)
		return false
	}

	firstParts := strings.Join(parts[:4], "-")
	providedSig := parts[4]

	// Recalculate signature using derived key
	key := deriveKey(secret, []byte(firstParts))

	h := hmac.New(sha256.New, key)
	h.Write([]byte(firstParts))
	expectedSig := hex.EncodeToString(h.Sum(nil))[:12]

	return hmac.Equal([]byte(providedSig), []byte(expectedSig))
}
