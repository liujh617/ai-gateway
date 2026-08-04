package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

// ContentSafetyAction 内容安全动作类型
type ContentSafetyAction string

const (
	ContentSafetyActionAlert  ContentSafetyAction = "alert"
	ContentSafetyActionReject ContentSafetyAction = "reject"
	ContentSafetyActionAllow  ContentSafetyAction = "allow"
)

// ContentSafetyAuditor 内容安全审计器
type ContentSafetyAuditor struct {
	detector ContentSafetyDetector
	action   ContentSafetyAction
	logger   *slog.Logger
}

// NewContentSafetyAuditor 创建内容安全审计器
func NewContentSafetyAuditor(detector ContentSafetyDetector, action ContentSafetyAction, logger *slog.Logger) *ContentSafetyAuditor {
	if logger == nil {
		logger = slog.Default()
	}
	return &ContentSafetyAuditor{
		detector: detector,
		action:   action,
		logger:   logger,
	}
}

// AuditRequest 审计请求中的内容安全问题
func (a *ContentSafetyAuditor) AuditRequest(ctx context.Context, event *Event) error {
	if a.detector == nil {
		return nil
	}

	// 检测Body中的内容安全问题
	bodyText := string(event.Body)
	result, err := a.detector.Detect(ctx, bodyText)
	if err != nil {
		return fmt.Errorf("content safety detection failed: %w", err)
	}

	if !result.HasViolation {
		return nil
	}

	// 记录内容安全检测事件
	a.logContentSafetyViolation(event, result)

	// 根据action处理
	switch a.action {
	case ContentSafetyActionAlert:
		// 只记录告警，不阻止
		return nil
	case ContentSafetyActionReject:
		// 拒绝请求
		return &ContentSafetyError{
			Message: fmt.Sprintf("Unsafe content detected: %d violations found", result.TotalCount),
		}
	case ContentSafetyActionAllow:
		// 允许通过
		return nil
	default:
		return nil
	}
}

// logContentSafetyViolation 记录内容安全检测日志
func (a *ContentSafetyAuditor) logContentSafetyViolation(event *Event, result *ContentSafetyResult) {
	a.logger.Warn("Unsafe content detected in audit log",
		"request_id", event.RequestID,
		"client", event.Client,
		"violation_count", result.TotalCount,
		"categories", result.ByCategory,
		"event_type", event.Event,
		"timestamp", event.Timestamp,
	)
}

// ContentSafetyError 内容安全检测错误
type ContentSafetyError struct {
	Message string
}

func (e *ContentSafetyError) Error() string {
	return e.Message
}

// AuditEventWithContentSafety 包含内容安全信息的审计事件
type AuditEventWithContentSafety struct {
	Event
	ContentSafetyResult *ContentSafetyResult `json:"content_safety_violation,omitempty"`
}

// ToJSON 转换为JSON格式
func (e *AuditEventWithContentSafety) ToJSON() (json.RawMessage, error) {
	return json.Marshal(e)
}

// ContentSafetyAuditorRecorder 内容安全审计记录器包装器
type ContentSafetyAuditorRecorder struct {
	recorder Recorder
	auditor  *ContentSafetyAuditor
}

// NewContentSafetyAuditorRecorder 创建内容安全审计记录器
func NewContentSafetyAuditorRecorder(recorder Recorder, auditor *ContentSafetyAuditor) *ContentSafetyAuditorRecorder {
	return &ContentSafetyAuditorRecorder{
		recorder: recorder,
		auditor:  auditor,
	}
}

// Record 实现Recorder接口，添加内容安全检测
func (r *ContentSafetyAuditorRecorder) Record(ctx context.Context, event Event) {
	// 创建事件副本
	eventCopy := event

	// 执行内容安全检测
	if r.auditor != nil && len(eventCopy.Body) > 0 {
		result, err := r.auditor.detector.Detect(ctx, string(eventCopy.Body))
		if err != nil {
			r.auditor.logger.Error("Content safety detection failed", "error", err)
		} else if result.HasViolation {
			// 记录内容安全检测日志
			r.auditor.logContentSafetyViolation(&eventCopy, result)

			// 根据action处理
			if r.auditor.action == ContentSafetyActionReject {
				// 不记录包含违规内容的事件
				return
			}
		}
	}

	// 调用底层记录器
	r.recorder.Record(ctx, eventCopy)
}

// ContentSafetyAwareJSONLRecorder 支持内容安全检测的JSONL记录器
type ContentSafetyAwareJSONLRecorder struct {
	*JSONLRecorder
	detector ContentSafetyDetector
	action   ContentSafetyAction
	logger   *slog.Logger
}

// NewContentSafetyAwareJSONLRecorder 创建支持内容安全检测的JSONL记录器
func NewContentSafetyAwareJSONLRecorder(path string, options JSONLRecorderOptions, detector ContentSafetyDetector, action ContentSafetyAction) (*ContentSafetyAwareJSONLRecorder, error) {
	recorder, err := NewJSONLRecorderWithOptions(path, options)
	if err != nil {
		return nil, err
	}

	return &ContentSafetyAwareJSONLRecorder{
		JSONLRecorder: recorder,
		detector:      detector,
		action:        action,
		logger:        slog.Default(),
	}, nil
}

// Record 实现Recorder接口，添加内容安全检测
func (r *ContentSafetyAwareJSONLRecorder) Record(ctx context.Context, event Event) {
	// 检查context是否已取消
	if ctx.Err() != nil {
		return
	}

	// 执行内容安全检测
	if r.detector != nil && len(event.Body) > 0 {
		result, err := r.detector.Detect(ctx, string(event.Body))
		if err != nil {
			r.logger.Error("Content safety detection failed", "error", err, "request_id", event.RequestID)
		} else if result.HasViolation {
			// 记录内容安全检测日志
			r.logger.Warn("Unsafe content detected in audit log",
				"request_id", event.RequestID,
				"client", event.Client,
				"violation_count", result.TotalCount,
				"categories", result.ByCategory,
			)

			// 根据action处理
			if r.action == ContentSafetyActionReject {
				// 不记录包含违规内容的事件
				return
			}
		}
	}

	// 调用父类的Record方法
	r.JSONLRecorder.Record(ctx, event)
}

// ContentSafetySummary 内容安全检测摘要
type ContentSafetySummary struct {
	TotalEvents           int                          `json:"total_events"`
	EventsWithViolation   int                          `json:"events_with_violation"`
	ViolationCategoryCounts map[string]int             `json:"violation_category_counts"`
	LastDetectedTime      time.Time                    `json:"last_detected_time,omitempty"`
}

// NewContentSafetySummary 创建内容安全摘要
func NewContentSafetySummary() *ContentSafetySummary {
	return &ContentSafetySummary{
		ViolationCategoryCounts: make(map[string]int),
	}
}

// RecordEvent 记录事件到摘要
func (s *ContentSafetySummary) RecordEvent(hasViolation bool, categories map[ContentSafetyCategory]int) {
	s.TotalEvents++
	if hasViolation {
		s.EventsWithViolation++
		s.LastDetectedTime = time.Now()
		for category, count := range categories {
			s.ViolationCategoryCounts[string(category)] += count
		}
	}
}