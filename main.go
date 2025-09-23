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
	log.Printf("Token expiry time: %d seconds", cfg.TokenExpiry)
	// return time.Now().UTC().Add(time.Duration(cfg.TokenExpiry) * time.Second).Unix()
	return time.Now().UTC().Add(time.Duration(cfg.TokenExpiry) * time.Minute)
}

func genEntryToken(entryID string) (string, error) {
	nonce, err := genNonce()
	if err != nil {
		return "", err
	}

	expiry := getTokenExpiry()

	claim := &EntryClaim{
		EntryID: entryID,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        nonce,
			ExpiresAt: jwt.NewNumericDate(expiry),
			IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
		},
	}

	token := jwt.NewWithClaims(tokenSignatureAlg, claim)

	JWTSecret := []byte(cfg.Secret)

	return token.SignedString(JWTSecret)
}

// Parse JWT
// <https://pkg.go.dev/github.com/golang-jwt/jwt/v5#example-NewWithClaims-RegisteredClaims>
func decodeJWT(token string) (*EntryClaim, error) {
	parsedToken, err := jwt.ParseWithClaims(token, &EntryClaim{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		JWTSecret := []byte(cfg.Secret)
		return JWTSecret, nil
	}, jwt.WithValidMethods([]string{tokenSignatureAlg.Alg()}))

	if err != nil {
		return nil, err
	} else if parsedToken == nil || !parsedToken.Valid {
		return nil, ErrNonValidToken
	} else if claims, ok := parsedToken.Claims.(*EntryClaim); ok {
		return claims, nil
	}

	return nil, ErrInvalidClaimType

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
	claims, err := decodeJWT(qrCode)
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

}
