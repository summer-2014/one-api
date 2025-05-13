package middleware

import (
	"github.com/king133134/sensfilter"
	"github.com/songquanpeng/one-api/common/config"
	"sync"
)

var (
	// 增强的敏感词过滤器（处理大小写和全角半角）
	enhancedSensitiveFilter *sensfilter.Search
	enhancedFilterOnce      sync.Once
)

// 获取增强的敏感词过滤器实例
func GetEnhancedSensitiveFilter() *sensfilter.Search {
	enhancedFilterOnce.Do(func() {
		// 确保原始过滤器已初始化
		GetSensitiveFilter()

		// 创建增强的敏感词列表
		if len(config.SensitiveWords) > 0 {
			// 对每个敏感词进行处理
			normalizedWords := make([]string, 0, len(config.SensitiveWords))
			for _, word := range config.SensitiveWords {
				// 如果包含英文字母，则添加小写版本
				if containsEnglish(word) {
					normalizedWord := config.NormalizeText(word)
					// 避免重复添加
					if normalizedWord != word && !contains(normalizedWords, normalizedWord) {
						normalizedWords = append(normalizedWords, normalizedWord)
					}
				}
			}

			// 合并原始敏感词和处理后的敏感词
			allWords := append([]string{}, config.SensitiveWords...)
			allWords = append(allWords, normalizedWords...)

			// 创建增强的过滤器
			enhancedSensitiveFilter = sensfilter.Strings(allWords)
		} else {
			enhancedSensitiveFilter = sensfilter.NewSearch()
		}
	})
	return enhancedSensitiveFilter
}

// 检查字符串是否包含英文字母
func containsEnglish(s string) bool {
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			return true
		}
	}
	return false
}

// 检查字符串数组是否包含特定字符串
func contains(arr []string, str string) bool {
	for _, a := range arr {
		if a == str {
			return true
		}
	}
	return false
}

// 增强版敏感词检测
func containsEnhancedSensitiveWords(text string) bool {
	if !config.SensitiveFilterEnabled || text == "" {
		return false
	}

	// 先使用原始过滤器检测
	filter := GetSensitiveFilter()
	if filter.HasSens([]byte(text)) {
		return true
	}

	// 如果原始过滤器没有检测到，使用增强过滤器
	// 对输入文本进行标准化处理（小写和全角转半角）
	normalizedText := config.NormalizeText(text)
	if normalizedText == text {
		// 如果文本没有变化，则不需要再次检测
		return false
	}

	enhancedFilter := GetEnhancedSensitiveFilter()
	return enhancedFilter.HasSens([]byte(normalizedText))
}

// 更新增强的敏感词列表
func UpdateEnhancedSensitiveWords(words []string) {
	// 创建增强的敏感词列表
	normalizedWords := make([]string, 0, len(words))
	for _, word := range words {
		// 如果包含英文字母，则添加小写版本
		if containsEnglish(word) {
			normalizedWord := config.NormalizeText(word)
			// 避免重复添加
			if normalizedWord != word && !contains(normalizedWords, normalizedWord) {
				normalizedWords = append(normalizedWords, normalizedWord)
			}
		}
	}

	// 合并原始敏感词和处理后的敏感词
	allWords := append([]string{}, words...)
	allWords = append(allWords, normalizedWords...)

	// 创建增强的过滤器
	enhancedSensitiveFilter = sensfilter.Strings(allWords)
}
