package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/king133134/sensfilter"
	"github.com/songquanpeng/one-api/common"
	"github.com/songquanpeng/one-api/common/config"
	"github.com/songquanpeng/one-api/common/logger"
	"github.com/songquanpeng/one-api/relay/adaptor/openai"
	"github.com/songquanpeng/one-api/relay/model"
	"io"
)

var (
	sensitiveFilter *sensfilter.Search
	filterOnce      sync.Once
)

// 获取敏感词过滤器实例
func GetSensitiveFilter() *sensfilter.Search {
	filterOnce.Do(func() {
		config.LoadSensitiveWordsFromFile()
		config.LoadSensitiveResponseFromFile()
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
	commaSplit := strings.Split(words, ",")
	result := make([]string, 0, len(commaSplit))
	for _, part := range commaSplit {
		lines := strings.Split(part, "\n")
		for _, line := range lines {
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
	parsedWords := parseSensitiveWords(words)
	if len(parsedWords) > 0 {
		sensitiveFilter = sensfilter.Strings(parsedWords)
		UpdateEnhancedSensitiveWords(parsedWords)
		config.SaveSensitiveWordsToFile(parsedWords)
	}
}

// 更新敏感词响应
func UpdateSensitiveResponse(response string) {
	config.SensitiveFilterResponse = response
	config.SaveSensitiveResponseToFile(response)
}

// 检测文本是否包含敏感词
func containsSensitiveWords(text string) bool {
	return containsEnhancedSensitiveWords(text)
}

// fullyBufferingResponseWriter captures the response before it's sent.
type fullyBufferingResponseWriter struct {
	gin.ResponseWriter
	buffer               *bytes.Buffer
	statusCode           int
	headerWritten        bool
	headersToSetOnCommit http.Header
}

func newFullyBufferingResponseWriter(originalWriter gin.ResponseWriter) *fullyBufferingResponseWriter {
	return &fullyBufferingResponseWriter{
		ResponseWriter:       originalWriter,
		buffer:               &bytes.Buffer{},
		statusCode:           http.StatusOK,
		headerWritten:        false,
		headersToSetOnCommit: make(http.Header),
	}
}

func (w *fullyBufferingResponseWriter) Header() http.Header {
	return w.headersToSetOnCommit
}

func (w *fullyBufferingResponseWriter) WriteHeader(code int) {
	if !w.headerWritten {
		w.statusCode = code
		w.headerWritten = true
	}
}

func (w *fullyBufferingResponseWriter) Write(data []byte) (int, error) {
	return w.buffer.Write(data)
}

func (w *fullyBufferingResponseWriter) WriteString(s string) (int, error) {
	return w.buffer.WriteString(s)
}

func (w *fullyBufferingResponseWriter) Status() int {
	if w.headerWritten {
		return w.statusCode
	}
	return 0
}

func (w *fullyBufferingResponseWriter) Size() int {
	return w.buffer.Len()
}

func (w *fullyBufferingResponseWriter) Written() bool {
	return w.headerWritten
}

func (w *fullyBufferingResponseWriter) CommitToOriginalWriter() {
	for key, values := range w.headersToSetOnCommit {
		w.ResponseWriter.Header()[key] = values
	}
	if w.headerWritten {
		w.ResponseWriter.WriteHeader(w.statusCode)
	} else {
		if w.buffer.Len() > 0 {
			w.ResponseWriter.WriteHeader(http.StatusOK)
		}
	}
	w.ResponseWriter.Write(w.buffer.Bytes())
}

func (w *fullyBufferingResponseWriter) CloseNotify() <-chan bool {
	if cn, ok := w.ResponseWriter.(http.CloseNotifier); ok {
		return cn.CloseNotify()
	}
	return nil
}

func (w *fullyBufferingResponseWriter) Flush() {
	if fl, ok := w.ResponseWriter.(http.Flusher); ok {
		fl.Flush()
	}
}

func (w *fullyBufferingResponseWriter) Pusher() http.Pusher {
	if pusher, ok := w.ResponseWriter.(http.Pusher); ok {
		return pusher
	}
	return nil
}

// createChatCompletionErrorResponse modifies c.Writer (assumed to be *fullyBufferingResponseWriter)
// to buffer a structured error JSON response that mimics a chat completion object.
func createChatCompletionErrorResponse(c *gin.Context, statusCode int, errorCode string, message string, errType string, modelName string) {
	errorPayload := gin.H{
		"id":      "chatcmpl-filter-" + strings.ReplaceAll(uuid.NewString(), "-", "")[:20],
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   modelName,
		"choices": []gin.H{
			{
				"index": 0,
				"message": gin.H{
					"role":    "assistant",
					"content": message,
				},
				"finish_reason": "content_filter",
			},
		},
		"usage": gin.H{
			"prompt_tokens":     0,
			"completion_tokens": 0,
			"total_tokens":      0,
		},
		"error": gin.H{
			"message": message,
			"type":    errType,
			"code":    errorCode,
		},
	}

	if bw, ok := c.Writer.(*fullyBufferingResponseWriter); ok {
		bw.buffer.Reset()
		bw.headerWritten = false
		bw.headersToSetOnCommit = make(http.Header)
	}

	c.JSON(statusCode, errorPayload)
	c.Abort()
}

// SensitiveFilter 敏感词过滤中间件
func SensitiveFilter() gin.HandlerFunc {
	GetSensitiveFilter()
	GetEnhancedSensitiveFilter()

	return func(c *gin.Context) {
		if !config.SensitiveFilterEnabled {
			c.Next()
			return
		}

		path := c.Request.URL.Path
		if !strings.HasPrefix(path, "/v1/chat/completions") &&
			!strings.HasPrefix(path, "/v1/completions") {
			c.Next()
			return
		}

		requestBodyBytes, err := common.GetRequestBody(c)
		if err != nil {
			logger.Errorf(c.Request.Context(), "读取请求体失败: %v", err)
			c.Next()
			return
		}

		var request model.GeneralOpenAIRequest
		err = json.Unmarshal(requestBodyBytes, &request)
		if err != nil {
			logger.Errorf(c.Request.Context(), "解析请求体JSON失败: %v", err)
			c.Next()
			return
		}

		// 请求内容敏感词检查 (对所有请求都执行)
		for _, message := range request.Messages {
			content := message.StringContent()
			if containsSensitiveWords(content) {
				logger.Warnf(c.Request.Context(), "请求中检测到敏感词，请求被拦截")
				errorPayload := gin.H{
					"id":      "chatcmpl-req-filter-" + strings.ReplaceAll(uuid.NewString(), "-", "")[:20],
					"object":  "chat.completion",
					"created": time.Now().Unix(),
					"model":   request.Model,
					"choices": []gin.H{
						{
							"index": 0,
							"message": gin.H{
								"role":    "assistant",
								"content": config.SensitiveFilterResponse,
							},
							"finish_reason": "content_filter",
						},
					},
					"usage": gin.H{"prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0},
					"error": gin.H{
						"message": config.SensitiveFilterResponse,
						"type":    "invalid_request_error",
						"code":    "content_filter_request",
					},
				}
				c.AbortWithStatusJSON(http.StatusBadRequest, errorPayload)
				return
			}
		}

		c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBodyBytes)) // Restore request body for c.Next()

		if !request.Stream { // 非流式响应处理
			originalWriter := c.Writer
			bufferingWriter := newFullyBufferingResponseWriter(originalWriter)
			c.Writer = bufferingWriter

			c.Next()

			// 仅在未被下游处理程序中止（例如 createChatCompletionErrorResponse 未调用 c.Abort()）时检查响应
			if !c.IsAborted() {
				statusFromHandler := bufferingWriter.Status()
				if !bufferingWriter.headerWritten && bufferingWriter.buffer.Len() > 0 {
					statusFromHandler = http.StatusOK
				}

				if statusFromHandler == http.StatusOK {
					responseBodyStr := bufferingWriter.buffer.String()
					// 对于非流式，isStreamResponse 应该总是 false，但为了保险可以检查
					// isStreamResponse := strings.Contains(responseBodyStr, "data: ")

					sensitiveFoundInResponse := false
					// 处理非流式响应
					var resp openai.TextResponse
					if err := json.Unmarshal([]byte(responseBodyStr), &resp); err == nil {
						for _, choice := range resp.Choices {
							contentText := choice.Message.StringContent()
							if contentText != "" && containsSensitiveWords(contentText) {
								sensitiveFoundInResponse = true
								break
							}
						}
					} else {
						logger.Warnf(c.Request.Context(), "无法解析非流式响应以检查敏感词: %v; 响应体: %s", err, responseBodyStr)
					}

					if sensitiveFoundInResponse {
						logger.Warnf(c.Request.Context(), "响应中检测到敏感词，响应被拦截")
						createChatCompletionErrorResponse(c, http.StatusBadRequest,
							"content_filter_response",
							config.SensitiveFilterResponse,
							"invalid_request_error",
							request.Model)
					}
				}
			} // end if !c.IsAborted()

			bufferingWriter.CommitToOriginalWriter()
			c.Writer = originalWriter // Restore original writer

		} else { // 流式响应处理
			logger.Infof(c.Request.Context(), "处理流式响应，跳过响应体敏感词过滤和缓冲。")
			c.Next() // 直接让下游处理，不使用缓冲写入器
			// 对于流式响应，我们无法安全地在发送后修改或检查内容，也无法可靠获取最终头部
			// 除非我们实现一个完整的流代理，这超出了当前范围。
			// 这里的日志是针对请求时的，流的头部由下游直接写给客户端。
			logger.Infof(c.Request.Context(), "流式响应已由下游处理程序直接发送。")
		}
	}
}
