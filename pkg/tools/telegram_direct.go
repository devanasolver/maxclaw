package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// TelegramDirectTool Telegram 直接文件发送工具
// 这个工具直接调用 Telegram API，不通过 channel 架构
type TelegramDirectTool struct {
	BaseTool
	mu       sync.RWMutex
	token    string
	channel  string
	chatID   string
	proxy    string
}

// NewTelegramDirectTool 创建 Telegram 直接文件发送工具
func NewTelegramDirectTool(token, proxy string) *TelegramDirectTool {
	return &TelegramDirectTool{
		BaseTool: BaseTool{
			name:        "telegram_send_file",
			description: "Send a photo or document file directly to Telegram using the bot token. Use to send images, documents, or other files through Telegram bot.",
			parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"file_path": map[string]interface{}{
						"type":        "string",
						"description": "Path to the file to send",
						"minLength":   1,
					},
					"file_type": map[string]interface{}{
						"type":        "string",
						"description": "Type of file: 'photo' for images, 'document' for other files",
						"enum":        []string{"photo", "document"},
					},
					"caption": map[string]interface{}{
						"type":        "string",
						"description": "Optional caption for the file",
					},
					"chat_id": map[string]interface{}{
						"type":        "string",
						"description": "Chat ID to send to (required if not in context)",
					},
				},
				"required": []string{"file_path", "file_type"},
			},
		},
		token: token,
		proxy: proxy,
	}
}

// SetContext 设置当前上下文
func (t *TelegramDirectTool) SetContext(channel, chatID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.channel = channel
	t.chatID = chatID
}

// Execute 执行文件发送
func (t *TelegramDirectTool) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
	// 获取文件路径
	filePath, ok := params["file_path"].(string)
	if !ok || filePath == "" {
		return "", fmt.Errorf("file_path is required")
	}

	// 获取文件类型
	fileType, ok := params["file_type"].(string)
	if !ok || fileType == "" {
		return "", fmt.Errorf("file_type is required")
	}

	// 验证文件类型
	if fileType != "photo" && fileType != "document" {
		return "", fmt.Errorf("file_type must be 'photo' or 'document'")
	}

	// 获取可选的 caption
	caption, _ := params["caption"].(string)

	// 获取 chat_id
	t.mu.RLock()
	chatID := t.chatID
	t.mu.RUnlock()

	// 优先使用当前请求上下文
	if _, ctxChatID := RuntimeContextFrom(ctx); ctxChatID != "" {
		if chatID == "" {
			chatID = ctxChatID
		}
	}

	// 允许通过参数覆盖
	if v, ok := params["chat_id"].(string); ok && v != "" {
		chatID = v
	}

	// 验证必要参数
	if chatID == "" {
		return "", fmt.Errorf("chat_id must be set (either in context or as parameter)")
	}

	// 验证 token
	if t.token == "" {
		return "", fmt.Errorf("telegram bot token is not configured")
	}

	// 验证文件存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return "", fmt.Errorf("file does not exist: %s", filePath)
	}

	// 验证文件路径安全
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	// 检查是否在允许的目录内
	if allowedDir := GetAllowedDir(); allowedDir != "" {
		allowedAbs, err := filepath.Abs(allowedDir)
		if err != nil {
			return "", fmt.Errorf("failed to get allowed directory absolute path: %w", err)
		}
		
		if !strings.HasPrefix(absPath, allowedAbs) {
			return "", fmt.Errorf("file path %s is outside allowed directory %s", absPath, allowedAbs)
		}
	}

	// 确定 API 方法和字段名
	apiMethod := "sendPhoto"
	fileField := "photo"
	if fileType == "document" {
		apiMethod = "sendDocument"
		fileField = "document"
	}

	// 发送文件
	if err := t.sendFile(chatID, absPath, apiMethod, fileField, caption); err != nil {
		return "", fmt.Errorf("failed to send file: %w", err)
	}

	return fmt.Sprintf("File sent successfully to Telegram: %s (type: %s)", filepath.Base(filePath), fileType), nil
}

// sendFile 发送文件到 Telegram
func (t *TelegramDirectTool) sendFile(chatID, filePath, apiMethod, fileField, caption string) error {
	// 打开文件
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	// 创建 HTTP client
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// 设置代理（如果有）
	if t.proxy != "" {
		proxyURL, err := url.Parse(t.proxy)
		if err == nil {
			client.Transport = &http.Transport{
				Proxy: http.ProxyURL(proxyURL),
			}
		}
	}

	// 创建 multipart 请求体
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// 添加 chat_id
	if err := writer.WriteField("chat_id", chatID); err != nil {
		return fmt.Errorf("failed to write chat_id field: %w", err)
	}

	// 添加 caption（如果有）
	if caption != "" {
		if err := writer.WriteField("caption", caption); err != nil {
			return fmt.Errorf("failed to write caption field: %w", err)
		}
	}

	// 添加文件
	part, err := writer.CreateFormFile(fileField, filepath.Base(filePath))
	if err != nil {
		return fmt.Errorf("failed to create form file: %w", err)
	}

	// 复制文件内容
	if _, err := io.Copy(part, file); err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	// 关闭 writer
	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// 发送请求
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/%s", t.token, apiMethod)
	req, err := http.NewRequest("POST", apiURL, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	// 解析响应验证
	var result struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if !result.OK {
		return fmt.Errorf("telegram API returned not ok: %s", string(respBody))
	}

	return nil
}