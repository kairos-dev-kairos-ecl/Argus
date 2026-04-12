package adapters

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/argusxdr/argus/internal/notify"
	"github.com/RackSec/srslog"
	"go.uber.org/zap"
)

// SyslogConfig contains configuration for the Syslog notifier.
type SyslogConfig struct {
	Server    string // hostname:port (e.g., "siem.example.com:514")
	Transport string // udp, tcp, or tls (default: udp)
	Facility  srslog.Priority // syslog facility (e.g., srslog.LOG_LOCAL7)
}

// SyslogNotifier sends notifications to a syslog server in CEF format over RFC5424.
type SyslogNotifier struct {
	config    SyslogConfig
	logger    *zap.Logger
	writer    *srslog.Writer
	protocol  string
}

// NewSyslogNotifier creates a new Syslog notifier with the given configuration.
func NewSyslogNotifier(config SyslogConfig, logger *zap.Logger) (*SyslogNotifier, error) {
	if config.Server == "" {
		return nil  , fmt.Errorf("syslog server address is required")
	}

	// Determine transport protocol
	transport := config.Transport
	if transport == "" {
		transport = "udp"
	}
	if transport != "udp" && transport != "tcp" && transport != "tls" {
		return nil, fmt.Errorf("invalid transport: %s (must be udp, tcp, or tls)", transport)
	}

	// Validate facility (convert to syslog priority)
	facility := config.Facility
	if facility == 0 {
		facility = srslog.LOG_LOCAL7
	}

	// Create srslog writer using network connection
	network := transport
	if transport == "tls" {
		network = "tcp"
	}

	writer, err := srslog.Dial(network, config.Server, facility|srslog.LOG_NOTICE, "argus-xdr")
	if err != nil {
		return nil, fmt.Errorf("failed to connect to syslog server: %w", err)
	}

	sn := &SyslogNotifier{
		config:   config,
		logger:   logger,
		writer:   writer,
		protocol: transport,
	}

	return sn, nil
}

// Name returns the notifier name.
func (s *SyslogNotifier) Name() string {
	return "syslog"
}

// Send sends a notification to the syslog server in CEF format.
func (s *SyslogNotifier) Send(ctx context.Context, req *notify.NotificationRequest) (*notify.NotificationResponse, error) {
	if s.writer == nil {
		return nil, fmt.Errorf("syslog writer not initialized")
	}

	// Extract rule_id and rule_name from metadata (passed from Alert via dispatcher)
	ruleID := req.Metadata["rule_id"]
	ruleName := req.Metadata["rule_name"]
	signalCount := req.Metadata["signal_count"]
	confidence := req.Metadata["confidence"]
	fingerprint := req.Metadata["fingerprint"]
	dedupCount := req.Metadata["dedup_count"]

	// Map severity to CEF severity level (0-high, 3-medium, 6-low, 10-informational)
	cefSeverity := cefSeverityFromInt(req.Severity)

	// Build CEF message in RFC5424 format
	// RFC5424: <PRI>TIMESTAMP HOSTNAME TAG[PID]: MSG
	// CEF:0|Vendor|Product|Version|SignatureID|Name|Severity|Extensions
	cefExtensions := fmt.Sprintf("signal_count=%s confidence=%s fingerprint=%s dedup_count=%s alert_id=%s",
		signalCount, confidence, fingerprint, dedupCount, req.AlertID.String())

	cefMessage := fmt.Sprintf("CEF:0|Argus|XDR|1.0|%s|%s|%s|%s",
		ruleID, ruleName, cefSeverity, cefExtensions)

	// Send via srslog (includes RFC5424 framing automatically)
	// srslog.Writer.Write accepts []byte and returns (int, error)
	_, err := s.writer.Write([]byte(cefMessage))
	if err != nil {
		s.logger.Error("failed to send syslog message", zap.Error(err))
		return nil, fmt.Errorf("syslog send failed: %w", err)
	}

	s.logger.Debug("alert sent to syslog", zap.String("rule_id", ruleID), zap.String("rule_name", ruleName))

	return &notify.NotificationResponse{
		Status:    "sent",
		MessageID: fingerprint,
		Timestamp: time.Now().Unix(),
	}, nil
}

// Health checks if the notifier is operational by attempting a connection.
func (s *SyslogNotifier) Health(ctx context.Context) error {
	if s.writer == nil {
		return fmt.Errorf("syslog writer not initialized")
	}

	// Try to establish a test connection to syslog server
	network := s.protocol
	if s.protocol == "tls" {
		network = "tcp"
	}

	conn, err := net.DialTimeout(network, s.config.Server, 5*time.Second)
	if err != nil {
		return fmt.Errorf("syslog server unreachable: %w", err)
	}
	defer conn.Close()

	return nil
}

// cefSeverityFromInt converts an Argus severity (1-5) to CEF severity level.
// CEF severity: 0=High, 3=Medium, 6=Low, 10=Informational
func cefSeverityFromInt(severity int) string {
	switch severity {
	case 5: // CRITICAL
		return "0" // CEF High (0)
	case 4: // HIGH
		return "2" // CEF High (slightly lower)
	case 3: // MEDIUM
		return "5" // CEF Medium
	case 2: // LOW
		return "7" // CEF Low
	case 1: // INFO
		return "10" // CEF Informational
	default:
		return "5" // Default to Medium
	}
}

// Close closes the syslog connection.
func (s *SyslogNotifier) Close() error {
	if s.writer != nil {
		return s.writer.Close()
	}
	return nil
}
