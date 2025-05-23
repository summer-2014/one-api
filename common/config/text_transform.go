package config

import (
	"strings"
)

// 全角字符到半角字符的映射
var fullWidthToHalfWidth = map[rune]rune{
	'！': '!', '＂': '"', '＃': '#', '＄': '$', '％': '%', '＆': '&', '＇': '\'', '（': '(', '）': ')', '＊': '*',
	'＋': '+', '，': ',', '－': '-', '．': '.', '／': '/', '０': '0', '１': '1', '２': '2', '３': '3', '４': '4',
	'５': '5', '６': '6', '７': '7', '８': '8', '９': '9', '：': ':', '；': ';', '＜': '<', '＝': '=', '＞': '>',
	'？': '?', '＠': '@', 'Ａ': 'A', 'Ｂ': 'B', 'Ｃ': 'C', 'Ｄ': 'D', 'Ｅ': 'E', 'Ｆ': 'F', 'Ｇ': 'G', 'Ｈ': 'H',
	'Ｉ': 'I', 'Ｊ': 'J', 'Ｋ': 'K', 'Ｌ': 'L', 'Ｍ': 'M', 'Ｎ': 'N', 'Ｏ': 'O', 'Ｐ': 'P', 'Ｑ': 'Q', 'Ｒ': 'R',
	'Ｓ': 'S', 'Ｔ': 'T', 'Ｕ': 'U', 'Ｖ': 'V', 'Ｗ': 'W', 'Ｘ': 'X', 'Ｙ': 'Y', 'Ｚ': 'Z', '［': '[', '＼': '\\',
	'］': ']', '＾': '^', '＿': '_', '｀': '`', 'ａ': 'a', 'ｂ': 'b', 'ｃ': 'c', 'ｄ': 'd', 'ｅ': 'e', 'ｆ': 'f',
	'ｇ': 'g', 'ｈ': 'h', 'ｉ': 'i', 'ｊ': 'j', 'ｋ': 'k', 'ｌ': 'l', 'ｍ': 'm', 'ｎ': 'n', 'ｏ': 'o', 'ｐ': 'p',
	'ｑ': 'q', 'ｒ': 'r', 'ｓ': 's', 'ｔ': 't', 'ｕ': 'u', 'ｖ': 'v', 'ｗ': 'w', 'ｘ': 'x', 'ｙ': 'y', 'ｚ': 'z',
	'｛': '{', '｜': '|', '｝': '}', '～': '~', '　': ' ',
}

// 将全角字符转换为半角字符
func FullWidthToHalfWidth(text string) string {
	result := []rune(text)
	for i, char := range result {
		if halfWidth, ok := fullWidthToHalfWidth[char]; ok {
			result[i] = halfWidth
		}
	}
	return string(result)
}

// 统一处理文本，使其对大小写不敏感且处理全角半角字符
func NormalizeText(text string) string {
	// 转换为小写
	text = strings.ToLower(text)
	// 全角转半角
	text = FullWidthToHalfWidth(text)
	return text
}
