// Email link login provider
package auth

import (
	. "entry-access-control/utils"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

var req struct {
	Email string `json:"email" binding:"required"`
}

type EmailLoginConfig struct {
	// TTL for email login links in seconds
	EmailLinkTTL uint `mapstructure:"email_link_ttl"`

	// Minimum interval between sending emails to the same address in seconds
	EmailSendInterval uint `mapstructure:"email_send_interval"`
}

// Structure to prevent multiple emails being sent in quick succession
var emailSentCache = struct {
	sync.RWMutex
	m map[string]time.Time
}{m: make(map[string]time.Time)}

func generateEmailToken(email string) string {
	return "token-for-" + email
}

// Validate email format (basic validation)
func isValidEmail(email string) bool {
	return strings.Contains(email, "@") && strings.Contains(email, ".")
}

func generateEmail(email string, token string, c *gin.Context) {
	login_url := UrlFor(c, "/auth/email/verify?token="+token)

	// Send the email (omitted for brevity)
}

func NewEmailProvider() gin.RouterGroup {

	viper.SetDefault("EMAIL_LINK_TTL", 60*5) // 5 minutes

	r := gin.RouterGroup{}
	r.POST("/", func(c *gin.Context) {
		email := strings.TrimSpace(c.PostForm("email"))
		if email == "" || !isValidEmail(email) {
			c.JSON(400, gin.H{"error": "Invalid email address"})
			return
		}
		// Check if we have sent an email to this address recently
		emailSentCache.RLock()
		lastSent, exists := emailSentCache.m[email]
		emailSentCache.RUnlock()

		if exists && time.Since(lastSent) < time.Duration(viper.GetUint("EMAIL_SEND_INTERVAL"))*time.Second {
			c.JSON(429, gin.H{"error": "Email already sent recently. Please wait before requesting another link."})
			return
		}

		// Generate a token and send the email
		token := generateEmailToken(email)
		emailSentCache.Lock()
		emailSentCache.m[email] = time.Now()

		// Schedule removal from cache after interval
		go func(email string) {
			time.Sleep(time.Duration(viper.GetUint("EMAIL_SEND_INTERVAL")) * time.Second)
			emailSentCache.Lock()
			delete(emailSentCache.m, email)
			emailSentCache.Unlock()
		}(email)

		emailSentCache.Unlock()

		// Send the email (omitted for brevity)
		c.JSON(200, gin.H{"token": token})
	})

	return r
}
