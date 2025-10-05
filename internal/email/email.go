package email

// Package email provides functionality to send emails using SMTP.
// Example usage:
//
//	cfg := &email.SMTPConfig{
//		Host:     "smtp.example.com",
//		Port:     "587",
//		Username: "your-username",
//		Password: "your-password",
//		From:     "your-email@example.com",
//	}
//
//	client, err := email.NewClient(cfg)
//	if err != nil {
//		log.Fatalf("Failed to create email client: %v", err)
//	}
//
//	msg := &email.Message{
//		To:      []string{"recipient@example.com"},
//		Subject: "Test Email",
//		HTML:    "<h1>Hello</h1><p>This is a test email.</p>",
//	}
//
//	if err := client.Send(msg); err != nil {
//		log.Fatalf("Failed to send email: %v", err)
//	}

import (
	"fmt"
	"log/slog"
	"strconv"

	"github.com/inbucket/html2text"
	"github.com/wneessen/go-mail"
)

// SMTPConfig represents an email client configuration
type SMTPConfig struct {
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	From     string `mapstructure:"from"`
}

// EmailClient represents an email client
type EmailClient struct {
	cfg    *SMTPConfig
	client *mail.Client
}

// Message represents an email message
type Message struct {
	To      []string
	Subject string
	HTML    string
	Text    string // optional, will be auto-generated from HTML if empty
}

// NewClient creates a new email client
func NewClient(cfg *SMTPConfig) (*EmailClient, error) {
	portInt, err := strconv.Atoi(cfg.Port)
	if err != nil {
		return nil, fmt.Errorf("invalid port number: %w", err)
	}

	client, err := mail.NewClient(cfg.Host,
		mail.WithPort(portInt),
		mail.WithUsername(cfg.Username),
		mail.WithPassword(cfg.Password),
		// mail.WithSMTPAuth(mail.SMTPAuthPlain),
		// mail.WithTLSPolicy(mail.TLSMandatory),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create mail client: %w", err)
	}
	return &EmailClient{
		cfg:    cfg,
		client: client,
	}, nil
}

// Compose creates a mail.Msg from a Message
func (c *EmailClient) Compose(Message *Message) (*mail.Msg, error) {
	if Message == nil {
		return nil, fmt.Errorf("message is nil")
	}

	m := mail.NewMsg()
	m.From(c.cfg.From)
	m.To(Message.To...)
	m.Subject(Message.Subject)

	// Auto-generate plain text from HTML if Text is empty
	if Message.Text == "" {
		text, err := htmlToText(Message.HTML)
		if err != nil {
			slog.Error("failed to convert HTML to text", "error", err)
		} else {
			Message.Text = text
		}
	}

	if Message.HTML != "" {
		m.SetBodyString(mail.TypeTextHTML, Message.HTML)
		if Message.Text != "" {
			m.AddAlternativeString(mail.TypeTextPlain, Message.Text)
		}
	} else if Message.Text != "" {
		m.SetBodyString(mail.TypeTextPlain, Message.Text)
	} else {
		slog.Warn("both HTML and Text content are empty")
		return nil, fmt.Errorf("both HTML and Text content are empty")
	}

	return m, nil
}

// Send sends an email message
func (c *EmailClient) Send(msg *Message) error {
	m, err := c.Compose(msg)
	if err != nil {
		return fmt.Errorf("failed to compose email: %w", err)
	}

	if err := c.client.DialAndSend(m); err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}
	return nil
}

// htmlToText converts HTML to plain text
func htmlToText(htmlContent string) (string, error) {
	text, err := html2text.FromString(htmlContent, html2text.Options{
		PrettyTables: true,
		OmitLinks:    false,
	})
	if err != nil {
		slog.Error("failed to convert HTML to text", "error", err)
		return "", err
	}
	return text, nil
}
