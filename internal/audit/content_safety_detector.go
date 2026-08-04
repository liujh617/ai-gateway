package audit

import (
	"context"
	"strings"
	"sync"
)

// ContentSafetyCategory 定义内容安全类别
type ContentSafetyCategory string

const (
	CategoryPolitics     ContentSafetyCategory = "politics"
	CategoryPornography  ContentSafetyCategory = "pornography"
	CategoryViolence     ContentSafetyCategory = "violence"
	CategoryAdvertising  ContentSafetyCategory = "advertising"
)

// Threshold 阈值级别
type Threshold string

const (
	ThresholdLow    Threshold = "low"
	ThresholdMedium Threshold = "medium"
	ThresholdHigh   Threshold = "high"
)

// ContentSafetyMatch 表示检测到的内容安全匹配
type ContentSafetyMatch struct {
	Category ContentSafetyCategory `json:"category"`
	Keyword  string                `json:"keyword"`
	Position int                   `json:"position"`
	IsWildcard bool                `json:"is_wildcard,omitempty"`
}

// ContentSafetyResult 表示内容安全检测结果
type ContentSafetyResult struct {
	HasViolation bool                          `json:"has_violation"`
	Matches      []ContentSafetyMatch          `json:"matches,omitempty"`
	TotalCount   int                           `json:"total_count"`
	ByCategory   map[ContentSafetyCategory]int `json:"by_category,omitempty"`
}

// ContentSafetyDetector 内容安全检测器接口
type ContentSafetyDetector interface {
	// Detect 检测文本中的内容安全问题
	Detect(ctx context.Context, text string) (*ContentSafetyResult, error)
}

// KeywordDetector 基于关键词的内容安全检测器
type KeywordDetector struct {
	keywords     map[ContentSafetyCategory][]string
	wildcards    map[ContentSafetyCategory][]string
	threshold    Threshold
	categories   []ContentSafetyCategory
	mu           sync.RWMutex
}

// NewKeywordDetector 创建新的关键词检测器
func NewKeywordDetector(threshold Threshold, categories []ContentSafetyCategory) *KeywordDetector {
	if len(categories) == 0 {
		categories = []ContentSafetyCategory{
			CategoryPolitics,
			CategoryPornography,
			CategoryViolence,
			CategoryAdvertising,
		}
	}

	return &KeywordDetector{
		keywords:   make(map[ContentSafetyCategory][]string),
		wildcards:  make(map[ContentSafetyCategory][]string),
		threshold:  threshold,
		categories: categories,
	}
}

// LoadKeywords 加载关键词（内置或自定义）
func (d *KeywordDetector) LoadKeywords(category ContentSafetyCategory, keywords []string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// 分离普通关键词和通配符关键词
	var regularKeywords []string
	var wildcardKeywords []string

	for _, keyword := range keywords {
		keyword = strings.TrimSpace(keyword)
		if keyword == "" || strings.HasPrefix(keyword, "#") {
			continue // 跳过空行和注释
		}

		if strings.HasSuffix(keyword, "*") {
			wildcardKeywords = append(wildcardKeywords, keyword)
		} else {
			regularKeywords = append(regularKeywords, keyword)
		}
	}

	d.keywords[category] = regularKeywords
	d.wildcards[category] = wildcardKeywords

	return nil
}

// Detect 检测文本中的内容安全问题
func (d *KeywordDetector) Detect(ctx context.Context, text string) (*ContentSafetyResult, error) {
	if text == "" {
		return &ContentSafetyResult{
			HasViolation: false,
			TotalCount:   0,
		}, nil
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	var matches []ContentSafetyMatch
	byCategory := make(map[ContentSafetyCategory]int)

	textLower := strings.ToLower(text)

	for _, category := range d.categories {
		// 检测普通关键词
		for _, keyword := range d.keywords[category] {
			keywordLower := strings.ToLower(keyword)
			pos := strings.Index(textLower, keywordLower)
			if pos >= 0 {
				matches = append(matches, ContentSafetyMatch{
					Category:   category,
					Keyword:    keyword,
					Position:   pos,
					IsWildcard: false,
				})
				byCategory[category]++
			}
		}

		// 检测通配符关键词
		for _, wildcard := range d.wildcards[category] {
			prefix := strings.TrimSuffix(strings.ToLower(wildcard), "*")
			if strings.Contains(textLower, prefix) {
				matches = append(matches, ContentSafetyMatch{
					Category:   category,
					Keyword:    wildcard,
					Position:   strings.Index(textLower, prefix),
					IsWildcard: true,
				})
				byCategory[category]++
			}
		}
	}

	// 根据阈值判断是否违规
	hasViolation := d.applyThreshold(matches, byCategory)

	return &ContentSafetyResult{
		HasViolation: hasViolation,
		Matches:      matches,
		TotalCount:   len(matches),
		ByCategory:   byCategory,
	}, nil
}

// applyThreshold 根据阈值判断是否违规
func (d *KeywordDetector) applyThreshold(matches []ContentSafetyMatch, byCategory map[ContentSafetyCategory]int) bool {
	if len(matches) == 0 {
		return false
	}

	// 统计通配符匹配数量
	wildcardCount := 0
	for _, match := range matches {
		if match.IsWildcard {
			wildcardCount++
		}
	}

	// 统计总关键词匹配数量（不含通配符）
	keywordCount := len(matches) - wildcardCount

	switch d.threshold {
	case ThresholdLow:
		// 低阈值：匹配任意关键词即违规
		return len(matches) >= 1

	case ThresholdMedium:
		// 中阈值：匹配2+个关键词或1个通配符
		return keywordCount >= 2 || wildcardCount >= 1

	case ThresholdHigh:
		// 高阈值：匹配3+个关键词或2+个通配符
		return keywordCount >= 3 || wildcardCount >= 2

	default:
		return len(matches) >= 1
	}
}

// GetCategories 获取检测类别
func (d *KeywordDetector) GetCategories() []ContentSafetyCategory {
	return d.categories
}

// GetKeywordCount 获取关键词数量（用于调试和监控）
func (d *KeywordDetector) GetKeywordCount(category ContentSafetyCategory) int {
	d.mu.RLock()
	defer d.mu.RUnlock()

	count := len(d.keywords[category]) + len(d.wildcards[category])
	return count
}