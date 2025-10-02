package app

import (
	. "entry-access-control/internal/config"
	. "entry-access-control/internal/utils"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidNonce     = errors.New("invalid nonce")
	ErrNonValidToken    = errors.New("token did not pass validation")
	ErrInvalidClaimType = errors.New("invalid claim type")
)

var tokenSignatureAlg = jwt.SigningMethodHS256

// Claim for entry access token
type EntryClaim struct {
	EntryID string `json:"entry_id"`
	jwt.RegisteredClaims
}

func NewEntryClaim(entryId string) EntryClaim {
	return EntryClaim{
		EntryID:          entryId,
		RegisteredClaims: mustCreateRegisteredClaim(Cfg.TokenTTL),
	}
}

func DecodeEntryJWT(tokenString string) (*EntryClaim, error) {

	claims, err := decodeJWT(tokenString, &EntryClaim{})
	if err != nil {
		return nil, err
	}
	if ok, err := NonceStore.Consume(nil, claims.ID); err != nil || !ok {
		if err != nil {
			return nil, err
		}
		return nil, ErrInvalidNonce
	}
	return claims, nil
}

type DeviceProvisionClaim struct {
	DeviceID string `json:"device_id"`
	ClientIP string `json:"client_ip"`
	jwt.RegisteredClaims
}

// Create a new device provision claim
// deviceId: ID of the device to be provisioned
// clientIP: IP address of the client requesting the token for preventing hijacking
func NewDeviceProvisionClaim(deviceId string, clientIP string) DeviceProvisionClaim {
	// TODO: Make TTL configurable
	var ttl uint = 5 * 60 // Device provision tokens are valid for 5 minutes
	return DeviceProvisionClaim{
		DeviceID:         deviceId,
		ClientIP:         clientIP,
		RegisteredClaims: mustCreateRegisteredClaim(ttl),
	}
}

func mustCreateRegisteredClaim(ttl uint) jwt.RegisteredClaims {
	nonce, err := Nonce(ttl + 10) // nonce TTL is slightly longer than token TTL to allow for clock skew
	if err != nil {
		panic(fmt.Sprintf("failed to generate nonce: %v", err))
	}

	return jwt.RegisteredClaims{
		ID:        nonce,
		IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
		ExpiresAt: jwtExpiry(ttl),
	}
}

// Convert TTL to time in future
func tokenTTL(ttl uint) time.Time {
	if ttl <= 0 {
		panic("invalid token TTL")
	}
	return time.Now().UTC().Add(time.Duration(ttl) * time.Second)
}

func jwtExpiry(ttl uint) *jwt.NumericDate {
	expiry := tokenTTL(ttl)
	return jwt.NewNumericDate(expiry)
}

// Generic JWT token generation function
func GenerateJWT(claims jwt.Claims) (string, error) {
	token := jwt.NewWithClaims(tokenSignatureAlg, claims)
	JWTSecret := []byte(Cfg.Secret)
	return token.SignedString(JWTSecret)
}

func decodeJWT[T jwt.Claims](tokenString string, claimsType T) (T, error) {
	var zero T

	parsedToken, err := jwt.ParseWithClaims(tokenString, claimsType, func(token *jwt.Token) (interface{}, error) {
		JWTSecret := []byte(Cfg.Secret)
		return JWTSecret, nil
	}, jwt.WithValidMethods([]string{tokenSignatureAlg.Alg()}))

	if err != nil {
		return zero, err
	} else if parsedToken == nil || !parsedToken.Valid {
		return zero, ErrNonValidToken
	} else if claims, ok := parsedToken.Claims.(T); ok {
		return claims, nil
	}

	return zero, ErrInvalidClaimType
}
