package jwt

import (
	"context"
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
	ctx := context.Background()
	// Consume nonce to prevent replay attacks
	// Note: This must be done after validating the token to avoid DoS attacks
	// with random nonces.
	if ok, err := NonceStore.Consume(ctx, claims.ID); err != nil || !ok {
		if err != nil {
			return nil, err
		}
		return nil, ErrInvalidNonce
	}
	return claims, nil
}

// AuthClaims represents the expected claims in the JWT token
type AuthClaims struct {
	UserID string `json:"uid"`
	// Must renew indicates if the token must be renewed. It will trigger nonce consumption.
	MustRenew bool `json:"renew,omitempty"`
	// Add other fields as necessary
	jwt.RegisteredClaims
}

func NewAuthClaims(uid string) *AuthClaims {
	return &AuthClaims{
		UserID: uid,
	}
}

func DecodeAuthJWT(tokenString string) (*AuthClaims, error) {

	claims, err := decodeJWT(tokenString, &AuthClaims{})
	if err != nil {
		return nil, err
	}
	// Note: We do not consume the nonce here as auth tokens are long-lived and
	// can be renewed. Nonce consumption is done during token renewal.
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
	return DeviceProvisionClaim{
		DeviceID:         deviceId,
		ClientIP:         clientIP,
		RegisteredClaims: mustCreateRegisteredClaim(5 * 60),
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

// Claim when user is requesting access code
type AccessCodeClaim struct {
	Verify           string `json:"verify"`
	Email            string `json:"email"`
	EntryID          string `json:"entry_id"`
	AuthenticateOnly bool   `json:"auth,omitempty"` // Whether to send authentication token after verification
	jwt.RegisteredClaims
}

func NewAccessCodeClaim(otpVerify string, email string, entryId string, ttl uint) AccessCodeClaim {
	return AccessCodeClaim{
		Verify:           otpVerify,
		Email:            email,
		EntryID:          entryId,
		RegisteredClaims: mustCreateRegisteredClaim(ttl),
	}
}

// NOTE: Nonce is  not consumed here. It must be consumed by the caller after validating the token.
func DecodeAccessCodeJWT(tokenString string, options ...jwt.ParserOption) (*AccessCodeClaim, error) {
	claims, err := decodeJWT(tokenString, &AccessCodeClaim{}, options...)
	if err != nil {
		return nil, err
	}
	return claims, nil
}

func ConsumeClaimNonce(claims *jwt.RegisteredClaims) error {
	ctx := context.Background()
	if ok, err := NonceStore.Consume(ctx, claims.ID); err != nil || !ok {
		if err != nil {
			return err
		}
		return ErrInvalidNonce
	}
	return nil
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

func decodeJWT[T jwt.Claims](tokenString string, claimsType T, options ...jwt.ParserOption) (T, error) {
	var zero T

	// Add default options
	options = append(options, jwt.WithValidMethods([]string{tokenSignatureAlg.Alg()}))

	parsedToken, err := jwt.ParseWithClaims(tokenString, claimsType, func(token *jwt.Token) (interface{}, error) {
		JWTSecret := []byte(Cfg.Secret)
		return JWTSecret, nil
	}, options...)

	if err != nil {
		return zero, err
	} else if parsedToken == nil || !parsedToken.Valid {
		return zero, ErrNonValidToken
	} else if claims, ok := parsedToken.Claims.(T); ok {
		return claims, nil
	}

	return zero, ErrInvalidClaimType
}
