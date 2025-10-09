package routes

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/argon2"

	access "entry-access-control/internal/access"
	. "entry-access-control/internal/config"
	"entry-access-control/internal/email"
	"entry-access-control/internal/jwt"
	. "entry-access-control/internal/utils"
)

// TODO: Get actual entry ID
const ENTRY_ID = "Ag C331"

// If not runninng in production, use this test user to skip email sending
// and just print the OTP code to the log.
const TEST_USER = "user@example.com"

// var authFailures = prometheus.NewCounter(prometheus.CounterOpts{
// 	Name: "auth_failures_total",
// 	Help: "Total number of authentication failures",
// })

const LINK_TTL = time.Duration(10) * time.Minute

const EMAIL_TITLE = "Access code for %s"

// Salt for SAS key derivation. Used to prevent rainbow table attacks.
const SAS_KEY_SALT = "Ð¥ðVwj¯xR¨Øò\"9îzE5B:ëø1K*,EöþJjM"

type emailLoginLink struct {
	EntryName  string  // Name of the entry point (e.g., Ag C331)
	Link       string  // The actual login link URL
	EntryCode  string  // The entry code for the login link
	Expires    string  // Expiration time of the login link
	LinkTTL    float64 // Link time-to-live in minutes
	IP         string  // IP address of the user
	IPLocation string  // Location of the user
}

var emailLoginVerifyStore NonceStoreInterface

func loginErr(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"error": message})
}

// Encrypt the OTP token as hash
func otpEncode(data string, key string) string {
	// Derive key from secret.
	derivedKey := argon2.IDKey(
		[]byte(key),
		[]byte(SAS_KEY_SALT),
		3,       // time (number of iterations)
		64*1024, // memory in KB (64 MB)
		4,       // parallelism
		32,      // key length in bytes
	)

	h := hmac.New(sha256.New, derivedKey)
	h.Write([]byte(data))
	key = base64.StdEncoding.EncodeToString(h.Sum(nil))
	return key
}

// Verify the OTP token hash
func otpVerify(data string, key string, hash string) bool {
	expectedHash := otpEncode(data, key)
	return hmac.Equal([]byte(expectedHash), []byte(hash))
}

// generateOTP generates a random 6-digit OTP as a string.
func generateOTP() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// Login user by renewing auth cookie and consuming the claim nonce
func login(c *gin.Context, claim jwt.AccessCodeClaim) {
	slog.Info("User logged in via email verification", "email", claim.Email)
	renewAuth(c, claim.Email, true)
	jwt.ConsumeClaimNonce(&claim.RegisteredClaims)
}

func EmailLoginRoute(r *gin.RouterGroup) {

	emailLoginVerifyStore, err := NewStore(Cfg)
	if err != nil {
		slog.Error("Failed to create email login verify store", "error", err)
		panic("Failed to create email login verify store")
	}

	r.GET("/login", func(c *gin.Context) {
		// Collect necessary info for email
		c.HTML(200, "email_login.html.tmpl", gin.H{
			"LinkTTL": LINK_TTL.Minutes(),
		})
	})

	// Too lazy to support both HTML and JSON for now, just returns JSON
	r.POST("/login", func(c *gin.Context) {
		// Handle emailAddr login form submission
		emailAddr := c.PostForm("email")
		if emailAddr == "" {
			loginErr(c, 400, "Email is required")
			return
		}

		// Remove leading and trailing spaces
		emailAddr = strings.Trim(emailAddr, " ")

		// TODO validate user can access premise
		if err := access.ValidEmail(emailAddr); err != nil {
			switch err {
			case access.ErrMissingEmail:
				slog.Warn("Email is missing", "email", emailAddr, "ip", c.ClientIP())
				loginErr(c, 400, "Email is required")
			case access.ErrInvalidEmail:
				slog.Warn("Email is invalid", "email", emailAddr, "ip", c.ClientIP())
				loginErr(c, 400, "Invalid email format")
			default:
				slog.Error("Failed to validate email", "error", err, "email", emailAddr, "ip", c.ClientIP())
				loginErr(c, 500, "Internal server error")
			}
			return
		}

		// TODO: Check if user can access the entry
		// if !access.CanAccessEntry(userId, entryId) {
		// 	slog.Warn("User does not have access to the entry", "email", email, "entryId", entryId, "ip", c.ClientIP())
		// 	c.HTML(403, "email_login.html.tmpl", gin.H{"error": "You do not have access to this entry"})
		// 	return
		// }

		entryId := ENTRY_ID

		expires := time.Now().Add(LINK_TTL).Format(time.RFC3339)

		otp, err := generateOTP()
		if err != nil {
			slog.Error("Failed to generate OTP", "error", err)
			loginErr(c, 500, "Internal server error: failed to generate OTP")
			return
		}

		code := otpEncode(otp, Cfg.Secret)

		claim := jwt.NewAccessCodeClaim(code, emailAddr, entryId, uint(LINK_TTL.Seconds()))

		token, err := jwt.GenerateJWT(claim)
		if err != nil {
			slog.Error("Failed to generate OTP claim token", "error", err)
			loginErr(c, 500, "Internal server error: failed to generate OTP claim")
			return
		}

		link := UrlFor(c, "/auth/email/verify/"+token)

		slog.Debug("Generated email login link and OTP", "email", emailAddr, "link", link, "otp", otp, "expires", expires)

		// Collect necessary info for email
		data := emailLoginLink{
			EntryName:  entryId, // TODO: Get actual entry name
			Link:       link,
			EntryCode:  otp, // text version of the OTP
			Expires:    expires,
			LinkTTL:    LINK_TTL.Minutes(),
			IP:         c.ClientIP(),
			IPLocation: "", // TODO: Implement IP to location lookup
		}

		// Render email template
		emailMsg, err := RenderTemplate(c, "login_link.html.tmpl", data)
		if err != nil {
			slog.Error("Failed to render email login template", "error", err, "data", data)
			loginErr(c, 500, "Internal server error: failed to render template")
			return
		}
		emailTitle := fmt.Sprintf(EMAIL_TITLE, template.HTMLEscapeString(data.EntryName))

		// Send email with login link
		client, err := email.NewClient(Cfg.Email)
		if err != nil {
			slog.Error("Failed to create email client", "error", err)
			loginErr(c, 500, "Internal server error: failed to create email client")
			return
		}
		msg := &email.Message{
			To:      []string{emailAddr},
			Subject: emailTitle,
			HTML:    emailMsg,
		}

		if emailAddr == TEST_USER && os.Getenv("GIN_MODE") != "release" {
			// In debug mode, skip sending email for the example address
			slog.Debug("Debug mode: skipping email send", "to", emailAddr, "subject", emailTitle, "body", emailMsg)
			slog.Info("Use the following OTP code to login", "otp", otp, "link", link)
		} else {
			err = client.Send(msg)
			if err != nil {
				slog.Error("Failed to send email", "error", err, "to", emailAddr)
				loginErr(c, 500, "Internal server error: failed to send email")
				return
			}

			slog.Info("Sent login link email", "to", emailAddr)
		}

		// Return token for OTP validation
		c.JSON(200, gin.H{
			"message":  "Login link sent",
			"otpclaim": token,
			"id":       claim.ID, // It could be extracted from the token, but this is more explicit
		})

	})

	// Check if the user has clicked the link or submitted the OTP code
	r.GET("/status/:token", func(c *gin.Context) {

		userID, err := verifyAuth(c)
		if err == nil && userID != "" {
			// Already authenticated, redirect to entryway page
			c.JSON(200, gin.H{
				"status": "authenticated",
			})
			return
		}

		// Decode JWT token from URL parameter
		token := c.Param("token")
		if token == "" {
			slog.Warn("Email status check token is missing", "token", token)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Token is required"})
			return
		}

		claim, err := jwt.DecodeAccessCodeJWT(token)
		if err != nil {
			slog.Warn("Failed to decode email status check token", "error", err, "ip", c.ClientIP())
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to decode token. Please request a new login link."})
			return
		}

		// Not authenticated, start SSE to wait for OTP verification
		// Set SSE headers
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.Writer.Header().Set("Transfer-Encoding", "chunked") // Important for streaming

		c.Writer.WriteHeader(http.StatusOK)

		// Ensure the connection is closed when the client disconnects
		clientGone := c.Request.Context().Done()

		// Start the event loop
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Format the data according to SSE specification
				// Data must start with 'data: ' and end with '\n\n'
				// We are sending a JSON string for the status

				var data = gin.H{
					"status": "pending",
				}
				// Check if the user has verified the OTP by looking up the nonce store
				// If verified, set status to "authenticated"

				userID, err := verifyAuth(c)
				if err == nil && userID != "" {
					data["status"] = "authenticated"
					// No need to redirect to actual entryway page, email link has done that
					data["redirect"] = UrlFor(c, "/entry/success")
				}

				// Check if the OTP claim ID has been marked as verified
				if emailLoginVerifyStore.Exists(c.Request.Context(), claim.ID) {
					data["status"] = "authenticated"
					// No need to redirect to actual entryway page, email link has done that
					data["redirect"] = UrlFor(c, "/entry/success")
				}

				eventData, err := json.Marshal(data)
				if err != nil {
					slog.Error("Failed to marshal SSE data", "error", err)
					return
				}
				eventData = append([]byte("data: "), eventData...)
				eventData = append(eventData, []byte("\n\n")...)

				// Write the event to the response stream
				_, err = io.WriteString(c.Writer, string(eventData))
				if err != nil {
					// Client has likely disconnected
					slog.Debug("SSE client disconnected", "error", err)
					return
				}
				// Flush the buffer to ensure the data is sent immediately
				c.Writer.Flush()
				if data["status"] == "authenticated" {
					slog.Debug("SSE client authenticated, closing connection")
					return
				}
			case <-clientGone:
				// Client closed the connection (e.g., closed the tab)
				slog.Debug("SSE client disconnected")
				return
			}
		}

	})

	r.POST("/verify", func(c *gin.Context) {
		// Handle email verification link submission
		otp := c.PostForm("otp")
		if otp == "" {
			slog.Info("OTP code is missing")
			loginErr(c, 400, "OTP Code is required")
			return
		}

		if len(otp) != 6 {
			slog.Debug("OTP code format is invalid", "otp", otp)
			loginErr(c, 400, "Invalid OTP code format")
			return
		}
		claim := c.PostForm("otpclaim")
		if claim == "" {
			slog.Warn("OTP claim is missing")
			loginErr(c, 400, "OTP Claim is required")
			return
		}

		// Decode claim
		// TODO: Do not consume the claim until OTP is verified
		emailClaim, err := jwt.DecodeAccessCodeJWT(claim)
		if err != nil {
			if err == jwt.ErrInvalidNonce {
				slog.Info("OTP claim token has been used", "error", err)
				loginErr(c, 400, "Code has been already been used. Please request a new login link.")
				return
			} else {
				slog.Warn("Failed to decode OTP claim", "error", err)
				loginErr(c, 400, "Failed to decode OTP claim.")
			}
			return
		}

		// Check that the code matches
		expected := emailClaim.Verify
		if !otpVerify(otp, Cfg.Secret, expected) {
			slog.Info("OTP code is invalid", "otp", otp)
			loginErr(c, 400, "Invalid OTP code. Please check and try again.")
			return
		}

		slog.Info("User logged in via email OTP", "email", emailClaim.Email)

		// TODO: generate new EntryWay claim
		// Quote the entry ID for URL
		entry_url := template.URLQueryEscaper("...")

		login(c, *emailClaim)

		// TODO: Redirect to entryway page
		// Generate new EntryWay claim

		c.JSON(200, gin.H{
			"status":   "success",
			"message":  "OTP verification successful",
			"redirect": UrlFor(c, "/entry/"+entry_url),
		})
	})

	// TODO: Implement GET /verify/:token route to handle email link clicks
	r.GET("/verify/:token", func(c *gin.Context) {
		// Handle email verification link
		token := c.Param("token")
		if token == "" {
			slog.Warn("Email verification token is missing", "token", token, "ip", c.ClientIP())
			c.HTML(400, "email_login.html.tmpl", gin.H{"error": "Token is required"})
			return
		}

		emailClaim, err := jwt.DecodeAccessCodeJWT(token)
		if err != nil {
			if err == jwt.ErrInvalidNonce {
				slog.Info("Email verification token has been used", "error", err, "ip", c.ClientIP())
				c.HTML(400, "email_login.html.tmpl", gin.H{"error": "Link has been already been used. Please request a new login link."})
				return
			} else {
				slog.Warn("Failed to decode email verification token", "error", err, "ip", c.ClientIP())
				c.HTML(400, "email_login.html.tmpl", gin.H{"error": "Failed to decode token. Please request a new login link."})
			}
			return
		}

		slog.Info("User clicked email link", "email", emailClaim.Email)

		// Store the ID of the clicked link to allow polling to detect it
		ttl := time.Duration(emailClaim.ExpiresAt.Unix()-time.Now().UTC().Unix()) * time.Second
		emailLoginVerifyStore.Put(c.Request.Context(), emailClaim.ID, ttl)

		// TODO: Check the entry attempted to access
		//  - Redirect to entry page
		//  - Add user id into SSE polling to auto-login

		// On success, redirect to entryway page
	})

}
