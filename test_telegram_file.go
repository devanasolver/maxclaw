package main

import (
	"fmt"
	"log"
	"os"

	"github.com/Lichas/maxclaw/internal/channels"
)

func main() {
	// 从环境变量获取配置
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatID := os.Getenv("TELEGRAM_CHAT_ID")
	filePath := os.Getenv("FILE_PATH")
	
	if token == "" || chatID == "" || filePath == "" {
		log.Fatal("请设置环境变量: TELEGRAM_BOT_TOKEN, TELEGRAM_CHAT_ID, FILE_PATH")
	}

	// 创建 Telegram channel
	config := &channels.TelegramConfig{
		Token:   token,
		Enabled: true,
	}
	
	channel := channels.NewTelegramChannel(config)
	
	// 测试发送图片
	fmt.Printf("测试发送文件: %s\n", filePath)
	
	// 检查文件类型
	ext := getFileExtension(filePath)
	fileType := "document"
	
	// 常见图片扩展名
	imageExts := map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true,
		".gif": true, ".bmp": true, ".webp": true,
	}
	
	if imageExts[ext] {
		fileType = "photo"
		fmt.Printf("检测为图片文件，使用 sendPhoto\n")
		err := channel.SendPhoto(chatID, filePath, "测试图片发送")
		if err != nil {
			log.Fatalf("发送图片失败: %v", err)
		}
		fmt.Println("图片发送成功!")
	} else {
		fmt.Printf("检测为文档文件，使用 sendDocument\n")
		err := channel.SendDocument(chatID, filePath, "测试文档发送")
		if err != nil {
			log.Fatalf("发送文档失败: %v", err)
		}
		fmt.Println("文档发送成功!")
	}
}

func getFileExtension(path string) string {
	for i := len(path) - 1; i >= 0 && path[i] != '/'; i-- {
		if path[i] == '.' {
			return path[i:]
		}
	}
	return ""
}