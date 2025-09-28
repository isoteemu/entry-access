package main

// https://www.golinuxcloud.com/golang-jwt/

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/joho/godotenv"
)

var cfg *Config
var nonceStore NonceStore

var tokenSignatureAlg = jwt.SigningMethodHS256

// NOTE: jti is used as nonce value
type EntryClaim struct {
	EntryID string `json:"entry_id"`
	jwt.RegisteredClaims
}

func (c *EntryClaim) GetEntryID() string {
	return c.EntryID
}

type DeviceProvisionClaim struct {
	DeviceID string `json:"device_id"`
	jwt.RegisteredClaims
}

// Token errors
// TODO: When raised, these should be logged as they are signs of hacking attempts
var (
	// Token did not pass validation
	ErrInvalidToken     = errors.New("invalid token")
	ErrNonValidToken    = errors.New("token did not pass validation")
	ErrInvalidClaimType = errors.New("invalid claim type")
)

// Get token expiry time. now + TokenExpiry
func getTokenExpiry() time.Time {
	log.Printf("Token expiry time: %d seconds", cfg.TokenTTL)
	// return time.Now().UTC().Add(time.Duration(cfg.TokenExpiry) * time.Second).Unix()
	return time.Now().UTC().Add(time.Duration(cfg.TokenTTL) * time.Minute)
}

// Generic JWT token generation function
func generateJWT(claims jwt.Claims) (string, error) {
	token := jwt.NewWithClaims(tokenSignatureAlg, claims)
	JWTSecret := []byte(cfg.Secret)
	return token.SignedString(JWTSecret)
}

// Generic function to create registered claims with nonce
func mustCreateRegisteredClaims() jwt.RegisteredClaims {
	nonce, err := genNonce()
	if err != nil {
		panic(fmt.Sprintf("failed to generate nonce: %v", err))
	}

	expiry := getTokenExpiry()

	return jwt.RegisteredClaims{
		ID:        nonce,
		ExpiresAt: jwt.NewNumericDate(expiry),
		IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
	}
}

func genEntryToken(entryID string) (string, error) {
	registeredClaims := mustCreateRegisteredClaims()

	claim := &EntryClaim{
		EntryID:          entryID,
		RegisteredClaims: registeredClaims,
	}

	return generateJWT(claim)
}

func GenDeviceProvisionToken(deviceID string) (string, error) {
	registeredClaims := mustCreateRegisteredClaims()

	claim := &DeviceProvisionClaim{
		DeviceID:         deviceID,
		RegisteredClaims: registeredClaims,
	}

	return generateJWT(claim)
}

func decodeJWT[T jwt.Claims](tokenString string, claimsType T) (T, error) {
	var zero T

	parsedToken, err := jwt.ParseWithClaims(tokenString, claimsType, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		JWTSecret := []byte(cfg.Secret)
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

// Specific function for decoding entry tokens (uses the generic function)
func decodeEntryJWT(token string) (*EntryClaim, error) {
	return decodeJWT(token, &EntryClaim{})
}

// Specific function for decoding device provision tokens
func DecodeDeviceProvisionJWT(token string) (*DeviceProvisionClaim, error) {
	return decodeJWT(token, &DeviceProvisionClaim{})
}

func consumeNonce(nonce string) (bool, error) {
	ctx := context.Background()
	return nonceStore.Consume(ctx, nonce)
}

func genNonce() (string, error) {
	nonce, err := GenerateNonce()
	if err != nil {
		return "", err
	}

	ctx := context.Background()
	if err := nonceStore.Put(ctx, nonce, nonceTTL); err != nil {
		log.Fatalf("store put error: %v", err)
	}
	return nonce, nil
}

// Initialize logger
func InitLogger(cfg *Config) {

	logLevel := strings.ToUpper(cfg.LogLevel)

	switch logLevel {
	case "DEBUG":
		slog.SetLogLoggerLevel(slog.LevelDebug)
	case "INFO":
		slog.SetLogLoggerLevel(slog.LevelInfo)
	case "WARN":
		slog.SetLogLoggerLevel(slog.LevelWarn)
	case "ERROR":
		slog.SetLogLoggerLevel(slog.LevelError)
	default:
		slog.SetLogLoggerLevel(slog.LevelInfo)
		slog.Warn("Invalid log level, defaulting to info", "log_level", cfg.LogLevel)
	}
}

func main() {
	// Load config
	var err error

	godotenv.Load()

	cfg, err = LoadConfig()
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	InitLogger(cfg)

	nonceStore, err = NewStore(cfg)
	if err != nil {
		log.Fatalf("Error creating nonce store: %v", err)
	}

	doorID := "Ag C331"

	// Generate a QR code for a door
	qrCode, err := genEntryToken(doorID)
	if err != nil {
		log.Fatalf("Error generating QR code: %v", err)
	}

	// Print the QR code
	fmt.Printf("Generated QR code: %s\n", qrCode)

	// Decode the QR code
	claims, err := decodeEntryJWT(qrCode)
	if err != nil {
		log.Fatalf("Error decoding claims: %v", err)
	}

	// Print the claims
	fmt.Printf("Decoded claims: %+v\n", claims)

	// Verify nonce
	nonce := claims.ID
	consumed, err := consumeNonce(nonce)
	if err != nil {
		log.Fatalf("Error consuming nonce: %v", err)
	}
	if !consumed {
		log.Fatalf("Nonce has already been consumed")
	} else {
		slog.Info("Nonce consumed successfully", "nonce", nonce)
	}

	server := HTTPServer()
	server.Run()
}
