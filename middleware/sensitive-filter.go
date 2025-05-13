package middleware

import (
	"bytes"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/king133134/sensfilter"
	"github.com/songquanpeng/one-api/common"
	"github.com/songquanpeng/one-api/common/config"
	"github.com/songquanpeng/one-api/common/logger"
	"github.com/songquanpeng/one-api/relay/adaptor/openai"
	"github.com/songquanpeng/one-api/relay/model"
	"io"
	"net/http"
	"strings"
	"sync"
)

var (
	sensitiveFilter *sensfilter.Search
	filterOnce      sync.Once
)

// 获取敏感词过滤器实例
func GetSensitiveFilter() *sensfilter.Search {
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
		// 更新增强的过滤器
		UpdateEnhancedSensitiveWords(parsedWords)
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
	// 使用增强的敏感词检测功能
	return containsEnhancedSensitiveWords(text)
}

// fullyBufferingResponseWriter captures the response before it's sent.
type fullyBufferingResponseWriter struct {
	gin.ResponseWriter
	buffer               *bytes.Buffer
	statusCode           int
	headerWritten        bool
	headersToSetOnCommit http.Header // Store headers that c.Header() tries to set
}

func newFullyBufferingResponseWriter(originalWriter gin.ResponseWriter) *fullyBufferingResponseWriter {
	return &fullyBufferingResponseWriter{
		ResponseWriter:       originalWriter,
		buffer:               &bytes.Buffer{},
		statusCode:           http.StatusOK, // Default status code
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
	if !w.headerWritten {
		// Typically, Gin handlers or renderers call WriteHeader before Write.
		// If not, this would be a place to default status, but Gin's flow usually ensures it.
	}
	return w.buffer.Write(data)
}

func (w *fullyBufferingResponseWriter) WriteString(s string) (int, error) {
	if !w.headerWritten {
		// Similar to Write
	}
	return w.buffer.WriteString(s)
}

func (w *fullyBufferingResponseWriter) Status() int {
	if w.headerWritten {
		return w.statusCode
	}
	// If WriteHeader was never called on this buffering writer,
	// return 0 to indicate that status is not yet determined by this writer.
	// The underlying ResponseWriter's status might be different if accessed directly,
	// but this Status() should reflect the state of the buffer.
	return 0
}

func (w *fullyBufferingResponseWriter) Size() int {
	return w.buffer.Len()
}

func (w *fullyBufferingResponseWriter) Written() bool {
	return w.headerWritten
}

// CommitToOriginalWriter sends the buffered response to the actual ResponseWriter.
func (w *fullyBufferingResponseWriter) CommitToOriginalWriter() {
	// Copy headers collected in w.headersToSetOnCommit to the actual ResponseWriter's headers
	// Do this before writing status, as WriteHeader might be the point where headers are fixed.
	for key, values := range w.headersToSetOnCommit {
		// Ensure we don't add to existing headers if that's not desired,
		// but Gin's ResponseWriter.Header() typically returns a map that can be set directly.
		// For safety, let's clear and set, or just set if underlying is a map.
		// gin.ResponseWriter.Header() returns a http.Header (map[string][]string)
		// So setting it directly is usually fine.
		w.ResponseWriter.Header()[key] = values
	}

	if w.headerWritten {
		w.ResponseWriter.WriteHeader(w.statusCode)
	} else {
		// If WriteHeader was not called on our buffering writer, but there's data,
		// default to http.StatusOK for the underlying writer.
		// This handles cases where a handler might directly Write() without WriteHeader().
		if w.buffer.Len() > 0 {
			// Check if underlying writer already sent headers (e.g., status is non-zero)
			// This check is complex as w.ResponseWriter.Status() is from gin.ResponseWriter,
			// which might have its own buffering logic.
			// For now, assume if we have data and our WriteHeader wasn't called, we set default.
			w.ResponseWriter.WriteHeader(http.StatusOK)
		}
	}
	w.ResponseWriter.Write(w.buffer.Bytes())
}

// Pass-through methods for interfaces that gin.ResponseWriter might implement
func (w *fullyBufferingResponseWriter) CloseNotify() <-chan bool {
	if cn, ok := w.ResponseWriter.(http.CloseNotifier); ok {
		return cn.CloseNotify()
	}
	return nil
}

func (w *fullyBufferingResponseWriter) Flush() { // This is for http.Flusher
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

// createErrorJSONResponse modifies c.Writer (assumed to be *fullyBufferingResponseWriter)
// to buffer an error JSON response.
func createErrorJSONResponse(c *gin.Context, statusCode int, errorCode string, message string, errType string) {
	errorResponseData := map[string]interface{}{
		"error": map[string]interface{}{
			"message": message,
			"type":    errType,
			"code":    errorCode,
		},
	}

	if bw, ok := c.Writer.(*fullyBufferingResponseWriter); ok {
		bw.buffer.Reset()
		bw.headerWritten = false                    // Allow c.JSON to set the new status via WriteHeader
		bw.headersToSetOnCommit = make(http.Header) // Clear any previously collected headers
		// statusCode will be set by c.JSON calling bw.WriteHeader
	}

	// c.JSON will use c.Writer (our fullyBufferingResponseWriter)
	// It will set "Content-Type" via c.Writer.Header().Set()
	// and then call c.Writer.WriteHeader() and c.Writer.Write()
	c.JSON(statusCode, errorResponseData)
	c.Abort() // Crucial to stop further processing by other handlers in Gin
}

// SensitiveFilter 敏感词过滤中间件
func SensitiveFilter() gin.HandlerFunc {
	// 确保敏感词过滤器已初始化
	GetSensitiveFilter()
	// 确保增强的敏感词过滤器已初始化
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

		// 读取请求体
		requestBodyBytes, err := common.GetRequestBody(c)
		if err != nil {
			logger.Errorf(c.Request.Context(), "读取请求体失败: %v", err)
			c.Next() // Or return an error immediately
			return
		}

		// 解析请求
		var request model.GeneralOpenAIRequest
		err = json.Unmarshal(requestBodyBytes, &request)
		if err != nil {
			logger.Errorf(c.Request.Context(), "解析请求体JSON失败: %v", err)
			c.Next() // Or return an error immediately
			return
		}

		// 检查请求中的消息是否包含敏感词
		for _, message := range request.Messages {
			content := message.StringContent()
			if containsSensitiveWords(content) {
				logger.Warnf(c.Request.Context(), "请求中检测到敏感词，请求被拦截")
				// c.Writer is the original writer here.
				// AbortWithStatusJSON directly writes to the original writer and aborts.
				errorData := map[string]interface{}{
					"error": map[string]interface{}{
						"message": config.SensitiveFilterResponse,
						"type":    "invalid_request_error",
						"code":    "content_filter_request",
					},
				}
				c.AbortWithStatusJSON(http.StatusBadRequest, errorData)
				return
			}
		}

		// 重新设置请求体
		c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBodyBytes))

		// 设置响应拦截
		originalWriter := c.Writer
		bufferingWriter := newFullyBufferingResponseWriter(originalWriter)
		c.Writer = bufferingWriter

		c.Next() // Subsequent handlers write to bufferingWriter

		// 检查响应体是否包含敏感词 (after c.Next() has returned)
		// bufferingWriter.Status() gives the status code set by the handler, or 0 if not set.
		// bufferingWriter.headerWritten indicates if WriteHeader was called on our buffer.

		statusFromHandler := bufferingWriter.Status()
		if !bufferingWriter.headerWritten && bufferingWriter.buffer.Len() > 0 {
			// If data was written to buffer but no explicit status, assume it would be 200
			statusFromHandler = http.StatusOK
		}

		if statusFromHandler == http.StatusOK {
			responseBodyStr := bufferingWriter.buffer.String()
			isStreamResponse := strings.Contains(responseBodyStr, "data: ")

			sensitiveFoundInResponse := false
			if isStreamResponse {
				lines := strings.Split(responseBodyStr, "\n")
				for _, line := range lines {
					if strings.HasPrefix(line, "data: ") && line != "data: [DONE]" {
						data := strings.TrimPrefix(line, "data: ")
						var streamResp openai.ChatCompletionsStreamResponse
						if err := json.Unmarshal([]byte(data), &streamResp); err == nil {
							for _, choice := range streamResp.Choices {
								content, ok := choice.Delta.Content.(string)
								if ok && containsSensitiveWords(content) {
									sensitiveFoundInResponse = true
									break
								}
							}
						}
					}
					if sensitiveFoundInResponse {
						break
					}
				}
			} else {
				var resp openai.TextResponse
				if err := json.Unmarshal([]byte(responseBodyStr), &resp); err == nil {
					for _, choice := range resp.Choices {
						// choice.Message is type model.Message
						contentText := choice.Message.StringContent()
						if contentText != "" && containsSensitiveWords(contentText) {
							sensitiveFoundInResponse = true
							break
						}
					}
				} else {
					// Log if unmarshalling fails, but treat as non-sensitive for this check
					logger.Warnf(c.Request.Context(), "无法解析非流式响应以检查敏感词: %v; 响应体: %s", err, responseBodyStr)
				}
			}

			if sensitiveFoundInResponse {
				logger.Warnf(c.Request.Context(), "响应中检测到敏感词，响应被拦截")
				// This will modify bufferingWriter to contain the error response.
				createErrorJSONResponse(c, http.StatusBadRequest, "content_filter", config.SensitiveFilterResponse, "invalid_request_error")
				// c.Abort() is called within createErrorJSONResponse.
			}
		}
		// If no sensitive words found in response, or if status was not OK,
		// bufferingWriter contains what the downstream handlers put in it.

		bufferingWriter.CommitToOriginalWriter()
		c.Writer = originalWriter // Restore original writer
	}
}
