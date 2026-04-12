package adapters

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/argusxdr/argus/internal/notify"
	"github.com/slack-go/slack"
	"go.uber.org/zap"
)

// SlackConfig contains the configuration for the Slack notifier.
type SlackConfig struct {
	APIToken  string // Bot token for Slack API
	ChannelID string // Channel ID to post to
}

// SlackNotifier sends notifications to Slack using the slack-go/slack SDK.
type SlackNotifier struct {
	config SlackConfig
	client *slack.Client
	logger *zap.Logger
}

// NewSlackNotifier creates a new Slack notifier with the given configuration.
func NewSlackNotifier(config SlackConfig, logger *zap.Logger) (*SlackNotifier, error) {
	if config.APIToken == "" {
		return nil, fmt.Errorf("slack API token is required")
	}
	if config.ChannelID == "" {
		return nil, fmt.Errorf("slack channel ID is required")
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	return &SlackNotifier{
		config: config,
		client: slack.New(config.APIToken),
		logger: logger,
	}, nil
}

// Name returns the name of this notifier.
func (s *SlackNotifier) Name() string {
	return "slack"
}

// severityEmoji returns the emoji badge for the given severity level.
// 1 = Low (🟢), 2 = Medium (🟡), 3 = High (🟠), 4+ = Critical (🔴) or higher
func severityEmoji(severity int) string {
	switch severity {
	case 1:
		return "🟢"
	case 2:
		return "🟡"
	case 3:
		return "🟠"
	case 4:
		return "🔴"
	default: // 5+
		return "🔴🔴"
	}
}

// formatSignalIDs formats signal IDs for display, showing up to 5 and indicating if there are more.
func formatSignalIDs(signalIDs []string) string {
	if len(signalIDs) == 0 {
		return "No signals"
	}

	if len(signalIDs) <= 5 {
		return strings.Join(signalIDs, "\n")
	}

	// Show first 5 and indicate how many more
	displayed := strings.Join(signalIDs[:5], "\n")
	more := len(signalIDs) - 5
	return fmt.Sprintf("%s\nand %d more", displayed, more)
}

// Send sends a notification to Slack with structured message blocks.
func (s *SlackNotifier) Send(ctx context.Context, req *notify.NotificationRequest) (*notify.NotificationResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("notification request cannot be nil")
	}

	// Create the message blocks
	headerBlock := slack.NewHeaderBlock(
		slack.NewTextBlockObject(
			slack.PlainTextType,
			fmt.Sprintf("%s %s: %s", severityEmoji(req.Severity), req.Title, req.Message),
			false,
			false,
		),
	)

	// Context section with metadata
	contextElements := []slack.MixedElement{
		slack.NewTextBlockObject(slack.MarkdownType, fmt.Sprintf("*Alert ID:* `%s`", req.AlertID.String()), false, false),
		slack.NewTextBlockObject(slack.MarkdownType, fmt.Sprintf("*Rule ID:* `%s`", req.RuleID.String()), false, false),
	}

	// Add confidence if available
	if confidence, ok := req.Metadata["confidence"]; ok {
		contextElements = append(contextElements, slack.NewTextBlockObject(slack.MarkdownType, fmt.Sprintf("*Confidence:* %s", confidence), false, false))
	}

	contextBlock := slack.NewContextBlock("", contextElements...)

	// Signal IDs section
	signalIDs := strings.Split(req.Metadata["signal_ids"], ",")
	signalBlock := slack.NewSectionBlock(
		slack.NewTextBlockObject(slack.MarkdownType, fmt.Sprintf("*Signal IDs:*\n%s", formatSignalIDs(signalIDs)), false, false),
		nil,
		nil,
	)

	// Action buttons
	actionElements := []slack.BlockElement{
		slack.NewButtonBlockElement("action_acknowledge", "acknowledge", slack.NewTextBlockObject(slack.PlainTextType, "Acknowledge", false, false)),
		slack.NewButtonBlockElement("action_escalate", "escalate", slack.NewTextBlockObject(slack.PlainTextType, "Escalate", false, false)),
	}
	actionBlock := slack.NewActionBlock("", actionElements...)

	// Send message with context timeout
	msgOpts := []slack.MsgOption{
		slack.MsgOptionBlocks(
			headerBlock,
			contextBlock,
			signalBlock,
			actionBlock,
		),
	}

	channelID, timestamp, err := s.client.PostMessageContext(ctx, s.config.ChannelID, msgOpts...)
	if err != nil {
		s.logger.Error("failed to send slack message", zap.Error(err), zap.String("alert_id", req.AlertID.String()))

		// Check if it's a rate limit error (recoverable)
		if strings.Contains(err.Error(), "rate_limited") {
			return &notify.NotificationResponse{
				Status:    "retrying",
				Error:     err.Error(),
				Timestamp: time.Now().Unix(),
			}, fmt.Errorf("slack rate limited: %w", err)
		}

		return &notify.NotificationResponse{
			Status:    "failed",
			Error:     err.Error(),
			Timestamp: time.Now().Unix(),
		}, err
	}

	s.logger.Info("slack message sent", zap.String("alert_id", req.AlertID.String()), zap.String("timestamp", timestamp))

	return &notify.NotificationResponse{
		Status:    "sent",
		MessageID: fmt.Sprintf("%s:%s", channelID, timestamp),
		Timestamp: time.Now().Unix(),
	}, nil
}

// Health checks if the Slack API is accessible.
func (s *SlackNotifier) Health(ctx context.Context) error {
	// Test basic API connectivity by calling auth.test
	_, err := s.client.AuthTestContext(ctx)
	if err != nil {
		s.logger.Error("slack health check failed", zap.Error(err))
		return fmt.Errorf("slack health check failed: %w", err)
	}
	return nil
}
