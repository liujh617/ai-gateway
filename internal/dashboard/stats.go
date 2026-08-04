package dashboard

import (
	"context"
	"time"
)

// StatsCollector aggregates statistics from audit logs
type StatsCollector struct {
	logReader *LogReader
}

// SummaryStats represents overall gateway statistics
type SummaryStats struct {
	TotalRequests   int64            `json:"total_requests"`
	TodayRequests   int64            `json:"today_requests"`
	WeekRequests    int64            `json:"week_requests"`
	MonthRequests   int64            `json:"month_requests"`
	PIISummary      *PIISummary      `json:"pii_summary"`
	ContentSummary  *ContentSummary  `json:"content_summary"`
	AlertSummary    *AlertSummary    `json:"alert_summary"`
	TopProviders    []ProviderStats  `json:"top_providers"`
	TopModels       []ModelStats     `json:"top_models"`
}

// PIISummary represents PII detection summary
type PIISummary struct {
	TotalViolations int64            `json:"total_violations"`
	ByType          map[string]int64 `json:"by_type"`
	ByAction        map[string]int64 `json:"by_action"`
}

// ContentSummary represents content safety summary
type ContentSummary struct {
	TotalViolations int64            `json:"total_violations"`
	ByCategory      map[string]int64 `json:"by_category"`
	ByAction        map[string]int64 `json:"by_action"`
}

// AlertSummary represents alert summary
type AlertSummary struct {
	TotalAlerts int64            `json:"total_alerts"`
	ByChannel   map[string]int64 `json:"by_channel"`
	ByStatus    map[string]int64 `json:"by_status"`
}

// ProviderStats represents provider-level statistics
type ProviderStats struct {
	Name    string `json:"name"`
	Count   int64  `json:"count"`
	Latency int64  `json:"avg_latency_ms"`
}

// ModelStats represents model-level statistics
type ModelStats struct {
	Name    string `json:"name"`
	Count   int64  `json:"count"`
	Latency int64  `json:"avg_latency_ms"`
}

// PIIViolationFilter represents filters for PII violations
type PIIViolationFilter struct {
	StartTime *time.Time
	EndTime   *time.Time
	Type      string
	Action    string
	Page      int
	Size      int
}

// ContentSafetyViolationFilter represents filters for content safety violations
type ContentSafetyViolationFilter struct {
	StartTime *time.Time
	EndTime   *time.Time
	Category  string
	Action    string
	Page      int
	Size      int
}

// AlertFilter represents filters for alerts
type AlertFilter struct {
	StartTime *time.Time
	EndTime   *time.Time
	Source    string
	Channel   string
	Status    string
	Page      int
	Size      int
}

// NewStatsCollector creates a new StatsCollector instance
func NewStatsCollector(logReader *LogReader) *StatsCollector {
	return &StatsCollector{
		logReader: logReader,
	}
}

// GetSummary returns overall gateway statistics
func (sc *StatsCollector) GetSummary(ctx context.Context) (*SummaryStats, error) {
	// Get all logs for aggregation
	logs, _, err := sc.logReader.ListLogs(ctx, &AuditLogFilter{Page: 1, Size: 100000})
	if err != nil {
		return nil, err
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	week := today.AddDate(0, 0, -7)
	month := today.AddDate(0, -1, 0)

	stats := &SummaryStats{
		PIISummary: &PIISummary{
			ByType:   make(map[string]int64),
			ByAction: make(map[string]int64),
		},
		ContentSummary: &ContentSummary{
			ByCategory: make(map[string]int64),
			ByAction:   make(map[string]int64),
		},
		AlertSummary: &AlertSummary{
			ByChannel: make(map[string]int64),
			ByStatus:  make(map[string]int64),
		},
	}

	providerStats := make(map[string]*ProviderStats)
	modelStats := make(map[string]*ModelStats)

	for _, log := range logs {
		stats.TotalRequests++

		if log.Timestamp.After(today) {
			stats.TodayRequests++
		}
		if log.Timestamp.After(week) {
			stats.WeekRequests++
		}
		if log.Timestamp.After(month) {
			stats.MonthRequests++
		}

		if log.PIIDetected {
			stats.PIISummary.TotalViolations++
		}

		if log.ContentSafety {
			stats.ContentSummary.TotalViolations++
		}

		// Aggregate provider stats
		if log.Provider != "" {
			if _, exists := providerStats[log.Provider]; !exists {
				providerStats[log.Provider] = &ProviderStats{Name: log.Provider}
			}
			providerStats[log.Provider].Count++
			providerStats[log.Provider].Latency += log.LatencyMs
		}

		// Aggregate model stats
		if log.Model != "" {
			if _, exists := modelStats[log.Model]; !exists {
				modelStats[log.Model] = &ModelStats{Name: log.Model}
			}
			modelStats[log.Model].Count++
			modelStats[log.Model].Latency += log.LatencyMs
		}
	}

	// Calculate average latency
	for _, ps := range providerStats {
		if ps.Count > 0 {
			ps.Latency = ps.Latency / ps.Count
		}
		stats.TopProviders = append(stats.TopProviders, *ps)
	}

	for _, ms := range modelStats {
		if ms.Count > 0 {
			ms.Latency = ms.Latency / ms.Count
		}
		stats.TopModels = append(stats.TopModels, *ms)
	}

	return stats, nil
}

// GetPIIViolations returns paginated PII violations
func (sc *StatsCollector) GetPIIViolations(ctx context.Context, filter *PIIViolationFilter) ([]interface{}, int, error) {
	// TODO: Implement PII violation filtering and pagination
	return []interface{}{}, 0, nil
}

// GetPIIStats returns PII statistics
func (sc *StatsCollector) GetPIIStats(ctx context.Context) (*PIISummary, error) {
	// TODO: Implement detailed PII statistics
	return &PIISummary{
		ByType:   make(map[string]int64),
		ByAction: make(map[string]int64),
	}, nil
}

// GetContentSafetyViolations returns paginated content safety violations
func (sc *StatsCollector) GetContentSafetyViolations(ctx context.Context, filter *ContentSafetyViolationFilter) ([]interface{}, int, error) {
	// TODO: Implement content safety violation filtering and pagination
	return []interface{}{}, 0, nil
}

// GetContentSafetyStats returns content safety statistics
func (sc *StatsCollector) GetContentSafetyStats(ctx context.Context) (*ContentSummary, error) {
	// TODO: Implement detailed content safety statistics
	return &ContentSummary{
		ByCategory: make(map[string]int64),
		ByAction:   make(map[string]int64),
	}, nil
}

// GetAlerts returns paginated alerts
func (sc *StatsCollector) GetAlerts(ctx context.Context, filter *AlertFilter) ([]interface{}, int, error) {
	// TODO: Implement alert filtering and pagination
	return []interface{}{}, 0, nil
}

// GetAlertStats returns alert statistics
func (sc *StatsCollector) GetAlertStats(ctx context.Context) (*AlertSummary, error) {
	// TODO: Implement detailed alert statistics
	return &AlertSummary{
		ByChannel: make(map[string]int64),
		ByStatus:  make(map[string]int64),
	}, nil
}