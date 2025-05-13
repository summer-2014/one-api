package middleware

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/king133134/sensfilter"
	"github.com/songquanpeng/one-api/common"
	"github.com/songquanpeng/one-api/common/config"
	"github.com/songquanpeng/one-api/common/logger"
	"github.com/songquanpeng/one-api/relay/adaptor/openai"
	"github.com/songquanpeng/one-api/relay/model"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
)

var (
	sensitiveFilter *sensfilter.Search
	filterOnce      sync.Once
)

// 获取敏感词过滤器实例
func getSensitiveFilter() *sensfilter.Search {
	filterOnce.Do(func() {
		// 从配置文件加载敏感词
		config.LoadSensitiveWordsFromFile()
		config.LoadSensitiveResponseFromFile()

		// 添加敏感词
		if len(config.SensitiveWords) > 0 {
			sensitiveFilter = sensfilter.Strings(config.SensitiveWords)
		} else {
			sensitiveFilter = sensfilter.NewSearch()
		}
	})
	return sensitiveFilter
}

// 解析敏感词列表，支持每行一个敏感词或逗号分隔的格式
func parseSensitiveWords(words string) []string {
	// 首先按逗号分隔
	commaSplit := strings.Split(words, ",")
	result := make([]string, 0, len(commaSplit))

	// 然后处理每个部分，检查是否包含换行符
	for _, part := range commaSplit {
		// 按换行符分隔
		lines := strings.Split(part, "\n")
		for _, line := range lines {
			// 去除空白字符
			word := strings.TrimSpace(line)
			if word != "" {
				result = append(result, word)
			}
		}
	}

	return result
}

// 更新敏感词列表
func UpdateSensitiveWords(words string) {
	// 解析敏感词
	parsedWords := parseSensitiveWords(words)
	if len(parsedWords) > 0 {
		// 创建新的过滤器
		sensitiveFilter = sensfilter.Strings(parsedWords)
		// 保存到文件
		config.SaveSensitiveWordsToFile(parsedWords)
	}
}

// 更新敏感词响应
func UpdateSensitiveResponse(response string) {
	config.SensitiveFilterResponse = response
	// 保存到文件
	config.SaveSensitiveResponseToFile(response)
}

// 检测文本是否包含敏感词
func containsSensitiveWords(text string) bool {
	if !config.SensitiveFilterEnabled || text == "" {
		return false
	}

	filter := getSensitiveFilter()
	return filter.HasSens([]byte(text))
}

// 为OpenAI格式的请求创建敏感词响应
func createSensitiveResponse(c *gin.Context, isStream bool) {
	// 创建一个简单的错误响应
	errorResponse := map[string]interface{}{
		"error": map[string]interface{}{
			"message": config.SensitiveFilterResponse,
			"type":    "invalid_request_error",
			"code":    "content_filter",
		},
	}

	c.Header("Content-Type", "application/json")
	c.JSON(http.StatusBadRequest, errorResponse)
	c.Abort()
}

// SensitiveFilter 敏感词过滤中间件
func SensitiveFilter() gin.HandlerFunc {
	// 确保敏感词过滤器已初始化
	getSensitiveFilter()

	return func(c *gin.Context) {
		if !config.SensitiveFilterEnabled {
			c.Next()
			return
		}

		// 只处理特定的请求路径
		path := c.Request.URL.Path
		if !strings.HasPrefix(path, "/v1/chat/completions") &&
			!strings.HasPrefix(path, "/v1/completions") {
			c.Next()
			return
		}

		// 读取请求体
		requestBody, err := common.GetRequestBody(c)
		if err != nil {
			logger.Errorf(c.Request.Context(), "读取请求体失败: %v", err)
			c.Next()
			return
		}

		// 解析请求
		var request model.GeneralOpenAIRequest
		err = json.Unmarshal(requestBody, &request)
		if err != nil {
			logger.Errorf(c.Request.Context(), "解析请求体失败: %v", err)
			c.Next()
			return
		}

		// 检查请求中的消息是否包含敏感词
		for _, message := range request.Messages {
			content := message.StringContent()
			if containsSensitiveWords(content) {
				logger.Warnf(c.Request.Context(), "检测到敏感词，请求被拦截")
				createSensitiveResponse(c, request.Stream)
				return
			}
		}

		// 重新设置请求体
		c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))

		// 设置响应拦截
		bodyWriter := &responseBodyWriter{body: bytes.NewBufferString(""), ResponseWriter: c.Writer}
		c.Writer = bodyWriter

		c.Next()

		// 检查响应体是否包含敏感词
		if c.Writer.Status() == http.StatusOK {
			responseBody := bodyWriter.body.String()

			// 判断是否为流式响应
			isStreamResponse := strings.Contains(responseBody, "data: ")

			if isStreamResponse {
				// 处理流式响应
				lines := strings.Split(responseBody, "\n")
				for _, line := range lines {
					if strings.HasPrefix(line, "data: ") && line != "data: [DONE]" {
						data := strings.TrimPrefix(line, "data: ")
						var streamResp openai.ChatCompletionsStreamResponse
						if err := json.Unmarshal([]byte(data), &streamResp); err == nil {
							for _, choice := range streamResp.Choices {
								content, ok := choice.Delta.Content.(string)
								if ok && containsSensitiveWords(content) {
									logger.Warnf(c.Request.Context(), "检测到敏感词，响应被拦截")
									return
								}
							}
						}
					}
				}
			} else {
				// 处理非流式响应
				var resp openai.TextResponse
				if err := json.Unmarshal([]byte(responseBody), &resp); err == nil {
					for _, choice := range resp.Choices {
						var content string
						if contentStr, ok := choice.Content.(string); ok {
							content = contentStr
						} else if choice.Message.Content != "" {
							// 需要进行类型断言
							if messageContent, ok := choice.Message.Content.(string); ok {
								content = messageContent
							}
						}

						if content != "" && containsSensitiveWords(content) {
							logger.Warnf(c.Request.Context(), "检测到敏感词，响应被拦截")
							createSensitiveResponse(c, false)
							return
						}
					}
				}
			}
		}
	}
}

// responseBodyWriter 用于捕获响应体
type responseBodyWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (r *responseBodyWriter) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

// 确保实现所有必要的接口
func (r *responseBodyWriter) WriteString(s string) (int, error) {
	r.body.WriteString(s)
	return r.ResponseWriter.WriteString(s)
}

func (r *responseBodyWriter) WriteHeader(statusCode int) {
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *responseBodyWriter) Status() int {
	return r.ResponseWriter.Status()
}

func (r *responseBodyWriter) Size() int {
	return r.ResponseWriter.Size()
}

func (r *responseBodyWriter) Written() bool {
	return r.ResponseWriter.Written()
}

func (r *responseBodyWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := r.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, errors.New("responseBodyWriter: underlying ResponseWriter does not support Hijack")
}

func (r *responseBodyWriter) CloseNotify() <-chan bool {
	if cn, ok := r.ResponseWriter.(http.CloseNotifier); ok {
		return cn.CloseNotify()
	}
	return nil
}

func (r *responseBodyWriter) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (r *responseBodyWriter) Pusher() http.Pusher {
	if p, ok := r.ResponseWriter.(http.Pusher); ok {
		return p
	}
	return nil
}
