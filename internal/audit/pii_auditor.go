package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

// PIIAuditor PII审计器，负责检测和记录PII数据
type PIIAuditor struct {
	detector Detector
	action   PIIDetectionAction
	logger   *slog.Logger
}

// NewPIIAuditor 创建PII审计器
func NewPIIAuditor(detector Detector, action PIIDetectionAction, logger *slog.Logger) *PIAuditor {
	if logger == nil {
		logger = slog.Default()
	}
	return &PIIAssessor{
		detector: detector,
		action:   action,
		logger:   logger,
	}
}

// AuditRequest 审计请求中的PII数据
func (a *PIIAssessor) AuditRequest(ctx context.Context, event *Event) error {
	if a.detector == nil {
		return nil
	}

	// 检测Body中的PII
	bodyText := string(event.Body)
	result, err := a.detector.Detect(ctx, bodyText)
	if err != nil {
		return fmt.Errorf("PII detection failed: %w", err)
	}

	if !result.HasPII {
		return nil
	}

	// 记录PII检测事件
	a.logPIIDetection(event, result)

	// 根据action处理
	switch a.action {
	case PIIActionAlert:
		// 只记录告警，不阻止
		return nil
	case PIIActionReject:
		// 拒绝请求
		return &PIIDetectionError{
			Message: fmt.Sprintf("PII detected in request: %d instances found", result.TotalCount),
		}
	case PIIActionAllow:
		// 允许通过
		return nil
	default:
		return nil
	}
}

// logPIIDetection 记录PII检测日志
func (a *PIIAssessor) logPIIDetection(event *Event, result *PIIDetectionResult) {
	a.logger.Warn("PII data detected in audit log",
		"request_id", event.RequestID,
		"client", event.Client,
		"pii_count", result.TotalCount,
		"pii_types", result.TypesFound,
		"event_type", event.Event,
		"timestamp", event.Timestamp,
	)
}

// PIIDetectionError PII检测错误
type PIIDetectionError struct {
	Message string
}

func (e *PIIDetectionError) Error() string {
	return e.Message
}

// AuditEventWithPII 包含PII信息的审计事件
type AuditEventWithPII struct {
	Event
	PIIResult *PIIDetectionResult `json:"pii_result,omitempty"`
}

// ToJSON 转换为JSON格式
func (e *AuditEventWithPII) ToJSON() (json.RawMessage, error) {
	return json.Marshal(e)
}

// PIIAuditorRecorder PII审计记录器包装器
type PIIAuditorRecorder struct {
	recorder Recorder
	auditor  *PIIAssessor
}

// NewPIIAssessorRecorder 创建PII审计记录器
func NewPIIAssessorRecorder(recorder Recorder, auditor *PIIAssessor) *PIIAuditorRecorder {
	return &PIIAssessorRecorder{
		recorder: recorder,
		auditor:  auditor,
	}
}

// Record 实现Recorder接口，添加PII检测
func (r *PIIAssessorRecorder) Record(ctx context.Context, event Event) {
	// 创建事件副本
	eventCopy := event

	// 执行PII检测
	if r.auditor != nil && len(eventCopy.Body) > 0 {
		result, err := r.auditor.detector.Detect(ctx, string(eventCopy.Body))
		if err != nil {
			r.auditor.logger.Error("PII detection failed", "error", err)
		} else if result.HasPII {
			// 记录PII检测日志
			r.auditor.logPIIDetection(&eventCopy, result)

			// 根据action处理
			if r.auditor.action == PIIActionReject {
				// 不记录包含PII的事件
				return
			}
		}
	}

	// 调用底层记录器
	r.recorder.Record(ctx, eventCopy)
}

// PIIAwareJSONLRecorder 支持PII检测的JSONL记录器
type PIIAwareJSONLRecorder struct {
	*JSONLRecorder
	detector Detector
	action   PIIDetectionAction
	logger   *slog.Logger
}

// NewPIIAwareJSONLRecorder 创建支持PII检测的JSONL记录器
func NewPIIAwareJSONLRecorder(path string, options JSONLRecorderOptions, detector Detector, action PIIDetectionAction) (*PIIAssessorJSONLRecorder, error) {
	recorder, err := NewJSONLRecorderWithOptions(path, options)
	if err != nil {
		return nil, err
	}

	return &PIIAssessorJSONLRecorder{
		JSONLRecorder: recorder,
		detector:      detector,
		action:        action,
		logger:        slog.Default(),
	}, nil
}

// Record 实现Recorder接口，添加PII检测
func (r *PIIAssessorJSONLRecorder) Record(ctx context.Context, event Event) {
	// 检查context是否已取消
	if ctx.Err() != nil {
		return
	}

	// 执行PII检测
	if r.detector != nil && len(event.Body) > 0 {
		result, err := r.detector.Detect(ctx, string(event.Body))
		if err != nil {
			r.logger.Error("PII detection failed", "error", err, "request_id", event.RequestID)
		} else if result.HasPII {
			// 记录PII检测日志
			r.logger.Warn("PII detected in audit log",
				"request_id", event.RequestID,
				"client", event.Client,
				"pii_count", result.TotalCount,
				"pii_types", result.TypesFound,
			)

			// 根据action处理
			if r.action == PIIActionReject {
				// 不记录包含PII的事件
				return
			}
		}
	}

	// 调用父类的Record方法
	r.JSONLRecorder.Record(ctx, event)
}

// PIISummary PII检测摘要
type PIISummary struct {
	TotalEvents      int            `json:"total_events"`
	EventsWithPII    int            `json:"events_with_pii"`
	PIITypeCounts    map[string]int `json:"pii_type_counts"`
	LastDetectedTime time.Time      `json:"last_detected_time,omitempty"`
}

// NewPIISummary 创建PII摘要
func NewPIISummary() *PIISummary {
	return &PIISummary{
		PIITypeCounts: make(map[string]int),
	}
}

// RecordEvent 记录事件到摘要
func (s *PIISummary) RecordEvent(hasPII bool, piiTypes []PIIPatternType) {
	s.TotalEvents++
	if hasPII {
		s.EventsWithPII++
		s.LastDetectedTime = time.Now()
		for _, t := range piiTypes {
			s.PIITypeCounts[string(t)]++
		}
	}
}