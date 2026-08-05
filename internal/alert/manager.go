package alert

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

// AlertManagerImpl implements AlertManager interface
type AlertManagerImpl struct {
	channels  map[string]Channel
	rules     []AlertRule
	rateLimit *rateLimiter
	retry     RetryConfig
	storage   *alertStorage
	logger    *slog.Logger

	// Cooldown tracking
	cooldowns  map[string]time.Time
	cooldownMu sync.RWMutex

	// Alert queue
	queue     chan *Alert
	queueSize int

	// Shutdown
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewAlertManager creates a new alert manager
func NewAlertManager(opts AlertManagerOptions, logger *slog.Logger) (*AlertManagerImpl, error) {
	if logger == nil {
		logger = slog.Default()
	}

	// Validate options
	if len(opts.Channels) == 0 {
		return nil, fmt.Errorf("at least one channel is required")
	}
	if len(opts.Rules) == 0 {
		return nil, fmt.Errorf("at least one rule is required")
	}

	ctx, cancel := context.WithCancel(context.Background())

	am := &AlertManagerImpl{
		channels:  opts.Channels,
		rules:     opts.Rules,
		retry:     opts.Retry,
		logger:    logger,
		cooldowns: make(map[string]time.Time),
		queue:     make(chan *Alert, 100),
		queueSize: 100,
		ctx:       ctx,
		cancel:    cancel,
	}

	// Initialize rate limiter
	if opts.RateLimit.Enabled {
		am.rateLimit = newRateLimiter(opts.RateLimit.MaxAlertsPerMinute, opts.RateLimit.BurstSize)
	}

	// Initialize storage
	if opts.Storage.Enabled {
		storage, err := newAlertStorage(opts.Storage.Path, opts.Storage.MaxFileBytes)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("initialize alert storage: %w", err)
		}
		am.storage = storage
	}

	// Start worker
	am.wg.Add(1)
	go am.worker()

	return am, nil
}

// CreateAlert creates and queues a new alert
func (am *AlertManagerImpl) CreateAlert(ctx context.Context, alert *Alert) error {
	// Generate ID if not provided
	if alert.ID == "" {
		alert.ID = fmt.Sprintf("alert-%s", uuid.New().String()[:8])
	}

	// Set timestamp if not provided
	if alert.Timestamp.IsZero() {
		alert.Timestamp = time.Now()
	}

	// Set default status
	if alert.Status == "" {
		alert.Status = StatusPending
	}

	// Find matching rules
	rules := am.findMatchingRules(alert)
	if len(rules) == 0 {
		am.logger.Debug("no matching rules for alert",
			"alert_id", alert.ID,
			"source", alert.Source,
		)
		return nil
	}

	// Set channels from rules
	channelSet := make(map[string]bool)
	for _, rule := range rules {
		for _, ch := range rule.Channels {
			channelSet[ch] = true
		}
	}
	alert.Channels = make([]string, 0, len(channelSet))
	for ch := range channelSet {
		alert.Channels = append(alert.Channels, ch)
	}

	// Check cooldown
	if am.isInCooldown(alert) {
		am.logger.Debug("alert suppressed by cooldown",
			"alert_id", alert.ID,
			"source", alert.Source,
		)
		RecordAlertSuppressed("cooldown")
		return nil
	}

	// Check rate limit
	if am.rateLimit != nil && !am.rateLimit.Allow() {
		am.logger.Warn("alert rate limit exceeded",
			"alert_id", alert.ID,
		)
		RecordAlertSuppressed("rate_limit")
		return nil
	}

	// Record alert created
	RecordAlertCreated(alert.Source, string(alert.Severity))

	// Queue alert
	select {
	case am.queue <- alert:
		am.logger.Debug("alert queued",
			"alert_id", alert.ID,
			"channels", alert.Channels,
		)
		return nil
	default:
		return fmt.Errorf("alert queue is full")
	}
}

// Close shuts down the alert manager
func (am *AlertManagerImpl) Close() error {
	am.cancel()
	am.wg.Wait()

	// Close channels
	for _, ch := range am.channels {
		ch.Close()
	}

	// Close storage
	if am.storage != nil {
		am.storage.Close()
	}

	return nil
}

// findMatchingRules finds rules matching the alert
func (am *AlertManagerImpl) findMatchingRules(alert *Alert) []AlertRule {
	var matching []AlertRule
	for _, rule := range am.rules {
		if !rule.Enabled {
			continue
		}
		if rule.Source != alert.Source {
			continue
		}
		matching = append(matching, rule)
	}
	return matching
}

// isInCooldown checks if alert is in cooldown period
func (am *AlertManagerImpl) isInCooldown(alert *Alert) bool {
	am.cooldownMu.RLock()
	defer am.cooldownMu.RUnlock()

	key := fmt.Sprintf("%s:%s", alert.Source, alert.Client)
	if lastTime, exists := am.cooldowns[key]; exists {
		// Find matching rule to get cooldown duration
		for _, rule := range am.rules {
			if rule.Source == alert.Source && rule.CooldownMinutes > 0 {
				if time.Since(lastTime) < time.Duration(rule.CooldownMinutes)*time.Minute {
					return true
				}
			}
		}
	}
	return false
}

// setCooldown sets cooldown for alert
func (am *AlertManagerImpl) setCooldown(alert *Alert) {
	am.cooldownMu.Lock()
	defer am.cooldownMu.Unlock()

	key := fmt.Sprintf("%s:%s", alert.Source, alert.Client)
	am.cooldowns[key] = time.Now()
}

// worker processes alerts from queue
func (am *AlertManagerImpl) worker() {
	defer am.wg.Done()

	for {
		select {
		case <-am.ctx.Done():
			return
		case alert := <-am.queue:
			am.processAlert(alert)
		}
	}
}

// processAlert processes a single alert
func (am *AlertManagerImpl) processAlert(alert *Alert) {
	// Send to each channel
	var lastErr error
	for _, chName := range alert.Channels {
		ch, exists := am.channels[chName]
		if !exists {
			am.logger.Error("channel not found",
				"alert_id", alert.ID,
				"channel", chName,
			)
			continue
		}

		// Record delivery start time
		deliveryStart := time.Now()

		// Retry logic
		var err error
		for i := 0; i <= am.retry.MaxRetries; i++ {
			err = ch.Send(am.ctx, alert)
			if err == nil {
				break
			}

			if i < am.retry.MaxRetries && am.retry.Enabled {
				am.logger.Warn("alert send failed, retrying",
					"alert_id", alert.ID,
					"channel", chName,
					"attempt", i+1,
					"error", err,
				)
				time.Sleep(time.Duration(am.retry.RetryDelaySeconds) * time.Second)
			}
		}

		// Record delivery duration
		RecordAlertDeliveryDuration(chName, time.Since(deliveryStart).Seconds())

		if err != nil {
			lastErr = err
			RecordAlertFailed(chName, string(alert.Severity), "send_error")
			am.logger.Error("alert send failed",
				"alert_id", alert.ID,
				"channel", chName,
				"error", err,
			)
		} else {
			RecordAlertSent(chName, string(alert.Severity))
			am.logger.Info("alert sent",
				"alert_id", alert.ID,
				"channel", chName,
			)
		}
	}

	// Update status
	if lastErr != nil {
		alert.Status = StatusFailed
		alert.Error = lastErr.Error()
	} else {
		alert.Status = StatusSent
		now := time.Now()
		alert.SentAt = &now
		am.setCooldown(alert)
	}

	// Store alert
	if am.storage != nil {
		if err := am.storage.Store(alert); err != nil {
			am.logger.Error("failed to store alert",
				"alert_id", alert.ID,
				"error", err,
			)
		}
	}
}