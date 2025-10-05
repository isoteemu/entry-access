package email

import (
	"bytes"
	"fmt"
	"log/slog"
	"mime/multipart"
	"mime/quotedprintable"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"

	"github.com/inbucket/html2text"
)

// Client represents an email client
type Client struct {
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	From     string `mapstructure:"from"`
}

// Message represents an email message
type Message struct {
	To      []string
	Subject string
	HTML    string
	Text    string // optional, will be auto-generated from HTML if empty
}

// NewClient creates a new email client
func NewClient(host, port, username, password, from string) *Client {
	return &Client{
		Host:     host,
		Port:     port,
		Username: username,
		Password: password,
		From:     from,
	}
}

// Send sends an email message
func (c *Client) Send(msg *Message) error {
	if msg.Text == "" {
		text, err := htmlToText(msg.HTML)
		if err != nil {
			return fmt.Errorf("failed to convert HTML to text: %w", err)
		} else {
			msg.Text = text
		}
	}

	body, err := c.buildMultipartMessage(msg)
	if err != nil {
		return fmt.Errorf("failed to build message: %w", err)
	}

	auth := smtp.PlainAuth("", c.Username, c.Password, c.Host)
	addr := c.Host + ":" + c.Port

	return smtp.SendMail(addr, auth, c.From, msg.To, body)
}

// buildMultipartMessage creates a multipart email message
func (c *Client) buildMultipartMessage(msg *Message) ([]byte, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Headers
	buf.WriteString(fmt.Sprintf("From: %s\r\n", c.From))
	buf.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(msg.To, ", ")))
	buf.WriteString(fmt.Sprintf("Subject: %s\r\n", msg.Subject))
	buf.WriteString(fmt.Sprintf("Date: %s\r\n", time.Now().Format(time.RFC1123Z)))
	buf.WriteString("MIME-Version: 1.0\r\n")
	buf.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=%s\r\n", writer.Boundary()))
	buf.WriteString("\r\n")

	// Text part
	textHeader := make(textproto.MIMEHeader)
	textHeader.Set("Content-Type", "text/plain; charset=utf-8")
	textHeader.Set("Content-Transfer-Encoding", "quoted-printable")

	textPart, err := writer.CreatePart(textHeader)
	if err != nil {
		return nil, err
	}

	qpWriter := quotedprintable.NewWriter(textPart)
	if _, err := qpWriter.Write([]byte(msg.Text)); err != nil {
		return nil, err
	}
	qpWriter.Close()

	// HTML part
	htmlHeader := make(textproto.MIMEHeader)
	htmlHeader.Set("Content-Type", "text/html; charset=utf-8")
	htmlHeader.Set("Content-Transfer-Encoding", "quoted-printable")

	htmlPart, err := writer.CreatePart(htmlHeader)
	if err != nil {
		return nil, err
	}

	qpWriter = quotedprintable.NewWriter(htmlPart)
	if _, err := qpWriter.Write([]byte(msg.HTML)); err != nil {
		return nil, err
	}
	qpWriter.Close()

	writer.Close()

	return buf.Bytes(), nil
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
