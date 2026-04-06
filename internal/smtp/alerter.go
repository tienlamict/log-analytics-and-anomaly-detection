package smtp

import (
	"context"
	"fmt"
	"strings"
	"time"

	mail "github.com/wneessen/go-mail"
	"go.uber.org/zap"

	"github.com/log-analytics/server/internal/domain"
	"github.com/log-analytics/server/internal/metrics"
)

// SMTPConfig holds SMTP connection and addressing parameters.
type SMTPConfig struct {
	Host       string
	Port       int
	Username   string
	Password   string
	From       string
	Recipients []string
	TLSPolicy  string // "mandatory", "opportunistic", "none"
}

// SMTPAlerter implements domain.AlertChannel via async buffered dispatch.
type SMTPAlerter struct {
	client *mail.Client
	cfg    SMTPConfig
	logger *zap.Logger
	queue  chan domain.Anomaly
}

// NewSMTPAlerter constructs an SMTPAlerter with a configured go-mail client.
// When cfg.Username is empty, SMTP auth options are omitted (suitable for local MailHog).
func NewSMTPAlerter(cfg SMTPConfig, logger *zap.Logger) (*SMTPAlerter, error) {
	tlsPolicy, err := parseTLSPolicy(cfg.TLSPolicy)
	if err != nil {
		return nil, fmt.Errorf("smtp alerter: %w", err)
	}

	opts := []mail.Option{
		mail.WithPort(cfg.Port),
		mail.WithTLSPolicy(tlsPolicy),
		mail.WithTimeout(30 * time.Second),
	}

	if cfg.Username != "" {
		opts = append(opts,
			mail.WithSMTPAuth(mail.SMTPAuthPlain),
			mail.WithUsername(cfg.Username),
			mail.WithPassword(cfg.Password),
		)
	}

	client, err := mail.NewClient(cfg.Host, opts...)
	if err != nil {
		return nil, fmt.Errorf("smtp alerter: create client: %w", err)
	}

	return &SMTPAlerter{
		client: client,
		cfg:    cfg,
		logger: logger,
		queue:  make(chan domain.Anomaly, 500),
	}, nil
}

// Name returns the alerter channel identifier.
func (a *SMTPAlerter) Name() string {
	return "smtp"
}

// Send enqueues the anomaly for async dispatch and returns immediately.
// If the internal buffer is full the anomaly is dropped (non-fatal) and the
// failure counter is incremented.
func (a *SMTPAlerter) Send(_ context.Context, anomaly domain.Anomaly) error {
	select {
	case a.queue <- anomaly:
		return nil
	default:
		a.logger.Warn("smtp alert queue full, dropping anomaly",
			zap.String("rule_id", anomaly.RuleID))
		metrics.EmailAlertsFailedTotal.Inc()
		return nil
	}
}

// Run drains the internal queue and dispatches each anomaly via SMTP.
// It blocks until ctx is cancelled, then drains remaining items before returning.
func (a *SMTPAlerter) Run(ctx context.Context) {
	for {
		select {
		case anomaly := <-a.queue:
			a.dispatch(ctx, anomaly)
		case <-ctx.Done():
			// Drain remaining items from queue.
			for {
				select {
				case anomaly := <-a.queue:
					a.dispatch(ctx, anomaly)
				default:
					return
				}
			}
		}
	}
}

// dispatch builds and sends a single alert email. Errors are logged and metered
// but never propagated — SMTP failures must not block the pipeline.
func (a *SMTPAlerter) dispatch(ctx context.Context, anomaly domain.Anomaly) {
	m := mail.NewMsg()

	if err := m.From(a.cfg.From); err != nil {
		a.logger.Error("smtp alert: set from address", zap.Error(err))
		metrics.EmailAlertsFailedTotal.Inc()
		return
	}

	if err := m.To(a.cfg.Recipients...); err != nil {
		a.logger.Error("smtp alert: set recipients", zap.Error(err))
		metrics.EmailAlertsFailedTotal.Inc()
		return
	}

	m.Subject(fmt.Sprintf("[%s] %s - %s",
		strings.ToUpper(anomaly.Severity),
		anomaly.RuleID,
		anomaly.Service,
	))

	m.SetBodyString(mail.TypeTextPlain, formatAlertBody(anomaly))

	if err := a.client.DialAndSendWithContext(ctx, m); err != nil {
		a.logger.Error("smtp send failed",
			zap.Error(err),
			zap.String("rule_id", anomaly.RuleID),
			zap.String("service", anomaly.Service),
		)
		metrics.EmailAlertsFailedTotal.Inc()
		return
	}

	metrics.EmailAlertsSentTotal.Inc()
}

// formatAlertBody produces the plain-text email body for an anomaly.
// Evidence is capped at 5 sample log lines.
func formatAlertBody(a domain.Anomaly) string {
	var b strings.Builder

	b.WriteString("Anomaly Detected\n\n")
	fmt.Fprintf(&b, "Type:         %s\n", a.RuleID)
	fmt.Fprintf(&b, "Service:      %s\n", a.Service)
	fmt.Fprintf(&b, "Severity:     %s\n", a.Severity)
	fmt.Fprintf(&b, "Detected At:  %s\n", a.DetectedAt.UTC().Format(time.RFC3339))
	fmt.Fprintf(&b, "Description:  %s\n", a.Description)

	if len(a.Evidence) > 0 {
		b.WriteString("\nSample Log Lines:\n")
		for i, entry := range a.Evidence {
			if i >= 5 {
				break
			}
			fmt.Fprintf(&b, "  [%s] %s: %s\n",
				entry.Timestamp.UTC().Format(time.RFC3339),
				entry.Level,
				entry.Message,
			)
		}
	}

	return b.String()
}

// parseTLSPolicy maps a config string to a go-mail TLSPolicy constant.
func parseTLSPolicy(s string) (mail.TLSPolicy, error) {
	switch s {
	case "mandatory":
		return mail.TLSMandatory, nil
	case "opportunistic":
		return mail.TLSOpportunistic, nil
	case "none", "":
		return mail.NoTLS, nil
	default:
		return 0, fmt.Errorf("unknown TLS policy: %q", s)
	}
}
