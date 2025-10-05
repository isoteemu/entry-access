package routes

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	access "entry-access-control/internal/access"
	. "entry-access-control/internal/utils"
)

// var authFailures = prometheus.NewCounter(prometheus.CounterOpts{
// 	Name: "auth_failures_total",
// 	Help: "Total number of authentication failures",
// })

type loginForm struct {
	Email string `form:"email" binding:"required,email"`
	Claim string `form:"claim" binding:"required"`
}

type emailLoginLink struct {
	EntryName string // Name of the entry point (e.g., Ag C331)
	Link      string // The actual login link URL
	Expires   string // Expiration time of the login link
	IP        string // IP address of the user
	Location  string // Location of the user
}

func EmailLoginRoute(r *gin.RouterGroup) {

	r.GET("/login", func(c *gin.Context) {
		c.HTML(200, "email_login.html.tmpl", gin.H{})
	})

	r.POST("/login", func(c *gin.Context) {
		// Handle email login form submission
		email := c.PostForm("email")
		if email == "" {
			c.HTML(400, "email_login.html.tmpl", gin.H{"error": "Email is required"})
			return
		}

		// TODO validate user can access premise
		if err := access.ValidEmail(email); err != nil {
			switch err {
			case access.ErrMissingEmail:
				slog.Warn("Email is missing", "email", email, "ip", c.ClientIP())
				c.HTML(400, "email_login.html.tmpl", gin.H{"error": "Email is required"})
			case access.ErrInvalidEmail:
				slog.Warn("Email is invalid", "email", email, "ip", c.ClientIP())
				c.HTML(400, "email_login.html.tmpl", gin.H{"error": "Invalid email format"})
			default:
				slog.Error("Failed to validate email", "error", err, "email", email, "ip", c.ClientIP())
				c.HTML(500, "email_login.html.tmpl", gin.H{"error": "Internal server error"})
			}
			return
		}

		// Render template with a message
		_, err := RenderTemplate(c, "login_link.html.tmpl", gin.H{"message": "If the email is registered, a login link has been sent."})
		if err != nil {
			slog.Error("Failed to render email login template", "error", err)
			c.HTML(500, "email_login.html.tmpl", gin.H{"error": "Internal server error"})
			return
		}

		// TODO: Check if user can access the entry
		// if !access.CanAccessEntry(userId, entryId) {
		// 	slog.Warn("User does not have access to the entry", "email", email, "entryId", entryId, "ip", c.ClientIP())
		// 	c.HTML(403, "email_login.html.tmpl", gin.H{"error": "You do not have access to this entry"})
		// 	return
		// }

		// TODO Generate a login link

		// Read CSS
		// css, err := os.ReadFile(DIST_DIR + "/assets/email.css")
		// if err != nil {
		// 	slog.Error("Failed to read email CSS", "error", err)
		// // }

		// messageInfo := emailLoginLink{
		// 	CSS:       "", // Add your CSS styles here
		// 	EntryName: "Ag C331",
		// 	Link:      "https://example.com/verify?token=abc123",
		// 	Expires:
		// 	IP:        c.ClientIP(),
		// 	Location:  "Unknown",
		// }

		// c.HTML(200, "email_login.html.tmpl", messageInfo)
	})

	r.GET("/verify/:token", func(c *gin.Context) {
		// Handle email verification link
		token := c.Param("token")
		if token == "" {
			slog.Warn("Email verification token is missing", "token", token, "ip", c.ClientIP())
			c.HTML(400, "email_login.html.tmpl", gin.H{"error": "Token is required"})
			return
		}

		// Verify the token and log the user in
		// TODO

		renewAuth(c, "user-id-from-token", true) // Replace with actual user ID from token

		// On success, redirect to entryway page
	})

}
