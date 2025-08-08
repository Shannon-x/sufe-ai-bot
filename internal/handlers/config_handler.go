package handlers

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/cf-ai-tgbot-go/internal/config"
	dynamicconfig "github.com/cf-ai-tgbot-go/internal/services/config"
	"github.com/cf-ai-tgbot-go/internal/services/storage"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/sirupsen/logrus"
)

// ConfigHandler handles configuration management via Telegram
type ConfigHandler struct {
	bot           *tgbotapi.BotAPI
	configService *dynamicconfig.DynamicConfigService
	storage       *storage.Manager
	logger        *logrus.Logger
}

// NewConfigHandler creates a new config handler
func NewConfigHandler(
	bot *tgbotapi.BotAPI,
	configService *dynamicconfig.DynamicConfigService,
	storage *storage.Manager,
	logger *logrus.Logger,
) *ConfigHandler {
	return &ConfigHandler{
		bot:           bot,
		configService: configService,
		storage:       storage,
		logger:        logger,
	}
}

// HandleConfigCallback handles configuration-related callbacks
func (h *ConfigHandler) HandleConfigCallback(ctx context.Context, callback *tgbotapi.CallbackQuery) error {
	chatID := callback.Message.Chat.ID
	messageID := callback.Message.MessageID
	userID := callback.From.ID
	
	parts := strings.Split(callback.Data, ":")
	if len(parts) < 2 {
		return nil
	}
	
	action := parts[1]
	
	switch action {
	case "add_endpoint":
		return h.showAddEndpointForm(ctx, chatID, messageID, userID, callback.ID)
		
	case "test_endpoint":
		if len(parts) >= 3 {
			return h.testEndpoint(ctx, chatID, messageID, parts[2], callback.ID)
		}
		
	case "delete_endpoint":
		if len(parts) >= 3 {
			return h.confirmDeleteEndpoint(ctx, chatID, messageID, parts[2], callback.ID)
		}
		
	case "confirm_delete":
		if len(parts) >= 3 {
			return h.deleteEndpoint(ctx, chatID, messageID, parts[2], callback.ID)
		}
		
	case "add_model":
		if len(parts) >= 3 {
			return h.showAddModelForm(ctx, chatID, messageID, userID, parts[2], callback.ID)
		}
		
	case "edit_endpoint":
		if len(parts) >= 3 {
			return h.showEditEndpointMenu(ctx, chatID, messageID, parts[2], callback.ID)
		}
	}
	
	return nil
}

// showAddEndpointForm shows a form for adding new endpoint
func (h *ConfigHandler) showAddEndpointForm(ctx context.Context, chatID int64, messageID int, userID int64, callbackID string) error {
	// Create inline keyboard with form fields
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📝 输入端点信息", "noop"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⬅️ 取消", "menu:models"),
		),
	)
	
	text := `🆕 **添加新的AI端点**

请依次发送以下信息（每行一个）：

1️⃣ **端点名称**（英文，如: my-api）
2️⃣ **显示名称**（如: 我的API）
3️⃣ **API地址**（如: https://api.example.com/v1）
4️⃣ **API密钥**（如: sk-xxxxx）

📝 **示例消息：**
` + "```" + `
my-custom-api
我的自定义API
https://api.myservice.com/v1
sk-1234567890abcdef
` + "```" + `

💡 **提示：**
- API地址必须兼容OpenAI格式
- 所有信息请一次性发送
- 发送后将自动测试连接`
	
	// Set user state
	h.storage.SetUserState(ctx, userID, "config_action", "adding_endpoint")
	
	edit := tgbotapi.NewEditMessageText(chatID, messageID, text)
	edit.ParseMode = "Markdown"
	edit.ReplyMarkup = &keyboard
	
	_, err := h.bot.Send(edit)
	h.bot.Request(tgbotapi.NewCallback(callbackID, "请输入端点信息"))
	return err
}

// HandleConfigInput handles user input for configuration
func (h *ConfigHandler) HandleConfigInput(ctx context.Context, message *tgbotapi.Message) error {
	userID := message.From.ID
	chatID := message.Chat.ID
	
	// Get current config action
	action, err := h.storage.GetUserState(ctx, userID, "config_action")
	if err != nil || action == "" {
		return nil
	}
	
	switch action {
	case "adding_endpoint":
		return h.handleAddEndpointInput(ctx, message)
		
	case "adding_model":
		endpointName, _ := h.storage.GetUserState(ctx, userID, "config_endpoint")
		return h.handleAddModelInput(ctx, message, endpointName)
		
	case "editing_url":
		endpointName, _ := h.storage.GetUserState(ctx, userID, "config_endpoint")
		return h.handleEditURLInput(ctx, message, endpointName)
		
	case "editing_key":
		endpointName, _ := h.storage.GetUserState(ctx, userID, "config_endpoint")
		return h.handleEditKeyInput(ctx, message, endpointName)
	}
	
	return nil
}

// handleAddEndpointInput processes endpoint addition input
func (h *ConfigHandler) handleAddEndpointInput(ctx context.Context, message *tgbotapi.Message) error {
	userID := message.From.ID
	chatID := message.Chat.ID
	
	// Parse input
	lines := strings.Split(strings.TrimSpace(message.Text), "\n")
	if len(lines) < 4 {
		msg := tgbotapi.NewMessage(chatID, "❌ 请提供完整的端点信息（4行）")
		h.bot.Send(msg)
		return nil
	}
	
	name := strings.TrimSpace(lines[0])
	displayName := strings.TrimSpace(lines[1])
	baseURL := strings.TrimSpace(lines[2])
	apiKey := strings.TrimSpace(lines[3])
	
	// Validate input
	if err := h.validateEndpointInput(name, displayName, baseURL, apiKey); err != nil {
		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ 输入验证失败：%s", err.Error()))
		h.bot.Send(msg)
		return nil
	}
	
	// Create endpoint
	endpoint := &config.ModelEndpoint{
		Name:        name,
		DisplayName: displayName,
		BaseURL:     baseURL,
		APIKey:      apiKey,
		Models:      []config.Model{},
	}
	
	// Show loading message
	loadingMsg := tgbotapi.NewMessage(chatID, "⏳ 正在添加端点并测试连接...")
	sentMsg, _ := h.bot.Send(loadingMsg)
	
	// Test endpoint
	if err := h.configService.TestEndpoint(ctx, endpoint); err != nil {
		editMsg := tgbotapi.NewEditMessageText(chatID, sentMsg.MessageID, 
			fmt.Sprintf("❌ 连接测试失败：%s\n\n是否仍要添加此端点？", err.Error()))
		
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("✅ 仍然添加", fmt.Sprintf("config:force_add:%s", name)),
				tgbotapi.NewInlineKeyboardButtonData("❌ 取消", "menu:models"),
			),
		)
		editMsg.ReplyMarkup = &keyboard
		h.bot.Send(editMsg)
		
		// Store endpoint data temporarily
		h.storage.SetUserState(ctx, userID, "temp_endpoint", fmt.Sprintf("%s|%s|%s|%s", name, displayName, baseURL, apiKey))
		return nil
	}
	
	// Add endpoint
	if err := h.configService.AddEndpoint(ctx, endpoint); err != nil {
		editMsg := tgbotapi.NewEditMessageText(chatID, sentMsg.MessageID, 
			fmt.Sprintf("❌ 添加失败：%s", err.Error()))
		h.bot.Send(editMsg)
		return nil
	}
	
	// Success message with options
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("➕ 添加模型", fmt.Sprintf("config:add_model:%s", name)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📋 查看所有端点", "menu:models"),
			tgbotapi.NewInlineKeyboardButtonData("🏠 主菜单", "menu:main"),
		),
	)
	
	successMsg := fmt.Sprintf(`✅ **端点添加成功！**

📍 **名称：** %s
🏷 **显示名称：** %s
🌐 **API地址：** %s
✅ **连接状态：** 正常

现在您可以为此端点添加模型。`, name, displayName, baseURL)
	
	editMsg := tgbotapi.NewEditMessageText(chatID, sentMsg.MessageID, successMsg)
	editMsg.ParseMode = "Markdown"
	editMsg.ReplyMarkup = &keyboard
	h.bot.Send(editMsg)
	
	// Clear user state
	h.storage.DeleteUserState(ctx, userID, "config_action")
	
	return nil
}

// showAddModelForm shows form for adding model to endpoint
func (h *ConfigHandler) showAddModelForm(ctx context.Context, chatID int64, messageID int, userID int64, endpointName string, callbackID string) error {
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📝 输入模型信息", "noop"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🤖 使用常见模型", fmt.Sprintf("config:common_models:%s", endpointName)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⬅️ 返回", fmt.Sprintf("config:edit_endpoint:%s", endpointName)),
		),
	)
	
	text := fmt.Sprintf(`🤖 **为端点 [%s] 添加模型**

请发送以下信息（每行一个）：

1️⃣ **模型ID**（如: gpt-3.5-turbo）
2️⃣ **显示名称**（如: GPT-3.5 Turbo）
3️⃣ **最大令牌数**（如: 4096）

📝 **示例：**
`+"```"+`
gpt-4-turbo
GPT-4 Turbo
8192
`+"```"+`

💡 **提示：** 模型ID必须与API提供商的模型名称一致`, endpointName)
	
	// Set user state
	h.storage.SetUserState(ctx, userID, "config_action", "adding_model")
	h.storage.SetUserState(ctx, userID, "config_endpoint", endpointName)
	
	edit := tgbotapi.NewEditMessageText(chatID, messageID, text)
	edit.ParseMode = "Markdown"
	edit.ReplyMarkup = &keyboard
	
	_, err := h.bot.Send(edit)
	h.bot.Request(tgbotapi.NewCallback(callbackID, "请输入模型信息"))
	return err
}

// handleAddModelInput processes model addition input
func (h *ConfigHandler) handleAddModelInput(ctx context.Context, message *tgbotapi.Message, endpointName string) error {
	userID := message.From.ID
	chatID := message.Chat.ID
	
	// Parse input
	lines := strings.Split(strings.TrimSpace(message.Text), "\n")
	if len(lines) < 3 {
		msg := tgbotapi.NewMessage(chatID, "❌ 请提供完整的模型信息（3行）")
		h.bot.Send(msg)
		return nil
	}
	
	modelID := strings.TrimSpace(lines[0])
	modelName := strings.TrimSpace(lines[1])
	maxTokensStr := strings.TrimSpace(lines[2])
	
	maxTokens, err := strconv.Atoi(maxTokensStr)
	if err != nil || maxTokens <= 0 {
		msg := tgbotapi.NewMessage(chatID, "❌ 最大令牌数必须是正整数")
		h.bot.Send(msg)
		return nil
	}
	
	// Create model
	model := config.Model{
		ID:        modelID,
		Name:      modelName,
		MaxTokens: maxTokens,
	}
	
	// Add model to endpoint
	if err := h.configService.AddModelToEndpoint(ctx, endpointName, model); err != nil {
		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ 添加模型失败：%s", err.Error()))
		h.bot.Send(msg)
		return nil
	}
	
	// Success message
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("➕ 继续添加", fmt.Sprintf("config:add_model:%s", endpointName)),
			tgbotapi.NewInlineKeyboardButtonData("✅ 完成", "menu:models"),
		),
	)
	
	successMsg := fmt.Sprintf(`✅ **模型添加成功！**

🤖 **模型ID：** %s
📝 **显示名称：** %s
🔢 **最大令牌：** %d

模型已添加到端点 [%s]`, modelID, modelName, maxTokens, endpointName)
	
	msg := tgbotapi.NewMessage(chatID, successMsg)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard
	h.bot.Send(msg)
	
	// Clear user state
	h.storage.DeleteUserState(ctx, userID, "config_action")
	h.storage.DeleteUserState(ctx, userID, "config_endpoint")
	
	return nil
}

// validateEndpointInput validates endpoint input
func (h *ConfigHandler) validateEndpointInput(name, displayName, baseURL, apiKey string) error {
	// Validate name
	if !regexp.MustCompile(`^[a-zA-Z0-9-_]+$`).MatchString(name) {
		return fmt.Errorf("端点名称只能包含字母、数字、横线和下划线")
	}
	
	// Validate URL
	u, err := url.Parse(baseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("无效的API地址")
	}
	
	// Validate other fields
	if displayName == "" {
		return fmt.Errorf("显示名称不能为空")
	}
	
	if apiKey == "" {
		return fmt.Errorf("API密钥不能为空")
	}
	
	return nil
}

// showCommonModels shows common model presets
func (h *ConfigHandler) showCommonModels(ctx context.Context, chatID int64, messageID int, endpointName string, callbackID string) error {
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🟢 OpenAI GPT系列", fmt.Sprintf("config:preset:openai:%s", endpointName)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔵 Claude系列", fmt.Sprintf("config:preset:claude:%s", endpointName)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🟣 Gemini系列", fmt.Sprintf("config:preset:gemini:%s", endpointName)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🟠 Llama系列", fmt.Sprintf("config:preset:llama:%s", endpointName)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⬅️ 返回", fmt.Sprintf("config:add_model:%s", endpointName)),
		),
	)
	
	text := `🤖 **选择模型系列**

选择您要添加的模型系列，系统将自动添加该系列的常用模型。`
	
	edit := tgbotapi.NewEditMessageText(chatID, messageID, text)
	edit.ParseMode = "Markdown"
	edit.ReplyMarkup = &keyboard
	
	_, err := h.bot.Send(edit)
	h.bot.Request(tgbotapi.NewCallback(callbackID, ""))
	return err
}