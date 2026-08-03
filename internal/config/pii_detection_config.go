package config

// PIIDetectionAction 定义PII检测动作
type PIIDetectionAction string

const (
	PIIActionAlert   PIIDetectionAction = "alert"   // 记录告警
	PIIActionReject  PIIDetectionAction = "reject"  // 拒绝请求
	PIIActionAllow   PIIDetectionAction = "allow"   // 允许通过（默认）
)

// PIIDetectionConfig PII检测配置
type PIIDetectionConfig struct {
	Enabled   bool   `json:"enabled"`            // 是否启用PII检测
	Action    string `json:"action"`             // 检测到PII时的动作：alert/reject/allow
	LogPII    bool   `json:"log_pii"`            // 是否在日志中记录PII内容（默认false，脱敏）
	RedactPII bool   `json:"redact_pii"`         // 是否脱敏PII（默认true）
	
	// 检测范围
	DetectPhoneNumber    bool `json:"detect_phone_number"`    // 检测手机号（默认true）
	DetectIDCard         bool `json:"detect_id_card"`         // 检测身份证号（默认true）
	DetectBankCardNumber bool `json:"detect_bank_card_number"` // 检测银行卡号（默认true）
}

// DefaultPIIDetectionConfig 返回默认PII检测配置
func DefaultPIIDetectionConfig() PIIDetectionConfig {
	return PIIDetectionConfig{
		Enabled:              false, // 默认禁用
		Action:               string(PIIActionAlert),
		LogPII:               false,
		RedactPII:            true,
		DetectPhoneNumber:    true,
		DetectIDCard:         true,
		DetectBankCardNumber: true,
	}
}

// Validate 验证PII检测配置
func (c *PIIDetectionConfig) Validate() error {
	if !c.Enabled {
		return nil
	}

	// 验证action
	validActions := map[string]bool{
		string(PIIActionAlert):  true,
		string(PIIActionReject): true,
		string(PIIActionAllow):  true,
	}

	if !validActions[c.Action] {
		return &ConfigError{
			Field:   "pii_detection.action",
			Message: "invalid action, must be one of: alert, reject, allow",
		}
	}

	return nil
}

// GetAction 返回PIIDetectionAction类型
func (c *PIIDetectionConfig) GetAction() PIIDetectionAction {
	return PIIDetectionAction(c.Action)
}

// ShouldDetect 返回是否应该检测指定类型的PII
func (c *PIIDetectionConfig) ShouldDetect(piitype string) bool {
	if !c.Enabled {
		return false
	}

	switch piitype {
	case "phone_number":
		return c.DetectPhoneNumber
	case "id_card":
		return c.DetectIDCard
	case "bank_card_number":
		return c.DetectBankCardNumber
	default:
		return false
	}
}

// ConfigError 配置错误
type ConfigError struct {
	Field   string
	Message string
}

func (e *ConfigError) Error() string {
	return e.Field + ": " + e.Message
}