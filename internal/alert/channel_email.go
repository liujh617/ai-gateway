package alert

import (
	"bytes"
	"context"
	"fmt"
	"net/smtp"
	"strings"
	"text/template"
	"time"
)

// EmailChannel sends alerts via SMTP email
type EmailChannel struct {
	name           string
	smtpHost       string
	smtpPort       int
	smtpUser       string
	smtpPassword   string
	from           string
	to             []string
	subjectTemplate string
}

// EmailConfig configures email channel
type EmailConfig struct {
	SMTPHost       string   `json:"smtp_host"`
	SMTPPort       int      `json:"smtp_port"`
	SMTPUser       string   `json:"smtp_user"`
	SMTPPassword   string   `json:"smtp_password"`
	From           string   `json:"from"`
	To             []string `json:"to"`
	SubjectTemplate string  `json:"subject_template"`
}

// NewEmailChannel creates a new email channel
func NewEmailChannel(name string, config EmailConfig) (*EmailChannel, error) {
	if config.SMTPHost == "" {
		return nil, fmt.Errorf("SMTP host is required")
	}
	if config.SMTPPort == 0 {
		config.SMTPPort = 587
	}
	if len(config.To) == 0 {
		return nil, fmt.Errorf("email recipients are required")
	}

	subjectTemplate := config.SubjectTemplate
	if subjectTemplate == "" {
		subjectTemplate = "[{{.Severity}}] {{.Title}}"
	}

	return &EmailChannel{
		name:           name,
		smtpHost:       config.SMTPHost,
		smtpPort:       config.SMTPPort,
		smtpUser:       config.SMTPUser,
		smtpPassword:   config.SMTPPassword,
		from:           config.From,
		to:             config.To,
		subjectTemplate: subjectTemplate,
	}, nil
}

// Name returns the channel name
func (c *EmailChannel) Name() string {
	return c.name
}

// Send sends an alert via email
func (c *EmailChannel) Send(ctx context.Context, alert *Alert) error {
	// Generate email subject
	tmpl, err := template.New("subject").Parse(c.subjectTemplate)
	if err != nil {
		return fmt.Errorf("parse subject template: %w", err)
	}

	var subjectBuf bytes.Buffer
	if err := tmpl.Execute(&subjectBuf, alert); err != nil {
		return fmt.Errorf("execute subject template: %w", err)
	}
	subject := subjectBuf.String()

	// Generate email body
	body := c.generateEmailBody(alert)

	// Construct email message
	message := fmt.Sprintf("From: %s\r\n", c.from)
	message += fmt.Sprintf("To: %s\r\n", strings.Join(c.to, ","))
	message += fmt.Sprintf("Subject: %s\r\n", subject)
	message += "MIME-version: 1.0;\r\nContent-Type: text/plain; charset=\"UTF-8\";\r\n"
	message += "\r\n" + body

	// Send email
	auth := smtp.PlainAuth("", c.smtpUser, c.smtpPassword, c.smtpHost)
	addr := fmt.Sprintf("%s:%d", c.smtpHost, c.smtpPort)

	if err := smtp.SendMail(addr, auth, c.from, c.to, []byte(message)); err != nil {
		return fmt.Errorf("send email: %w", err)
	}

	return nil
}

// generateEmailBody generates the email body
func (c *EmailChannel) generateEmailBody(alert *Alert) string {
	var buf bytes.Buffer

	buf.WriteString(fmt.Sprintf("Alert ID: %s\r\n", alert.ID))
	buf.WriteString(fmt.Sprintf("Timestamp: %s UTC\r\n", alert.Timestamp.Format("2006-01-02 15:04:05")))
	buf.WriteString(fmt.Sprintf("Source: %s\r\n", alert.Source))
	buf.WriteString(fmt.Sprintf("Severity: %s\r\n\r\n", strings.ToUpper(alert.Severity)))

	buf.WriteString(fmt.Sprintf("Title: %s\r\n", alert.Title))
	buf.WriteString(fmt.Sprintf("Description: %s\r\n\r\n", alert.Description))

	if len(alert.Details) > 0 {
		buf.WriteString("Details:\r\n")
		for key, value := range alert.Details {
			buf.WriteString(fmt.Sprintf("- %s: %v\r\n", key, value))
		}
		buf.WriteString("\r\n")
	}

	buf.WriteString("Request Context:\r\n")
	if alert.RequestID != "" {
		buf.WriteString(fmt.Sprintf("- Request ID: %s\r\n", alert.RequestID))
	}
	if alert.TraceID != "" {
		buf.WriteString(fmt.Sprintf("- Trace ID: %s\r\n", alert.TraceID))
	}
	if alert.Client != "" {
		buf.WriteString(fmt.Sprintf("- Client: %s\r\n", alert.Client))
	}
	if alert.Provider != "" {
		buf.WriteString(fmt.Sprintf("- Provider: %s\r\n", alert.Provider))
	}
	if alert.Model != "" {
		buf.WriteString(fmt.Sprintf("- Model: %s\r\n", alert.Model))
	}
	buf.WriteString("\r\n")

	buf.WriteString("---\r\n")
	buf.WriteString("This is an automated alert from AI Gateway.\r\n")

	return buf.String()
}

// Close closes the email channel
func (c *EmailChannel) Close() error {
	return nil
}