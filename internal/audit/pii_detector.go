package audit

import (
	"context"
	"regexp"
	"strings"
)

// PIIPatternType 定义PII数据类型
type PIIPatternType string

const (
	PIIPhoneNumber    PIIPatternType = "phone_number"
	PIIIDCard         PIIPatternType = "id_card"
	PIIBankCardNumber PIIPatternType = "bank_card_number"
)

// PIIPattern 定义PII检测模式
type PIIPattern struct {
	Type        PIIPatternType
	Name        string
	Description string
	Pattern     *regexp.Regexp
}

// PIIMatch 表示检测到的PII匹配
type PIIMatch struct {
	Type        PIIPatternType `json:"type"`
	Name        string         `json:"name"`
	Value       string         `json:"value,omitempty"`   // 原始值（可脱敏）
	StartIndex  int            `json:"start_index"`       // 开始位置
	EndIndex    int            `json:"end_index"`         // 结束位置
	Confidence  float64        `json:"confidence"`        // 置信度 (0.0-1.0)
}

// PIIDetectionResult 表示PII检测结果
type PIIDetectionResult struct {
	HasPII      bool         `json:"has_pii"`
	Matches     []PIIMatch   `json:"matches,omitempty"`
	TotalCount  int          `json:"total_count"`
	TypesFound  []PIIPatternType `json:"types_found,omitempty"`
}

// Detector 定义PII检测器接口
type Detector interface {
	// Detect 检测文本中的PII数据
	Detect(ctx context.Context, text string) (*PIIDetectionResult, error)
}

// RegexDetector 基于正则表达式的PII检测器
type RegexDetector struct {
	patterns []*PIIPattern
}

// NewRegexDetector 创建新的正则检测器
func NewRegexDetector() *RegexDetector {
	return &RegexDetector{
		patterns: getDefaultPIIPatterns(),
	}
}

// NewRegexDetectorWithPatterns 创建自定义模式的正则检测器
func NewRegexDetectorWithPatterns(patterns []*PIIPattern) *RegexDetector {
	return &RegexDetector{
		patterns: patterns,
	}
}

// getDefaultPIIPatterns 返回默认的PII检测模式
func getDefaultPIIPatterns() []*PIIPattern {
	return []*PIIPattern{
		{
			Type:        PIIPhoneNumber,
			Name:        "中国手机号",
			Description: "中国大陆手机号码（11位，以1开头）",
			Pattern:     regexp.MustCompile(`1[3-9]\d{9}`),
		},
		{
			Type:        PIIIDCard,
			Name:        "中国身份证号",
			Description: "中国居民身份证号码（15位或18位）",
			Pattern:     regexp.MustCompile(`[1-9]\d{5}(?:18|19|20)\d{2}(?:0[1-9]|1[0-2])(?:0[1-9]|[12]\d|3[01])\d{3}[\dXx]`),
		},
		{
			Type:        PIIBankCardNumber,
			Name:        "银行卡号",
			Description: "中国银行卡号（16-19位数字）",
			Pattern:     regexp.MustCompile(`(?:62|4|5)\d{14,18}`),
		},
	}
}

// Detect 实现Detector接口，检测文本中的PII
func (d *RegexDetector) Detect(ctx context.Context, text string) (*PIIDetectionResult, error) {
	if text == "" {
		return &PIIDetectionResult{
			HasPII:     false,
			Matches:    []PIIMatch{},
			TotalCount: 0,
			TypesFound: []PIIPatternType{},
		}, nil
	}

	matches := []PIIMatch{}
	typesFoundMap := make(map[PIIPatternType]bool)

	for _, pattern := range d.patterns {
		// 检查context是否已取消
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		// 查找所有匹配
		locs := pattern.Pattern.FindAllStringIndex(text, -1)
		for _, loc := range locs {
			start, end := loc[0], loc[1]
			value := text[start:end]

			match := PIIMatch{
				Type:       pattern.Type,
				Name:       pattern.Name,
				Value:      value,
				StartIndex: start,
				EndIndex:   end,
				Confidence: 0.9, // 正则匹配的置信度固定为0.9
			}
			matches = append(matches, match)
			typesFoundMap[pattern.Type] = true
		}
	}

	// 转换typesFoundMap为数组
	typesFound := make([]PIIPatternType, 0, len(typesFoundMap))
	for t := range typesFoundMap {
		typesFound = append(typesFound, t)
	}

	return &PIIDetectionResult{
		HasPII:     len(matches) > 0,
		Matches:    matches,
		TotalCount: len(matches),
		TypesFound: typesFound,
	}, nil
}

// AddPattern 添加自定义PII模式
func (d *RegexDetector) AddPattern(pattern *PIIPattern) error {
	if pattern == nil || pattern.Pattern == nil {
		return ErrInvalidPattern
	}
	d.patterns = append(d.patterns, pattern)
	return nil
}

// ErrInvalidPattern 无效模式错误
var ErrInvalidPattern = &PIIDetectionError{
	Message: "invalid PII pattern",
}

// PIIDetectionError PII检测错误
type PIIDetectionError struct {
	Message string
}

func (e *PIIDetectionError) Error() string {
	return e.Message
}

// NoopDetector 空检测器（不执行任何检测）
type NoopDetector struct{}

// NewNoopDetector 创建空检测器
func NewNoopDetector() *NoopDetector {
	return &NoopDetector{}
}

// Detect 实现Detector接口，返回空结果
func (d *NoopDetector) Detect(ctx context.Context, text string) (*PIIDetectionResult, error) {
	return &PIIDetectionResult{
		HasPII:     false,
		Matches:    []PIIMatch{},
		TotalCount: 0,
		TypesFound: []PIIPatternType{},
	}, nil
}

// Helper functions

// RedactPII 脱敏PII数据
func RedactPII(text string, matches []PIIMatch, replacement string) string {
	if len(matches) == 0 {
		return text
	}

	// 从后往前替换，避免索引变化
	for i := len(matches) - 1; i >= 0; i-- {
		match := matches[i]
		text = text[:match.StartIndex] + replacement + text[match.EndIndex:]
	}

	return text
}

// ContainsPII 快速检查文本是否包含PII
func ContainsPII(detector Detector, text string) (bool, error) {
	result, err := detector.Detect(context.Background(), text)
	if err != nil {
		return false, err
	}
	return result.HasPII, nil
}

// CountPIIByType 按类型统计PII数量
func CountPIIByType(matches []PIIMatch) map[PIIPatternType]int {
	counts := make(map[PIIPatternType]int)
	for _, match := range matches {
		counts[match.Type]++
	}
	return counts
}

// FilterPIIByType 按类型过滤PII匹配
func FilterPIIByType(matches []PIIMatch, patternType PIIPatternType) []PIIMatch {
	filtered := []PIIMatch{}
	for _, match := range matches {
		if match.Type == patternType {
			filtered = append(filtered, match)
		}
	}
	return filtered
}

// ValidatePIIPattern 验证PII模式是否有效
func ValidatePIIPattern(pattern *PIIPattern) error {
	if pattern == nil {
		return ErrInvalidPattern
	}
	if pattern.Type == "" {
		return &PIIDetectionError{Message: "pattern type cannot be empty"}
	}
	if pattern.Name == "" {
		return &PIIDetectionError{Message: "pattern name cannot be empty"}
	}
	if pattern.Pattern == nil {
		return &PIIDetectionError{Message: "pattern regex cannot be nil"}
	}
	return nil
}

// String 实现Stringer接口
func (m *PIIMatch) String() string {
	return m.Name + ":" + m.Type
}

// String 实现Stringer接口
func (r *PIIDetectionResult) String() string {
	if !r.HasPII {
		return "No PII detected"
	}
	return strings.Join([]string{
		"PII detected:",
		"  Total count:", string(r.TotalCount),
		"  Types:", strings.Join(func() []string {
			types := make([]string, len(r.TypesFound))
			for i, t := range r.TypesFound {
				types[i] = string(t)
			}
			return types
		}(), ","),
	}, "\n")
}