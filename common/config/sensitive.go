package config

import (
	"log"
	"os"
	"path/filepath"
	"strings"
)

// 从文件中加载敏感词列表
func LoadSensitiveWordsFromFile() {
	// 确保配置目录存在
	ensureConfigDir()

	// 如果文件不存在，创建默认文件
	if !fileExists(SensitiveWordsFile) {
		// 创建默认的敏感词文件
		err := os.WriteFile(SensitiveWordsFile, []byte("敏感词1\n敏感词2\n敏感词3"), 0644)
		if err != nil {
			log.Printf("创建默认敏感词文件失败: %s", err.Error())
			return
		}
		log.Printf("已创建默认敏感词文件: %s", SensitiveWordsFile)
	}

	// 读取敏感词文件
	content, err := os.ReadFile(SensitiveWordsFile)
	if err != nil {
		log.Printf("读取敏感词文件失败: %s", err.Error())
		return
	}

	// 解析敏感词列表
	words := parseSensitiveWordsContent(string(content))
	if len(words) > 0 {
		SensitiveWords = words
		log.Printf("已从文件加载 %d 个敏感词", len(SensitiveWords))
	}
}

// 从文件中加载敏感词响应
func LoadSensitiveResponseFromFile() {
	// 确保配置目录存在
	ensureConfigDir()

	// 如果文件不存在，创建默认文件
	if !fileExists(SensitiveResponseFile) {
		// 创建默认的敏感词响应文件
		err := os.WriteFile(SensitiveResponseFile, []byte(SensitiveFilterResponse), 0644)
		if err != nil {
			log.Printf("创建默认敏感词响应文件失败: %s", err.Error())
			return
		}
		log.Printf("已创建默认敏感词响应文件: %s", SensitiveResponseFile)
	}

	// 读取敏感词响应文件
	content, err := os.ReadFile(SensitiveResponseFile)
	if err != nil {
		log.Printf("读取敏感词响应文件失败: %s", err.Error())
		return
	}

	// 设置敏感词响应
	if len(content) > 0 {
		SensitiveFilterResponse = string(content)
		log.Printf("已从文件加载敏感词响应")
	}
}

// 解析敏感词内容
func parseSensitiveWordsContent(content string) []string {
	var result []string

	// 首先按行分割
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		// 去除空白字符
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 检查是否包含逗号
		if strings.Contains(line, ",") {
			// 按逗号分割
			parts := strings.Split(line, ",")
			for _, part := range parts {
				part = strings.TrimSpace(part)
				if part != "" {
					result = append(result, part)
				}
			}
		} else {
			// 直接添加
			result = append(result, line)
		}
	}

	return result
}

// 保存敏感词列表到文件
func SaveSensitiveWordsToFile(words []string) error {
	// 确保配置目录存在
	ensureConfigDir()

	// 将敏感词列表转换为文本格式（每行一个敏感词）
	content := strings.Join(words, "\n")

	// 写入文件
	err := os.WriteFile(SensitiveWordsFile, []byte(content), 0644)
	if err != nil {
		log.Printf("保存敏感词文件失败: %s", err.Error())
		return err
	}

	log.Printf("已保存敏感词列表到文件: %s", SensitiveWordsFile)
	return nil
}

// 保存敏感词响应到文件
func SaveSensitiveResponseToFile(response string) error {
	// 确保配置目录存在
	ensureConfigDir()

	// 写入文件
	err := os.WriteFile(SensitiveResponseFile, []byte(response), 0644)
	if err != nil {
		log.Printf("保存敏感词响应文件失败: %s", err.Error())
		return err
	}

	log.Printf("已保存敏感词响应到文件: %s", SensitiveResponseFile)
	return nil
}

// 确保配置目录存在
func ensureConfigDir() {
	dir := filepath.Dir(SensitiveWordsFile)
	if !fileExists(dir) {
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			log.Printf("创建配置目录失败: %s", err.Error())
		}
	}
}

// 检查文件或目录是否存在
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}
