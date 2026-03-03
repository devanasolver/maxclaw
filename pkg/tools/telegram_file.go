package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// TelegramFileCallback Telegram 文件发送回调函数类型
type TelegramFileCallback func(channel, chatID, filePath, fileType, caption string) error

// TelegramFileTool Telegram 文件发送工具
type TelegramFileTool struct {
	BaseTool
	callback TelegramFileCallback
	mu       sync.RWMutex
	channel  string
	chatID   string
}

// NewTelegramFileTool 创建 Telegram 文件发送工具
func NewTelegramFileTool(callback TelegramFileCallback) *TelegramFileTool {
	return &TelegramFileTool{
		BaseTool: BaseTool{
			name:        "telegram_file",
			description: "Send a photo or document file to Telegram. Use to send images, documents, or other files through Telegram bot.",
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
					"channel": map[string]interface{}{
						"type":        "string",
						"description": "Channel to send to (optional, uses current if not specified)",
					},
					"chat_id": map[string]interface{}{
						"type":        "string",
						"description": "Chat ID to send to (optional, uses current if not specified)",
					},
				},
				"required": []string{"file_path", "file_type"},
			},
		},
		callback: callback,
	}
}

// SetContext 设置当前上下文
func (t *TelegramFileTool) SetContext(channel, chatID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.channel = channel
	t.chatID = chatID
}

// Execute 执行文件发送
func (t *TelegramFileTool) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
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

	// 获取当前上下文
	t.mu.RLock()
	channel := t.channel
	chatID := t.chatID
	t.mu.RUnlock()

	// 优先使用当前请求上下文，避免并发请求时的上下文串线
	if ctxChannel, ctxChatID := RuntimeContextFrom(ctx); ctxChannel != "" || ctxChatID != "" {
		if channel == "" {
			channel = ctxChannel
		}
		if chatID == "" {
			chatID = ctxChatID
		}
	}

	// 允许通过参数覆盖
	if v, ok := params["channel"].(string); ok && v != "" {
		channel = v
	}
	if v, ok := params["chat_id"].(string); ok && v != "" {
		chatID = v
	}

	// 验证必要参数
	if channel == "" || chatID == "" {
		return "", fmt.Errorf("channel and chat_id must be set")
	}

	// 验证文件存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return "", fmt.Errorf("file does not exist: %s", filePath)
	}

	// 验证文件路径安全（防止目录遍历）
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	// 检查是否在允许的目录内（如果设置了限制）
	if allowedDir := GetAllowedDir(); allowedDir != "" {
		allowedAbs, err := filepath.Abs(allowedDir)
		if err != nil {
			return "", fmt.Errorf("failed to get allowed directory absolute path: %w", err)
		}
		
		if !strings.HasPrefix(absPath, allowedAbs) {
			return "", fmt.Errorf("file path %s is outside allowed directory %s", absPath, allowedAbs)
		}
	}

	// 调用回调函数发送文件
	if t.callback != nil {
		if err := t.callback(channel, chatID, absPath, fileType, caption); err != nil {
			return "", fmt.Errorf("failed to send file: %w", err)
		}
	}

	return fmt.Sprintf("File sent successfully: %s (type: %s)", filepath.Base(filePath), fileType), nil
}