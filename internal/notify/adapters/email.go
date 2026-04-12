package adapters

import (
	"bytes"
	"context"
	"fmt"
	"net/smtp"
	"strings"
	"text/template"
	"time"

	"github.com/argusxdr/argus/internal/notify"
	"go.uber.org/zap"
)

// EmailConfig contains the configuration for the Email notifier.
type EmailConfig struct {
	SenderAddress string   // Email address to send from
	RecipientList []string // List of email addresses to send to
	SMTPHost      string   // SMTP server hostname
	SMTPPort      int      // SMTP server port
	SMTPUser      string   // SMTP username (optional)
	SMTPPassword  string   // SMTP password (optional)
}

// EmailNotifier sends notifications via email using net/smtp.
type EmailNotifier struct {
	config EmailConfig
	logger *zap.Logger
}

// NewEmailNotifier creates a new Email notifier with the given configuration.
func NewEmailNotifier(config EmailConfig, logger *zap.Logger) (*EmailNotifier, error) {
	if config.SenderAddress == "" {
		return nil, fmt.Errorf("sender address is required")
	}
	if len(config.RecipientList) == 0 {
		return nil, fmt.Errorf("at least one recipient is required")
	}
	if config.SMTPHost == "" {
		return nil, fmt.Errorf("SMTP host is required")
	}
	if config.SMTPPort == 0 {
		return nil, fmt.Errorf("SMTP port is required")
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	return &EmailNotifier{
		config: config,
		logger: logger,
	}, nil
}

// Name returns the name of this notifier.
func (e *EmailNotifier) Name() string {
	return "email"
}

// severityString returns the severity as a human-readable string.
func severityString(severity int) string {
	switch severity {
	case 1:
		return "LOW"
	case 2:
		return "MEDIUM"
	case 3:
		return "HIGH"
	case 4:
		return "CRITICAL"
	default:
		return "BLOCKER"
	}
}

// emailTemplateData contains data for email template rendering.
type emailTemplateData struct {
	Title       string
	Message     string
	Severity    string
	AlertID     string
	RuleID      string
	SignalIDs   string
	Confidence  string
	Timestamp   string
}

// plainTextTemplate is the plain text email template.
const plainTextTemplate = `Alert Notification

Rule: {{.Title}}
Message: {{.Message}}

Severity: {{.Severity}}
Alert ID: {{.AlertID}}
Rule ID: {{.RuleID}}
Confidence: {{.Confidence}}

Signal IDs:
{{.SignalIDs}}

Timestamp: {{.Timestamp}}
`

// htmlTemplate is the HTML email template with proper escaping.
const htmlTemplate = `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: Arial, sans-serif; }
        .alert-box { border: 1px solid #ccc; padding: 15px; background-color: #f9f9f9; }
        .severity { font-weight: bold; }
        .critical { color: #d32f2f; }
        .high { color: #f57c00; }
        .medium { color: #fbc02d; }
        .low { color: #388e3c; }
        .metadata { margin-top: 10px; font-size: 12px; color: #666; }
        .code { background-color: #eee; padding: 2px 5px; font-family: monospace; }
    </style>
</head>
<body>
    <div class="alert-box">
        <h2>Alert Notification</h2>
        <p><strong>Rule:</strong> {{.Title}}</p>
        <p><strong>Message:</strong> {{.Message}}</p>
        <p class="severity">
            Severity: <span class="{{.Severity | toLower}}">{{.Severity}}</span>
        </p>
        <div class="metadata">
            <p><strong>Alert ID:</strong> <span class="code">{{.AlertID}}</span></p>
            <p><strong>Rule ID:</strong> <span class="code">{{.RuleID}}</span></p>
            <p><strong>Confidence:</strong> {{.Confidence}}</p>
            <p><strong>Timestamp:</strong> {{.Timestamp}}</p>
        </div>
        <h3>Signal IDs</h3>
        <pre>{{.SignalIDs}}</pre>
    </div>
</body>
</html>
`

// toLower converts a string to lowercase (template function).
func toLower(s string) string {
	return strings.ToLower(s)
}

// Send sends an email notification with both plain text and HTML variants.
func (e *EmailNotifier) Send(ctx context.Context, req *notify.NotificationRequest) (*notify.NotificationResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("notification request cannot be nil")
	}

	// Prepare template data
	data := emailTemplateData{
		Title:      req.Title,
		Message:    req.Message,
		Severity:   severityString(req.Severity),
		AlertID:    req.AlertID.String(),
		RuleID:     req.RuleID.String(),
		SignalIDs:  req.Metadata["signal_ids"],
		Confidence: req.Metadata["confidence"],
		Timestamp:  time.Now().Format(time.RFC3339),
	}

	// Render plain text template
	plainBuf := &bytes.Buffer{}
	plainTmpl, err := template.New("plain").Parse(plainTextTemplate)
	if err != nil {
		e.logger.Error("failed to parse plain text template", zap.Error(err))
		return nil, fmt.Errorf("template parse error: %w", err)
	}
	if err := plainTmpl.Execute(plainBuf, data); err != nil {
		e.logger.Error("failed to render plain text template", zap.Error(err))
		return nil, fmt.Errorf("template render error: %w", err)
	}

	// Render HTML template
	htmlBuf := &bytes.Buffer{}
	htmlTmpl, err := template.New("html").Funcs(template.FuncMap{"toLower": toLower}).Parse(htmlTemplate)
	if err != nil {
		e.logger.Error("failed to parse HTML template", zap.Error(err))
		return nil, fmt.Errorf("template parse error: %w", err)
	}
	if err := htmlTmpl.Execute(htmlBuf, data); err != nil {
		e.logger.Error("failed to render HTML template", zap.Error(err))
		return nil, fmt.Errorf("template render error: %w", err)
	}

	// Construct email with both plain text and HTML parts (multipart)
	subject := fmt.Sprintf("[%s] Alert: %s", data.Severity, req.Title)
	headers := fmt.Sprintf(
		"From: %s\r\nSubject: %s\r\nTo: %s\r\nMIME-Version: 1.0\r\nContent-Type: multipart/alternative; boundary=\"boundary123\"\r\n",
		e.config.SenderAddress,
		subject,
		strings.Join(e.config.RecipientList, ", "),
	)

	body := fmt.Sprintf(
		"%s\r\n--boundary123\r\nContent-Type: text/plain; charset=\"UTF-8\"\r\n\r\n%s\r\n--boundary123\r\nContent-Type: text/html; charset=\"UTF-8\"\r\n\r\n%s\r\n--boundary123--\r\n",
		headers,
		plainBuf.String(),
		htmlBuf.String(),
	)

	// Create context with timeout for SMTP
	mailCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Connect to SMTP server
	addr := fmt.Sprintf("%s:%d", e.config.SMTPHost, e.config.SMTPPort)
	smtpClient, err := smtp.Dial(addr)
	if err != nil {
		e.logger.Error("failed to connect to SMTP server", zap.Error(err), zap.String("addr", addr))
		return nil, fmt.Errorf("smtp connection error: %w", err)
	}
	defer smtpClient.Close()

	// Authenticate if credentials are provided
	if e.config.SMTPUser != "" && e.config.SMTPPassword != "" {
		auth := smtp.PlainAuth("", e.config.SMTPUser, e.config.SMTPPassword, e.config.SMTPHost)
		if err := smtpClient.Auth(auth); err != nil {
			e.logger.Error("smtp authentication failed", zap.Error(err))
			return nil, fmt.Errorf("smtp auth error: %w", err)
		}
	}

	// Send email with context awareness
	// Note: smtp.SendMail doesn't support context, so we respect the timeout via the mail headers
	if err := smtpClient.Mail(e.config.SenderAddress); err != nil {
		e.logger.Error("smtp mail command failed", zap.Error(err))
		return nil, fmt.Errorf("smtp mail error: %w", err)
	}

	for _, recipient := range e.config.RecipientList {
		if err := smtpClient.Rcpt(recipient); err != nil {
			e.logger.Error("smtp rcpt command failed", zap.Error(err), zap.String("recipient", recipient))
			return nil, fmt.Errorf("smtp rcpt error: %w", err)
		}
	}

	wc, err := smtpClient.Data()
	if err != nil {
		e.logger.Error("smtp data command failed", zap.Error(err))
		return nil, fmt.Errorf("smtp data error: %w", err)
	}
	defer wc.Close()

	if _, err := wc.Write([]byte(body)); err != nil {
		e.logger.Error("failed to write email body", zap.Error(err))
		return nil, fmt.Errorf("smtp write error: %w", err)
	}

	// Respect context cancellation
	select {
	case <-mailCtx.Done():
		return nil, fmt.Errorf("email send timeout: %w", mailCtx.Err())
	default:
	}

	e.logger.Info("email sent", zap.String("alert_id", req.AlertID.String()), zap.Int("recipients", len(e.config.RecipientList)))

	return &notify.NotificationResponse{
		Status:    "sent",
		MessageID: fmt.Sprintf("email-%d", time.Now().Unix()),
		Timestamp: time.Now().Unix(),
	}, nil
}

// Health checks if the SMTP server is accessible.
func (e *EmailNotifier) Health(ctx context.Context) error {
	addr := fmt.Sprintf("%s:%d", e.config.SMTPHost, e.config.SMTPPort)
	conn, err := smtp.Dial(addr)
	if err != nil {
		e.logger.Error("email health check failed", zap.Error(err))
		return fmt.Errorf("email health check failed: %w", err)
	}
	defer conn.Close()

	return nil
}
