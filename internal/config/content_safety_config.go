package config

import (
	"os"
	"strings"
)

// ContentSafetyAction 内容安全动作
type ContentSafetyAction string

const (
	ContentSafetyActionAlert  ContentSafetyAction = "alert"  // 记录告警
	ContentSafetyActionReject ContentSafetyAction = "reject" // 拒绝请求
	ContentSafetyActionAllow  ContentSafetyAction = "allow"  // 允许通过
)

// ContentSafetyThreshold 阈值级别
type ContentSafetyThreshold string

const (
	ThresholdLow    ContentSafetyThreshold = "low"
	ThresholdMedium ContentSafetyThreshold = "medium"
	ThresholdHigh   ContentSafetyThreshold = "high"
)

// CustomKeywordsConfig 自定义关键词配置
type CustomKeywordsConfig struct {
	Enabled bool     `json:"enabled"`        // 是否启用自定义关键词
	Paths   []string `json:"paths,omitempty"` // 关键词文件路径列表
}

// ContentSafetyConfig 内容安全检测配置
type ContentSafetyConfig struct {
	Enabled         bool                  `json:"enabled"`           // 是否启用内容安全检测
	Action          string                `json:"action"`            // 检测到违规时的动作：alert/reject/allow
	Categories      []string              `json:"categories"`        // 检测类别：politics/pornography/violence/advertising
	CustomKeywords  CustomKeywordsConfig  `json:"custom_keywords"`   // 自定义关键词配置
	LogMatches      bool                  `json:"log_matches"`       // 是否记录匹配的关键词
	Threshold       string                `json:"threshold"`         // 阈值级别：low/medium/high
}

// DefaultContentSafetyConfig 返回默认内容安全配置
func DefaultContentSafetyConfig() ContentSafetyConfig {
	return ContentSafetyConfig{
		Enabled:    false, // 默认禁用
		Action:     string(ContentSafetyActionAlert),
		Categories: []string{"politics", "pornography", "violence", "advertising"},
		CustomKeywords: CustomKeywordsConfig{
			Enabled: false,
			Paths:   []string{},
		},
		LogMatches: true,
		Threshold:  string(ThresholdMedium),
	}
}

// Validate 验证内容安全配置
func (c *ContentSafetyConfig) Validate() error {
	if !c.Enabled {
		return nil
	}

	// 验证action
	validActions := map[string]bool{
		string(ContentSafetyActionAlert):  true,
		string(ContentSafetyActionReject): true,
		string(ContentSafetyActionAllow):  true,
	}

	if !validActions[c.Action] {
		return &ConfigError{
			Field:   "content_safety.action",
			Message: "invalid action, must be one of: alert, reject, allow",
		}
	}

	// 验证categories
	if len(c.Categories) == 0 {
		return &ConfigError{
			Field:   "content_safety.categories",
			Message: "categories must be a non-empty array",
		}
	}

	validCategories := map[string]bool{
		"politics":    true,
		"pornography": true,
		"violence":    true,
		"advertising": true,
	}

	for _, category := range c.Categories {
		if !validCategories[category] {
			return &ConfigError{
				Field:   "content_safety.categories",
				Message: "invalid category: " + category + ", must be one of: politics, pornography, violence, advertising",
			}
		}
	}

	// 验证threshold
	validThresholds := map[string]bool{
		string(ThresholdLow):    true,
		string(ThresholdMedium): true,
		string(ThresholdHigh):   true,
	}

	if !validThresholds[c.Threshold] {
		return &ConfigError{
			Field:   "content_safety.threshold",
			Message: "invalid threshold, must be one of: low, medium, high",
		}
	}

	// 验证自定义关键词路径
	if c.CustomKeywords.Enabled && len(c.CustomKeywords.Paths) == 0 {
		return &ConfigError{
			Field:   "content_safety.custom_keywords.paths",
			Message: "custom keywords enabled but no paths provided",
		}
	}

	return nil
}

// GetAction 返回ContentSafetyAction类型
func (c *ContentSafetyConfig) GetAction() ContentSafetyAction {
	return ContentSafetyAction(c.Action)
}

// GetThreshold 返回ContentSafetyThreshold类型
func (c *ContentSafetyConfig) GetThreshold() ContentSafetyThreshold {
	return ContentSafetyThreshold(c.Threshold)
}

// ShouldDetectCategory 返回是否应该检测指定类别
func (c *ContentSafetyConfig) ShouldDetectCategory(category string) bool {
	if !c.Enabled {
		return false
	}

	for _, cat := range c.Categories {
		if cat == category {
			return true
		}
	}

	return false
}

// GetCategoriesString 返回类别列表的字符串表示
func (c *ContentSafetyConfig) GetCategoriesString() string {
	return strings.Join(c.Categories, ",")
}

// ApplyEnvOverrides 应用环境变量覆盖
func (c *ContentSafetyConfig) ApplyEnvOverrides() {
	if val := os.Getenv("GATEWAY_CONTENT_SAFETY_ENABLED"); val != "" {
		c.Enabled = strings.ToLower(val) == "true"
	}

	if val := os.Getenv("GATEWAY_CONTENT_SAFETY_ACTION"); val != "" {
		c.Action = val
	}

	if val := os.Getenv("GATEWAY_CONTENT_SAFETY_CATEGORIES"); val != "" {
		c.Categories = strings.Split(val, ",")
	}

	if val := os.Getenv("GATEWAY_CONTENT_SAFETY_THRESHOLD"); val != "" {
		c.Threshold = val
	}

	if val := os.Getenv("GATEWAY_CONTENT_SAFETY_LOG_MATCHES"); val != "" {
		c.LogMatches = strings.ToLower(val) == "true"
	}
}