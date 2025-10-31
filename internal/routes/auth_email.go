package routes

// OTP Handling

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"math/big"
	"net/http"
	"net/url"
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

	gojwt "github.com/golang-jwt/jwt/v5"
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

const (
	JWT_AUDIENCE_EMAIL_LINK  = "email_link"  // Audience for email link verification
	JWT_AUDIENCE_EMAIL_OTP   = "email_otp"   // Audience for email OTP verification
	JWT_AUDIENCE_EMAIL_LOGIN = "email_login" // Audience for logging user in
)

const (
	VERIFY_STATUS_ERROR         = "error"
	VERIFY_STATUS_EXPIRED       = "expired"
	VERIFY_STATUS_PENDING       = "pending"
	VERIFY_STATUS_CONFIRMED     = "confirmed"
	VERIFY_STATUS_AUTHENTICATED = "authenticated" // Not used, SSE doesn't need to react to this
)

// Map of error codes to user-friendly messages
var ErrorCodes = map[string]string{
	"VERIFY_TOKEN_USED":    "This login link has already been used. Please request a new link.",
	"VERIFY_TOKEN_EXPIRED": "This login link has expired or is invalid. Please request a new login link.",
	"EMAIL_TOKEN_MISSING":  "The email verification token is missing. Please request a new login link.",
}

type emailLoginLink struct {
	EntryName  string  // Name of the entry point (e.g., Ag C331)
	Link       string  // The actual login link URL
	EntryCode  string  // The entry code for the login link
	Created    string  // Creation time of the login link
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

type EventErr struct {
	Status string `json:"status"`
	Error  string `json:"error"`
}

// EventError sends an error event to the SSE client
func eventMessage(c *gin.Context, data any) {
	// Format the data according to SSE specification
	// Data must start with 'data: ' and end with '\n\n'
	// We are sending a JSON string for the status

	serialized, err := json.Marshal(data)
	if err != nil {
		slog.Error("Failed to marshal SSE event message", "error", err)
		return
	}

	serialized = append([]byte("data: "), serialized...)
	serialized = append(serialized, []byte("\n\n")...)

	c.Writer.Write(serialized)

	// Flush the buffer to ensure the data is sent immediately
	c.Writer.Flush()
}

// Generate a URL for showing door open
func SuccessUrl(c *gin.Context, entryId string, data ...map[string]interface{}) string {
	entryToken, err := genEntryToken(entryId)
	if err != nil {
		slog.Error("Failed to generate entry token", "error", err)
		c.AbortWithStatusJSON(500, gin.H{"error": "Internal server error"})
	}
	// Convert data to URL parameters
	params := ""
	if len(data) > 0 {
		paramList := []string{}
		for k, v := range data[0] {
			paramList = append(paramList, fmt.Sprintf("%s=%v", template.URLQueryEscaper(k), template.URLQueryEscaper(fmt.Sprintf("%v", v))))
		}
		if len(paramList) > 0 {
			params = "?" + strings.Join(paramList, "&")
		}
	}

	return UrlFor(c, fmt.Sprintf("/entry/%s%s", entryToken, params))
}

// isSafeUrl checks if the target URL is within the same origin as the base URL
func isSafeUrl(c *gin.Context, targetUrl string) bool {
	baseUrl := c.MustGet("BaseURL").(string)

	refUrl, _ := url.Parse(baseUrl)
	testUrl, err := url.Parse(targetUrl)
	if err != nil {
		slog.Debug("Failed to parse target URL for safety check", "url", targetUrl, "error", err)
		return false
	}

	// Check scheme
	if refUrl.Scheme != testUrl.Scheme {
		slog.Debug("Target URL scheme does not match base URL", "target_scheme", testUrl.Scheme, "base_scheme", refUrl.Scheme)
		return false
	}

	// Check host and path prefix
	return refUrl.Host == testUrl.Host && strings.HasPrefix(testUrl.Path, refUrl.Path)
}

func EmailLoginRoute(r *gin.RouterGroup) {

	emailLoginVerifyStore, err := NewStore(Cfg)
	if err != nil {
		slog.Error("Failed to create email login verify store", "error", err)
		panic("Failed to create email login verify store")
	}

	r.GET("/login", func(c *gin.Context) {

		var pageData = gin.H{
			"LinkTTL": LINK_TTL.Minutes(),
			"Error":   "",
		}

		// Check for error code in URL, display friendly message
		err := c.Query("error")
		if err != "" {
			if errorMsg, exists := ErrorCodes[err]; exists {
				pageData["Error"] = errorMsg
			} else {
				pageData["Error"] = "An unknown error occurred. Please try again."
				slog.Warn("Unknown error code in login URL", "error", err)
			}
		}

		// Collect necessary info for email
		c.HTML(200, "email_login.html.tmpl", pageData)
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
				slog.Warn("Email is missing", "email", emailAddr)
				loginErr(c, 400, "Email is required")
			case access.ErrInvalidEmail:
				slog.Warn("Email is invalid", "email", emailAddr)
				loginErr(c, 400, "Invalid email format")
			default:
				slog.Error("Failed to validate email", "error", err, "email", emailAddr)
				loginErr(c, 500, "Internal server error")
			}
			return
		}

		// Get user ID from access list
		if user, err := userExists(c, emailAddr); err != nil {
			slog.Warn("User not found", "email", emailAddr)
			loginErr(c, http.StatusUnauthorized, "User not found")
			return
		} else {
			slog.Debug("User found in access list", "email", emailAddr, "userID", user)
		}

		// TODO: Check if user can access the entry
		// if !access.CanAccessEntry(userId, entryId) {
		// 	slog.Warn("User does not have access to the entry", "email", email, "entryId", entryId)
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

		// Create two JWT claims: one for OTP verification, one for email link
		// Both claims contain the same info, but different audience to distinguish them
		// OTP claim is returned to the client for verification
		// Link claim is sent in the email link for auto-verification

		// Both claims have the same nonce, so consuming one will invalidate the other
		// This prevents reuse of either method

		baseClaim := jwt.NewAccessCodeClaim(code, emailAddr, entryId, uint(LINK_TTL.Seconds()))

		otpClaim := baseClaim
		otpClaim.Audience = []string{"email_otp"}
		otpToken, err := jwt.GenerateJWT(otpClaim)
		if err != nil {
			slog.Error("Failed to generate OTP claim token", "error", err)
			loginErr(c, 500, "Internal server error: failed to generate OTP token")
			return
		}

		linkClaim := baseClaim
		linkClaim.Audience = []string{"email_link"}
		linkToken, err := jwt.GenerateJWT(linkClaim)
		if err != nil {
			slog.Error("Failed to generate link claim token", "error", err, "audience", linkClaim.Audience)
			loginErr(c, 500, "Internal server error: failed to generate email link token")
			return
		}

		link := UrlFor(c, "/auth/email/verify/"+linkToken)

		slog.Debug("Generated email login link and OTP", "email", emailAddr, "link", link, "otp", otp, "expires", expires)

		// Collect necessary info for email
		data := emailLoginLink{
			EntryName:  entryId, // TODO: Get actual entry name
			Link:       link,
			EntryCode:  otp, // text version of the OTP
			Created:    time.Now().Format(time.RFC3339),
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
			"status":   "success",
			"message":  "Login link sent",
			"otpclaim": otpToken,
		})

	})

	// Check if the user has clicked the link or submitted the OTP code
	r.GET("/status", func(c *gin.Context) {

		// Decode JWT token from URL parameter
		token := c.Query("token")
		if token == "" {
			slog.Warn("Email status check token is missing", "token", token)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Token is required"})
			return
		}

		// Set SSE headers
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.Writer.Header().Set("Transfer-Encoding", "chunked") // Important for streaming
		c.Writer.Header().Set("X-Accel-Buffering", "no")      // Disable buffering for Nginx

		c.Writer.WriteHeader(http.StatusOK)

		// Ensure the connection is closed when the client disconnects
		clientGone := c.Request.Context().Done()

		claim, err := jwt.DecodeAccessCodeJWT(token, gojwt.WithAudience(JWT_AUDIENCE_EMAIL_OTP))
		if err != nil {
			slog.Warn("Failed to decode email status check token", "error", err)
			eventMessage(c, EventErr{
				Status: "error",
				Error:  "Failed to decode token. Please request a new login link.",
			})
			return
		}

		// Generate a login token for the user to use once verified
		loginClaim := *claim // Make a copy to avoid modifying the original
		loginClaim.Audience = []string{JWT_AUDIENCE_EMAIL_LOGIN}
		loginClaim.AuthenticateOnly = true
		loginToken, err := jwt.GenerateJWT(loginClaim)
		if err != nil {
			slog.Error("Failed to generate login claim token", "error", err, "audience", loginClaim.Audience)
			eventMessage(c, EventErr{
				Status: "error",
				Error:  "Internal server error. Please try again later.",
			})
			return
		}
		loginUrl := UrlFor(c, "/auth/email/verify/"+loginToken)

		// Start the event loop
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:

				// TODO: Uudelleenfaktorinti kesken

				var data = gin.H{
					"status": "pending",
				}

				// Check if the OTP claim ID has been marked as verified
				confirmed, err := emailLoginVerifyStore.Consume(c.Request.Context(), claim.ID)
				if confirmed && err == nil {
					data["status"] = VERIFY_STATUS_CONFIRMED
					data["redirect"] = loginUrl
				} else if err != nil {
					switch err {
					case &NonceMissingError{}:
						// Not verified yet, keep waiting
						data["status"] = VERIFY_STATUS_PENDING
					case &NonceExpiredError{}:
						data["status"] = VERIFY_STATUS_EXPIRED
						data["error"] = "Login link has expired. Please request a new login link."
					default:
						// Not found - assume not verified yet
						data["status"] = VERIFY_STATUS_PENDING
					}
				} else {
					slog.Warn("Email login verify store returned unexpected result", "confirmed", confirmed, "error", err)
				}

				// Send event to client
				eventMessage(c, data)

				if data["status"] == VERIFY_STATUS_CONFIRMED || data["status"] == VERIFY_STATUS_EXPIRED {
					slog.Debug("Ending SSE connection for email login status", "status", data["status"], "email", claim.Email)
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
		emailClaim, err := jwt.DecodeAccessCodeJWT(claim, gojwt.WithAudience(JWT_AUDIENCE_EMAIL_OTP))
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

	r.GET("/verify/:token", func(c *gin.Context) {
		// Handle email verification link
		token := c.Param("token")
		if token == "" {
			slog.Warn("Email verification token is missing", "token", token, "ip", c.ClientIP())
			c.Redirect(302, UrlFor(c, "/auth/email/login?error=EMAIL_TOKEN_MISSING"))
			return
		}

		// TODO: Improve logging based on audience
		emailClaim, err := jwt.DecodeAccessCodeJWT(token, gojwt.WithAudience(JWT_AUDIENCE_EMAIL_LINK, JWT_AUDIENCE_EMAIL_LOGIN))
		if err != nil {
			if err == jwt.ErrInvalidNonce {
				slog.Info("Email verification token has been used", "error", err, "ip", c.ClientIP())
				c.Redirect(302, UrlFor(c, "/auth/email/login?error=VERIFY_TOKEN_USED"))
				return
			} else {
				slog.Warn("Failed to decode email verification token", "error", err, "ip", c.ClientIP())
				c.Redirect(302, UrlFor(c, "/auth/email/login?error=VERIFY_TOKEN_EXPIRED"))
			}
			return
		}

		slog.Info("User clicked email link", "email", emailClaim.Email)

		// If the claim has AuthenticateOnly set, login user only and show success page
		if emailClaim.AuthenticateOnly {
			login(c, *emailClaim)
			entryID := emailClaim.EntryID

			// Redirect to entryway page
			c.Redirect(http.StatusFound, SuccessUrl(c, entryID))
		} else {
			// Store the ID of the clicked link to allow polling to detect it
			ttl := time.Duration(emailClaim.ExpiresAt.Unix()-time.Now().UTC().Unix()) * time.Second
			emailLoginVerifyStore.Put(c.Request.Context(), emailClaim.ID, ttl)

			//
		}

		// TODO: Check the entry attempted to access
		//  - Redirect to entry page
		//  - Add user id into SSE polling to auto-login

		// On success, redirect to entryway page
		c.JSON(200, gin.H{
			"status":   "success",
			"message":  "Email link verification successful. You can close this tab and return to the previous window.",
			"redirect": UrlFor(c, "/entry/success"),
		})
	})

}
