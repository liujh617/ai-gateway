package audit

import (
	"context"
	"regexp"
	"strings"
	"testing"
)

func TestRegexDetector_Detect(t *testing.T) {
	detector := NewRegexDetector()

	tests := []struct {
		name        string
		text        string
		wantHasPII  bool
		wantCount   int
		wantTypes   []PIIPatternType
	}{
		{
			name:        "empty text",
			text:        "",
			wantHasPII:  false,
			wantCount:   0,
			wantTypes:   []PIIPatternType{},
		},
		{
			name:        "no PII",
			text:        "This is a normal text without any PII data",
			wantHasPII:  false,
			wantCount:   0,
			wantTypes:   []PIIPatternType{},
		},
		{
			name:        "phone number",
			text:        "联系我：13812345678",
			wantHasPII:  true,
			wantCount:   1,
			wantTypes:   []PIIPatternType{PIIPhoneNumber},
		},
		{
			name:        "ID card",
			text:        "身份证号：110101199003071234",
			wantHasPII:  true,
			wantCount:   1,
			wantTypes:   []PIIPatternType{PIIIDCard},
		},
		{
			name:        "bank card",
			text:        "银行卡号：6222021234567890123",
			wantHasPII:  true,
			wantCount:   1,
			wantTypes:   []PIIPatternType{PIIBankCardNumber},
		},
		{
			name:        "multiple PII types",
			text:        "电话：13812345678，身份证：110101199003071234",
			wantHasPII:  true,
			wantCount:   2,
			wantTypes:   []PIIPatternType{PIIPhoneNumber, PIIIDCard},
		},
		{
			name:        "multiple phone numbers",
			text:        "联系人1：13812345678，联系人2：13987654321",
			wantHasPII:  true,
			wantCount:   2,
			wantTypes:   []PIIPatternType{PIIPhoneNumber},
		},
		{
			name:        "phone number with spaces",
			text:        "电话号码是 138 1234 5678",
			wantHasPII:  false, // 空格分隔的不匹配
			wantCount:   0,
			wantTypes:   []PIIPatternType{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := detector.Detect(context.Background(), tt.text)
			if err != nil {
				t.Fatalf("Detect() error = %v", err)
			}

			if result.HasPII != tt.wantHasPII {
				t.Errorf("Detect() HasPII = %v, want %v", result.HasPII, tt.wantHasPII)
			}

			if result.TotalCount != tt.wantCount {
				t.Errorf("Detect() TotalCount = %v, want %v", result.TotalCount, tt.wantCount)
			}

			// 检查类型是否匹配（顺序可能不同）
			if len(result.TypesFound) != len(tt.wantTypes) {
				t.Errorf("Detect() TypesFound length = %v, want %v", len(result.TypesFound), len(tt.wantTypes))
			}
		})
	}
}

func TestRegexDetector_Detect_Cancel(t *testing.T) {
	detector := NewRegexDetector()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	_, err := detector.Detect(ctx, "some text with phone 13812345678")
	if err == nil {
		t.Error("Detect() should return error when context is canceled")
	}
}

func TestRegexDetector_AddPattern(t *testing.T) {
	detector := NewRegexDetector()

	// 测试添加有效模式
	validPattern := &PIIPattern{
		Type:        "test_pattern",
		Name:        "Test Pattern",
		Description: "A test pattern",
		Pattern:     regexp.MustCompile(`test\d{3}`),
	}

	err := detector.AddPattern(validPattern)
	if err != nil {
		t.Errorf("AddPattern() error = %v", err)
	}

	// 测试添加无效模式
	invalidPattern := &PIIPattern{
		Type:        "invalid",
		Name:        "Invalid",
		Description: "Invalid pattern",
		Pattern:     nil,
	}

	err = detector.AddPattern(invalidPattern)
	if err == nil {
		t.Error("AddPattern() should return error for invalid pattern")
	}

	// 测试添加nil模式
	err = detector.AddPattern(nil)
	if err == nil {
		t.Error("AddPattern() should return error for nil pattern")
	}
}

func TestNoopDetector_Detect(t *testing.T) {
	detector := NewNoopDetector()

	result, err := detector.Detect(context.Background(), "some text with phone 13812345678")
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}

	if result.HasPII {
		t.Error("NoopDetector should always return HasPII = false")
	}

	if result.TotalCount != 0 {
		t.Error("NoopDetector should always return TotalCount = 0")
	}
}

func TestRedactPII(t *testing.T) {
	text := "联系人：张三，电话：13812345678，身份证：110101199003071234"
	matches := []PIIMatch{
		{
			Type:       PIIPhoneNumber,
			Name:       "中国手机号",
			Value:      "13812345678",
			StartIndex: 23,
			EndIndex:   34,
		},
		{
			Type:       PIIIDCard,
			Name:       "中国身份证号",
			Value:      "110101199003071234",
			StartIndex: 41,
			EndIndex:   59,
		},
	}

	redacted := RedactPII(text, matches, "***")

	expected := "联系人：张三，电话：***，身份证：***"
	if redacted != expected {
		t.Errorf("RedactPII() = %v, want %v", redacted, expected)
	}
}

func TestRedactPII_EmptyMatches(t *testing.T) {
	text := "some text without PII"
	matches := []PIIMatch{}

	redacted := RedactPII(text, matches, "***")

	if redacted != text {
		t.Errorf("RedactPII() should return original text when no matches")
	}
}

func TestContainsPII(t *testing.T) {
	detector := NewRegexDetector()

	// 测试包含PII
	hasPII, err := ContainsPII(detector, "电话：13812345678")
	if err != nil {
		t.Fatalf("ContainsPII() error = %v", err)
	}
	if !hasPII {
		t.Error("ContainsPII() should return true for text with phone number")
	}

	// 测试不包含PII
	hasPII, err = ContainsPII(detector, "这是一段普通文本")
	if err != nil {
		t.Fatalf("ContainsPII() error = %v", err)
	}
	if hasPII {
		t.Error("ContainsPII() should return false for text without PII")
	}
}

func TestCountPIIByType(t *testing.T) {
	matches := []PIIMatch{
		{Type: PIIPhoneNumber},
		{Type: PIIPhoneNumber},
		{Type: PIIIDCard},
		{Type: PIIBankCardNumber},
		{Type: PIIBankCardNumber},
		{Type: PIIBankCardNumber},
	}

	counts := CountPIIByType(matches)

	if counts[PIIPhoneNumber] != 2 {
		t.Errorf("CountPIIByType() phone count = %v, want 2", counts[PIIPhoneNumber])
	}

	if counts[PIIIDCard] != 1 {
		t.Errorf("CountPIIByType() ID card count = %v, want 1", counts[PIIIDCard])
	}

	if counts[PIIBankCardNumber] != 3 {
		t.Errorf("CountPIIByType() bank card count = %v, want 3", counts[PIIBankCardNumber])
	}
}

func TestFilterPIIByType(t *testing.T) {
	matches := []PIIMatch{
		{Type: PIIPhoneNumber, Value: "13812345678"},
		{Type: PIIIDCard, Value: "110101199003071234"},
		{Type: PIIPhoneNumber, Value: "13987654321"},
	}

	filtered := FilterPIIByType(matches, PIIPhoneNumber)

	if len(filtered) != 2 {
		t.Errorf("FilterPIIByType() should return 2 phone matches, got %d", len(filtered))
	}

	for _, match := range filtered {
		if match.Type != PIIPhoneNumber {
			t.Errorf("FilterPIIByType() should only return phone matches")
		}
	}
}

func TestValidatePIIPattern(t *testing.T) {
	// 测试有效模式
	validPattern := &PIIPattern{
		Type:    "test",
		Name:    "Test",
		Pattern: regexp.MustCompile(`test`),
	}

	err := ValidatePIIPattern(validPattern)
	if err != nil {
		t.Errorf("ValidatePIIPattern() should return nil for valid pattern, got %v", err)
	}

	// 测试nil模式
	err = ValidatePIIPattern(nil)
	if err == nil {
		t.Error("ValidatePIIPattern() should return error for nil pattern")
	}

	// 测试空类型
	emptyTypePattern := &PIIPattern{
		Type:    "",
		Name:    "Test",
		Pattern: regexp.MustCompile(`test`),
	}

	err = ValidatePIIPattern(emptyTypePattern)
	if err == nil {
		t.Error("ValidatePIIPattern() should return error for empty type")
	}

	// 测试空名称
	emptyNamePattern := &PIIPattern{
		Type:    "test",
		Name:    "",
		Pattern: regexp.MustCompile(`test`),
	}

	err = ValidatePIIPattern(emptyNamePattern)
	if err == nil {
		t.Error("ValidatePIIPattern() should return error for empty name")
	}

	// 测试nil正则
	nilRegexPattern := &PIIPattern{
		Type:    "test",
		Name:    "Test",
		Pattern: nil,
	}

	err = ValidatePIIPattern(nilRegexPattern)
	if err == nil {
		t.Error("ValidatePIIPattern() should return error for nil regex")
	}
}

func TestPIIMatch_String(t *testing.T) {
	match := PIIMatch{
		Type:  PIIPhoneNumber,
		Name:  "中国手机号",
		Value: "13812345678",
	}

	got := match.String()
	expected := "中国手机号:phone_number"
	if got != expected {
		t.Errorf("PIIMatch.String() = %v, want %v", got, expected)
	}
}

func TestPIIDetectionResult_String(t *testing.T) {
	// 测试无PII
	noPIIResult := &PIIDetectionResult{
		HasPII: false,
	}
	if noPIIResult.String() != "No PII detected" {
		t.Errorf("PIIDetectionResult.String() for no PII should be 'No PII detected'")
	}

	// 测试有PII
	hasPIIResult := &PIIDetectionResult{
		HasPII:     true,
		TotalCount: 3,
		TypesFound: []PIIPatternType{PIIPhoneNumber, PIIIDCard},
	}

	got := hasPIIResult.String()
	if !strings.Contains(got, "PII detected") {
		t.Errorf("PIIDetectionResult.String() should contain 'PII detected'")
	}
}

// 性能测试
func TestRegexDetector_Detect_Performance(t *testing.T) {
	detector := NewRegexDetector()

	// 生成包含多个PII的长文本
	text := strings.Repeat("这是一段测试文本，电话：13812345678，身份证：110101199003071234，银行卡：6222021234567890123 ", 100)

	for i := 0; i < 100; i++ {
		result, err := detector.Detect(context.Background(), text)
		if err != nil {
			t.Fatalf("Detect() error = %v", err)
		}

		if result.TotalCount != 300 { // 每行3个PII，100行
			t.Errorf("Detect() TotalCount = %v, want 300", result.TotalCount)
		}
	}
}

func TestRegexDetector_Detect_Concurrency(t *testing.T) {
	detector := NewRegexDetector()

	// 并发检测
	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func() {
			text := "电话：13812345678，身份证：110101199003071234"
			result, err := detector.Detect(context.Background(), text)
			if err != nil {
				t.Errorf("Detect() error = %v", err)
			}
			if !result.HasPII {
				t.Error("Detect() should detect PII")
			}
			done <- true
		}()
	}

	// 等待所有goroutine完成
	for i := 0; i < 10; i++ {
		<-done
	}
}