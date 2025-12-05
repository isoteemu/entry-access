package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

func GenerateDeviceID(secret []byte) (string, error) {
	// Generate UUID
	uuidObj, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}
	uuid := uuidObj.String()

	// Create HMAC signature
	h := hmac.New(sha256.New, secret)
	h.Write([]byte(uuid))
	signature := hex.EncodeToString(h.Sum(nil))[:16] // First 16 chars

	// Format: uuid-signature
	return fmt.Sprintf("%s-%s", uuid, signature), nil
}

func VerifyDeviceID(deviceID string, secret []byte) bool {
	parts := strings.Split(deviceID, "-")
	if len(parts) != 6 { // uuid (5 parts) + signature (1 part)
		return false
	}

	uuid := strings.Join(parts[:5], "-")
	providedSig := parts[5]

	// Recalculate signature
	h := hmac.New(sha256.New, secret)
	h.Write([]byte(uuid))
	expectedSig := hex.EncodeToString(h.Sum(nil))[:16]

	return hmac.Equal([]byte(providedSig), []byte(expectedSig))
}
