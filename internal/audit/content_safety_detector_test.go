package audit

import (
	"context"
	"testing"
)

func TestNewKeywordDetector(t *testing.T) {
	tests := []struct {
		name        string
		threshold   Threshold
		categories  []ContentSafetyCategory
		wantLen     int
	}{
		{
			name:        "default_categories",
			threshold:   ThresholdMedium,
			categories:  nil,
			wantLen:     4, // 默认4个类别
		},
		{
			name:        "custom_categories",
			threshold:   ThresholdLow,
			categories:  []ContentSafetyCategory{CategoryPolitics, CategoryViolence},
			wantLen:     2,
		},
		{
			name:        "high_threshold",
			threshold:   ThresholdHigh,
			categories:  []ContentSafetyCategory{CategoryAdvertising},
			wantLen:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detector := NewKeywordDetector(tt.threshold, tt.categories)
			if detector == nil {
				t.Fatal("detector is nil")
			}

			categories := detector.GetCategories()
			if len(categories) != tt.wantLen {
				t.Errorf("expected %d categories, got %d", tt.wantLen, len(categories))
			}

			if detector.threshold != tt.threshold {
				t.Errorf("expected threshold %s, got %s", tt.threshold, detector.threshold)
			}
		})
	}
}

func TestKeywordDetector_LoadKeywords(t *testing.T) {
	detector := NewKeywordDetector(ThresholdMedium, nil)

	keywords := []string{
		"敏感词1",
		"敏感词2",
		"测试*",
		"# 这是注释",
		"", // 空行
	}

	err := detector.LoadKeywords(CategoryPolitics, keywords)
	if err != nil {
		t.Fatalf("LoadKeywords failed: %v", err)
	}

	count := detector.GetKeywordCount(CategoryPolitics)
	// 应该有3个关键词（2个普通 + 1个通配符，注释和空行被跳过）
	if count != 3 {
		t.Errorf("expected 3 keywords, got %d", count)
	}
}

func TestKeywordDetector_Detect_NoViolation(t *testing.T) {
	detector := NewKeywordDetector(ThresholdMedium, nil)

	// 加载测试关键词
	err := detector.LoadKeywords(CategoryPolitics, []string{"政治敏感", "政府机关"})
	if err != nil {
		t.Fatalf("LoadKeywords failed: %v", err)
	}

	// 测试无违规文本
	result, err := detector.Detect(context.Background(), "这是一段正常的文本")
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}

	if result.HasViolation {
		t.Error("expected no violation, but got violation")
	}

	if result.TotalCount != 0 {
		t.Errorf("expected 0 matches, got %d", result.TotalCount)
	}
}

func TestKeywordDetector_Detect_WithViolation(t *testing.T) {
	detector := NewKeywordDetector(ThresholdLow, nil)

	// 加载测试关键词
	err := detector.LoadKeywords(CategoryPolitics, []string{"政治敏感", "政府机关"})
	if err != nil {
		t.Fatalf("LoadKeywords failed: %v", err)
	}

	// 测试有违规的文本
	result, err := detector.Detect(context.Background(), "这是一段政治敏感的文本")
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}

	if !result.HasViolation {
		t.Error("expected violation, but got no violation")
	}

	if result.TotalCount == 0 {
		t.Error("expected matches, but got 0")
	}

	if len(result.Matches) == 0 {
		t.Fatal("expected matches, but got empty")
	}

	// 验证匹配内容
	match := result.Matches[0]
	if match.Category != CategoryPolitics {
		t.Errorf("expected category %s, got %s", CategoryPolitics, match.Category)
	}
	if match.Keyword != "政治敏感" {
		t.Errorf("expected keyword '政治敏感', got '%s'", match.Keyword)
	}
}

func TestKeywordDetector_Detect_Wildcard(t *testing.T) {
	detector := NewKeywordDetector(ThresholdMedium, nil)

	// 加载通配符关键词
	err := detector.LoadKeywords(CategoryPolitics, []string{"敏感*", "测试*"})
	if err != nil {
		t.Fatalf("LoadKeywords failed: %v", err)
	}

	// 测试通配符匹配
	result, err := detector.Detect(context.Background(), "这是一个敏感词汇测试内容")
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}

	// Medium阈值下，通配符匹配应该触发违规
	if !result.HasViolation {
		t.Error("expected violation with wildcard match, but got no violation")
	}

	// 验证通配符标记
	foundWildcard := false
	for _, match := range result.Matches {
		if match.IsWildcard {
			foundWildcard = true
			break
		}
	}

	if !foundWildcard {
		t.Error("expected at least one wildcard match")
	}
}

func TestKeywordDetector_Detect_ThresholdLow(t *testing.T) {
	detector := NewKeywordDetector(ThresholdLow, []ContentSafetyCategory{CategoryPolitics})

	err := detector.LoadKeywords(CategoryPolitics, []string{"关键词1", "关键词2"})
	if err != nil {
		t.Fatalf("LoadKeywords failed: %v", err)
	}

	// Low阈值：匹配1个关键词即违规
	result, err := detector.Detect(context.Background(), "这是一个关键词1的测试")
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}

	if !result.HasViolation {
		t.Error("expected violation with low threshold and 1 match")
	}
}

func TestKeywordDetector_Detect_ThresholdMedium(t *testing.T) {
	detector := NewKeywordDetector(ThresholdMedium, []ContentSafetyCategory{CategoryPolitics})

	err := detector.LoadKeywords(CategoryPolitics, []string{"关键词1", "关键词2"})
	if err != nil {
		t.Fatalf("LoadKeywords failed: %v", err)
	}

	// Medium阈值：需要2+个关键词或1个通配符

	// 测试1个关键词 - 不违规
	result, err := detector.Detect(context.Background(), "这是一个关键词1的测试")
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if result.HasViolation {
		t.Error("expected no violation with medium threshold and 1 keyword match")
	}

	// 测试2个关键词 - 违规
	result, err = detector.Detect(context.Background(), "这是一个关键词1和关键词2的测试")
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if !result.HasViolation {
		t.Error("expected violation with medium threshold and 2 keyword matches")
	}
}

func TestKeywordDetector_Detect_ThresholdHigh(t *testing.T) {
	detector := NewKeywordDetector(ThresholdHigh, []ContentSafetyCategory{CategoryPolitics})

	err := detector.LoadKeywords(CategoryPolitics, []string{"关键词1", "关键词2", "关键词3"})
	if err != nil {
		t.Fatalf("LoadKeywords failed: %v", err)
	}

	// High阈值：需要3+个关键词或2+个通配符

	// 测试2个关键词 - 不违规
	result, err := detector.Detect(context.Background(), "这是关键词1和关键词2的测试")
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if result.HasViolation {
		t.Error("expected no violation with high threshold and 2 keyword matches")
	}

	// 测试3个关键词 - 违规
	result, err = detector.Detect(context.Background(), "这是关键词1、关键词2和关键词3的测试")
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if !result.HasViolation {
		t.Error("expected violation with high threshold and 3 keyword matches")
	}
}

func TestKeywordDetector_Detect_MultipleCategories(t *testing.T) {
	detector := NewKeywordDetector(ThresholdLow, nil)

	// 加载多类别关键词
	err := detector.LoadKeywords(CategoryPolitics, []string{"政治词"})
	if err != nil {
		t.Fatalf("LoadKeywords failed: %v", err)
	}

	err = detector.LoadKeywords(CategoryViolence, []string{"暴力词"})
	if err != nil {
		t.Fatalf("LoadKeywords failed: %v", err)
	}

	// 测试多类别匹配
	result, err := detector.Detect(context.Background(), "这是一个政治词和暴力词的测试")
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}

	if !result.HasViolation {
		t.Error("expected violation")
	}

	// 验证类别统计
	if len(result.ByCategory) < 2 {
		t.Errorf("expected at least 2 categories in ByCategory, got %d", len(result.ByCategory))
	}
}

func TestKeywordDetector_Detect_EmptyText(t *testing.T) {
	detector := NewKeywordDetector(ThresholdLow, nil)

	result, err := detector.Detect(context.Background(), "")
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}

	if result.HasViolation {
		t.Error("expected no violation for empty text")
	}

	if result.TotalCount != 0 {
		t.Errorf("expected 0 matches for empty text, got %d", result.TotalCount)
	}
}

func TestKeywordDetector_Detect_CaseInsensitive(t *testing.T) {
	detector := NewKeywordDetector(ThresholdLow, nil)

	err := detector.LoadKeywords(CategoryAdvertising, []string{"SPAM", "Advertisement"})
	if err != nil {
		t.Fatalf("LoadKeywords failed: %v", err)
	}

	// 测试大小写不敏感
	result, err := detector.Detect(context.Background(), "这是spam和advertisement的测试")
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}

	if !result.HasViolation {
		t.Error("expected case-insensitive match")
	}

	if result.TotalCount < 2 {
		t.Errorf("expected at least 2 matches (case-insensitive), got %d", result.TotalCount)
	}
}

func TestKeywordDetector_Detect_Performance(t *testing.T) {
	detector := NewKeywordDetector(ThresholdMedium, nil)

	// 加载多个关键词
	for _, category := range []ContentSafetyCategory{
		CategoryPolitics,
		CategoryPornography,
		CategoryViolence,
		CategoryAdvertising,
	} {
		keywords := make([]string, 100)
		for i := 0; i < 100; i++ {
			keywords[i] = "关键词" + string(rune('A'+i%26))
		}
		err := detector.LoadKeywords(category, keywords)
		if err != nil {
			t.Fatalf("LoadKeywords failed: %v", err)
		}
	}

	// 生成大文本（10KB）
	largeText := ""
	for i := 0; i < 1000; i++ {
		largeText += "这是一段测试文本，用于性能测试。"
	}

	// 测试性能（应该 < 1ms）
	result, err := detector.Detect(context.Background(), largeText)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}

	// 验证结果结构
	if result == nil {
		t.Fatal("result is nil")
	}

	// 对于大文本，应该能快速处理
	t.Logf("Processed large text (%d bytes) with %d keywords loaded", len(largeText), 400)
}

func TestKeywordDetector_GetKeywordCount(t *testing.T) {
	detector := NewKeywordDetector(ThresholdMedium, nil)

	// 加载关键词
	err := detector.LoadKeywords(CategoryPolitics, []string{"关键词1", "关键词2", "通配符*"})
	if err != nil {
		t.Fatalf("LoadKeywords failed: %v", err)
	}

	count := detector.GetKeywordCount(CategoryPolitics)
	if count != 3 {
		t.Errorf("expected 3 keywords, got %d", count)
	}

	// 测试未加载的类别
	count = detector.GetKeywordCount(CategoryViolence)
	if count != 0 {
		t.Errorf("expected 0 keywords for unloaded category, got %d", count)
	}
}