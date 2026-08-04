package audit

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// KeywordLoader 关键词加载器
type KeywordLoader struct {
	basePath string
}

// NewKeywordLoader 创建关键词加载器
func NewKeywordLoader(basePath string) *KeywordLoader {
	return &KeywordLoader{
		basePath: basePath,
	}
}

// LoadFromFile 从文件加载关键词
func (l *KeywordLoader) LoadFromFile(filename string) ([]string, error) {
	filePath := filepath.Join(l.basePath, filename)

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open keyword file %s: %w", filePath, err)
	}
	defer file.Close()

	var keywords []string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// 跳过空行和注释
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		keywords = append(keywords, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read keyword file %s: %w", filePath, err)
	}

	return keywords, nil
}

// LoadFromPath 从指定路径加载关键词
func (l *KeywordLoader) LoadFromPath(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open keyword file %s: %w", path, err)
	}
	defer file.Close()

	var keywords []string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// 跳过空行和注释
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		keywords = append(keywords, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read keyword file %s: %w", path, err)
	}

	return keywords, nil
}

// LoadBuiltinKeywords 加载内置关键词
func (l *KeywordLoader) LoadBuiltinKeywords() (map[ContentSafetyCategory][]string, error) {
	keywords := make(map[ContentSafetyCategory][]string)

	builtinFiles := map[ContentSafetyCategory]string{
		CategoryPolitics:    "politics.txt",
		CategoryPornography: "pornography.txt",
		CategoryViolence:    "violence.txt",
		CategoryAdvertising: "advertising.txt",
	}

	for category, filename := range builtinFiles {
		kws, err := l.LoadFromFile(filename)
		if err != nil {
			// 如果内置文件不存在，使用空列表（不报错）
			keywords[category] = []string{}
			continue
		}
		keywords[category] = kws
	}

	return keywords, nil
}

// LoadCustomKeywords 加载自定义关键词文件
func (l *KeywordLoader) LoadCustomKeywords(paths []string) (map[ContentSafetyCategory][]string, error) {
	keywords := make(map[ContentSafetyCategory][]string)

	for _, path := range paths {
		kws, err := l.LoadFromPath(path)
		if err != nil {
			return nil, fmt.Errorf("failed to load custom keywords from %s: %w", path, err)
		}

		// 根据文件名判断类别（简单实现）
		filename := filepath.Base(path)
		category := l.inferCategory(filename)
		if category != "" {
			keywords[category] = append(keywords[category], kws...)
		}
	}

	return keywords, nil
}

// inferCategory 根据文件名推断类别
func (l *KeywordLoader) inferCategory(filename string) ContentSafetyCategory {
	filename = strings.ToLower(filename)

	if strings.Contains(filename, "politics") || strings.Contains(filename, "政治") {
		return CategoryPolitics
	}
	if strings.Contains(filename, "pornography") || strings.Contains(filename, "色情") {
		return CategoryPornography
	}
	if strings.Contains(filename, "violence") || strings.Contains(filename, "暴力") {
		return CategoryViolence
	}
	if strings.Contains(filename, "advertising") || strings.Contains(filename, "广告") {
		return CategoryAdvertising
	}

	return ""
}