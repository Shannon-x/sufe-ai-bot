package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/cf-ai-tgbot-go/internal/config"
	"github.com/cf-ai-tgbot-go/internal/i18n"
	"github.com/cf-ai-tgbot-go/internal/models"
	"github.com/cf-ai-tgbot-go/internal/services/ai"
	"github.com/cf-ai-tgbot-go/internal/services/cache"
	"github.com/cf-ai-tgbot-go/internal/services/knowledge"
	"github.com/cf-ai-tgbot-go/internal/services/storage"
	"github.com/cf-ai-tgbot-go/internal/middleware"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/sirupsen/logrus"
)

// CommandHandler handles telegram commands
type CommandHandler struct {
	bot              *tgbotapi.BotAPI
	config           *config.Config
	aiService        ai.Service
	knowledgeService knowledge.Service
	storage          *storage.Manager
	cache            cache.Service
	rateLimiter      middleware.RateLimiter
	localizer        *i18n.Localizer
	logger           *logrus.Logger
}

// NewCommandHandler creates a new command handler
func NewCommandHandler(
	bot *tgbotapi.BotAPI,
	cfg *config.Config,
	aiService ai.Service,
	knowledgeService knowledge.Service,
	storage *storage.Manager,
	cache cache.Service,
	rateLimiter middleware.RateLimiter,
	localizer *i18n.Localizer,
	logger *logrus.Logger,
) *CommandHandler {
	return &CommandHandler{
		bot:              bot,
		config:           cfg,
		aiService:        aiService,
		knowledgeService: knowledgeService,
		storage:          storage,
		cache:            cache,
		rateLimiter:      rateLimiter,
		localizer:        localizer,
		logger:           logger,
	}
}

// HandleCommand processes telegram commands
func (h *CommandHandler) HandleCommand(ctx context.Context, message *tgbotapi.Message) error {
	chatID := message.Chat.ID
	userID := message.From.ID
	command := message.Command()
	
	// Get user language
	settings, _ := h.storage.GetUserSettings(ctx, userID)
	lang := h.config.I18n.DefaultLanguage
	if settings != nil && settings.Language != "" {
		lang = settings.Language
	}
	
	switch command {
	case "start":
		return h.handleStart(ctx, chatID, lang)
	case "help":
		return h.handleHelp(ctx, chatID, lang)
	case "models":
		return h.handleModels(ctx, chatID, userID, lang)
	case "settings":
		return h.handleSettings(ctx, chatID, userID, lang)
	case "clear":
		return h.handleClear(ctx, chatID, userID, lang)
	case "stats":
		return h.handleStats(ctx, chatID, userID, lang)
	case "knowledge":
		return h.handleKnowledge(ctx, chatID, userID, lang)
	default:
		return h.handleUnknown(ctx, chatID, lang)
	}
}

// HandleCallbackQuery processes inline keyboard callbacks
func (h *CommandHandler) HandleCallbackQuery(ctx context.Context, callback *tgbotapi.CallbackQuery) error {
	// Parse callback data
	parts := strings.Split(callback.Data, ":")
	if len(parts) < 1 {
		return nil
	}
	
	action := parts[0]
	userID := callback.From.ID
	chatID := callback.Message.Chat.ID
	messageID := callback.Message.MessageID
	
	// Get user language
	settings, _ := h.storage.GetUserSettings(ctx, userID)
	lang := h.config.I18n.DefaultLanguage
	if settings != nil && settings.Language != "" {
		lang = settings.Language
	}
	
	switch action {
	case "menu":
		if len(parts) >= 2 {
			return h.handleMenuCallback(ctx, chatID, messageID, userID, parts[1], lang)
		}
	case "model":
		if len(parts) >= 2 {
			return h.handleModelCallback(ctx, chatID, messageID, userID, parts[1], lang, callback.ID)
		}
	case "lang":
		if len(parts) >= 2 {
			return h.handleLanguageCallback(ctx, chatID, messageID, userID, parts[1], callback.ID)
		}
	case "action":
		if len(parts) >= 2 {
			return h.handleActionCallback(ctx, chatID, messageID, userID, parts[1], lang, callback.ID)
		}
	case "custom_model":
		if len(parts) >= 2 {
			return h.handleCustomModelCallback(ctx, chatID, messageID, userID, parts[1], lang, callback.ID)
		}
	case "knowledge":
		if len(parts) >= 2 {
			return h.handleKnowledgeCallback(ctx, chatID, messageID, userID, parts[1], lang, callback.ID)
		}
	case "mention":
		if len(parts) >= 2 {
			return h.handleMentionCallback(ctx, chatID, messageID, userID, parts[1], lang, callback.ID)
		}
	case "mention_del":
		if len(parts) >= 2 {
			return h.handleMentionCallback(ctx, chatID, messageID, userID, "del:"+parts[1], lang, callback.ID)
		}
	case "noop":
		// Answer callback to remove loading state
		h.bot.Request(tgbotapi.NewCallback(callback.ID, ""))
	}
	
	return nil
}

// handleStart handles /start command
func (h *CommandHandler) handleStart(ctx context.Context, chatID int64, lang string) error {
	text := h.localizer.Get(lang, i18n.MsgWelcome, nil)
	
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = h.createMainMenuKeyboard(lang)
	
	_, err := h.bot.Send(msg)
	return err
}

// handleHelp handles /help command
func (h *CommandHandler) handleHelp(ctx context.Context, chatID int64, lang string) error {
	text := h.localizer.Get(lang, i18n.MsgHelp, nil)
	
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	
	_, err := h.bot.Send(msg)
	return err
}

// handleModels handles /models command
func (h *CommandHandler) handleModels(ctx context.Context, chatID int64, userID int64, lang string) error {
	settings, err := h.storage.GetUserSettings(ctx, userID)
	if err != nil || settings == nil {
		settings = &models.UserSettings{
			Language: lang,
			Model:    h.config.Models.Default,
		}
	}
	
	// Get current model info
	currentModel, _ := h.aiService.GetModelByID(settings.Model)
	currentModelName := "Unknown"
	if currentModel != nil {
		currentModelName = currentModel.Name
	}
	
	text := h.localizer.Get(lang, i18n.MsgCurrentModel, map[string]interface{}{
		"Model": currentModelName,
	})
	
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = h.createModelSelectionKeyboard(settings.Model)
	
	_, err = h.bot.Send(msg)
	return err
}

// handleSettings handles /settings command
func (h *CommandHandler) handleSettings(ctx context.Context, chatID int64, userID int64, lang string) error {
	text := h.localizer.Get(lang, i18n.MsgSettings, map[string]interface{}{
		"Language": lang,
	})
	
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = h.createSettingsKeyboard(lang)
	
	_, err := h.bot.Send(msg)
	return err
}

// handleClear handles /clear command
func (h *CommandHandler) handleClear(ctx context.Context, chatID int64, userID int64, lang string) error {
	if err := h.storage.ClearContext(ctx, userID); err != nil {
		h.logger.WithError(err).Error("Failed to clear context")
	}
	
	text := h.localizer.Get(lang, i18n.MsgContextCleared, nil)
	msg := tgbotapi.NewMessage(chatID, text)
	
	_, err := h.bot.Send(msg)
	return err
}

// handleStats handles /stats command
func (h *CommandHandler) handleStats(ctx context.Context, chatID int64, userID int64, lang string) error {
	stats, err := h.storage.GetUserStats(ctx, userID)
	if err != nil {
		stats = &models.UserStats{
			TotalMessages: 0,
			TotalSessions: 0,
		}
	}
	
	text := h.localizer.Get(lang, i18n.MsgStats, map[string]interface{}{
		"Messages": stats.TotalMessages,
		"Sessions": stats.TotalSessions,
	})
	
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	
	_, err = h.bot.Send(msg)
	return err
}

// handleUnknown handles unknown commands
func (h *CommandHandler) handleUnknown(ctx context.Context, chatID int64, lang string) error {
	text := h.localizer.Get(lang, i18n.MsgUnknownCommand, nil)
	msg := tgbotapi.NewMessage(chatID, text)
	
	_, err := h.bot.Send(msg)
	return err
}

// Callback handlers

func (h *CommandHandler) handleMenuCallback(ctx context.Context, chatID int64, messageID int, userID int64, menu string, lang string) error {
	var text string
	var keyboard tgbotapi.InlineKeyboardMarkup
	
	switch menu {
	case "main":
		text = h.localizer.Get(lang, i18n.MsgWelcome, nil)
		keyboard = h.createMainMenuKeyboard(lang)
	case "models":
		settings, _ := h.storage.GetUserSettings(ctx, userID)
		if settings == nil {
			settings = &models.UserSettings{
				Model: h.config.Models.Default,
			}
		}
		currentModel, _ := h.aiService.GetModelByID(settings.Model)
		currentModelName := "Unknown"
		if currentModel != nil {
			currentModelName = currentModel.Name
		}
		text = h.localizer.Get(lang, i18n.MsgCurrentModel, map[string]interface{}{
			"Model": currentModelName,
		})
		keyboard = h.createModelSelectionKeyboard(settings.Model)
	case "settings":
		text = h.localizer.Get(lang, i18n.MsgSettings, map[string]interface{}{
			"Language": lang,
		})
		keyboard = h.createSettingsKeyboard(lang)
	case "stats":
		stats, _ := h.storage.GetUserStats(ctx, userID)
		if stats == nil {
			stats = &models.UserStats{}
		}
		text = h.localizer.Get(lang, i18n.MsgStats, map[string]interface{}{
			"Messages": stats.TotalMessages,
			"Sessions": stats.TotalSessions,
		})
		keyboard = h.createBackButtonKeyboard(lang)
	default:
		return nil
	}
	
	edit := tgbotapi.NewEditMessageText(chatID, messageID, text)
	edit.ParseMode = "Markdown"
	edit.ReplyMarkup = &keyboard
	
	_, err := h.bot.Send(edit)
	return err
}

func (h *CommandHandler) handleModelCallback(ctx context.Context, chatID int64, messageID int, userID int64, modelID string, lang string, callbackID string) error {
	// Update user settings
	settings, err := h.storage.GetUserSettings(ctx, userID)
	if err != nil || settings == nil {
		settings = &models.UserSettings{
			Language: lang,
		}
	}
	
	// Validate model exists
	model, err := h.aiService.GetModelByID(modelID)
	if err != nil {
		h.bot.Request(tgbotapi.NewCallback(callbackID, h.localizer.Get(lang, "error.model_not_found", nil)))
		return nil
	}
	
	settings.Model = modelID
	if err := h.storage.SaveUserSettings(ctx, userID, settings); err != nil {
		h.logger.WithError(err).Error("Failed to save user settings")
		h.bot.Request(tgbotapi.NewCallback(callbackID, h.localizer.Get(lang, "error.save_failed", nil)))
		return nil
	}
	
	// Clear context when model changes
	h.storage.ClearContext(ctx, userID)
	
	// Update message
	text := h.localizer.Get(lang, i18n.MsgModelChanged, map[string]interface{}{
		"Model": model.Name,
	})
	
	edit := tgbotapi.NewEditMessageText(chatID, messageID, text)
	edit.ParseMode = "Markdown"
	keyboard := h.createModelSelectionKeyboard(modelID)
	edit.ReplyMarkup = &keyboard
	
	_, err = h.bot.Send(edit)
	
	// Answer callback
	h.bot.Request(tgbotapi.NewCallback(callbackID, h.localizer.Get(lang, "success.model_changed", nil)))
	
	return err
}

func (h *CommandHandler) handleLanguageCallback(ctx context.Context, chatID int64, messageID int, userID int64, newLang string, callbackID string) error {
	// Validate language
	validLang := false
	for _, l := range h.config.I18n.Languages {
		if l == newLang {
			validLang = true
			break
		}
	}
	
	if !validLang {
		h.bot.Request(tgbotapi.NewCallback(callbackID, "Invalid language"))
		return nil
	}
	
	// Update user settings
	settings, err := h.storage.GetUserSettings(ctx, userID)
	if err != nil || settings == nil {
		settings = &models.UserSettings{
			Model: h.config.Models.Default,
		}
	}
	
	settings.Language = newLang
	if err := h.storage.SaveUserSettings(ctx, userID, settings); err != nil {
		h.logger.WithError(err).Error("Failed to save user settings")
		h.bot.Request(tgbotapi.NewCallback(callbackID, "Failed to save settings"))
		return nil
	}
	
	// Update message with new language
	text := h.localizer.Get(newLang, i18n.MsgSettings, map[string]interface{}{
		"Language": newLang,
	})
	
	edit := tgbotapi.NewEditMessageText(chatID, messageID, text)
	edit.ParseMode = "Markdown"
	keyboard := h.createSettingsKeyboard(newLang)
	edit.ReplyMarkup = &keyboard
	
	_, err = h.bot.Send(edit)
	
	// Answer callback
	h.bot.Request(tgbotapi.NewCallback(callbackID, h.localizer.Get(newLang, "success.language_changed", nil)))
	
	return err
}

func (h *CommandHandler) handleActionCallback(ctx context.Context, chatID int64, messageID int, userID int64, action string, lang string, callbackID string) error {
	switch action {
	case "clear":
		if err := h.storage.ClearContext(ctx, userID); err != nil {
			h.logger.WithError(err).Error("Failed to clear context")
			h.bot.Request(tgbotapi.NewCallback(callbackID, h.localizer.Get(lang, "error.clear_failed", nil)))
			return nil
		}
		
		// Update message
		text := h.localizer.Get(lang, i18n.MsgContextCleared, nil)
		edit := tgbotapi.NewEditMessageText(chatID, messageID, text)
		edit.ParseMode = "Markdown"
		keyboard := h.createBackButtonKeyboard(lang)
		edit.ReplyMarkup = &keyboard
		
		_, err := h.bot.Send(edit)
		
		// Answer callback
		h.bot.Request(tgbotapi.NewCallback(callbackID, h.localizer.Get(lang, "success.context_cleared", nil)))
		
		return err
		
	case "mention_words":
		// Get current settings
		settings, err := h.storage.GetSettings(ctx, chatID)
		if err != nil || settings == nil {
			// Use default mention words from config
			defaultMentionWords := h.config.Context.DefaultMentionWords
			if len(defaultMentionWords) == 0 {
				defaultMentionWords = []string{"小菲", "小菲ai", "小菲AI", "ai", "AI"}
			}
			settings = &models.ChatSettings{
				MentionWords: defaultMentionWords,
			}
		}
		
		// Build mention words display
		var text strings.Builder
		text.WriteString("💬 **提及词管理**\n\n")
		text.WriteString("当群组消息中包含以下词汇时，机器人将自动回复：\n\n")
		
		if len(settings.MentionWords) > 0 {
			text.WriteString("**当前提及词：**\n")
			for i, word := range settings.MentionWords {
				text.WriteString(fmt.Sprintf("%d. `%s`\n", i+1, word))
			}
		} else {
			text.WriteString("_暂无提及词_\n")
		}
		
		text.WriteString("\n请选择操作：")
		
		// Create keyboard
		rows := [][]tgbotapi.InlineKeyboardButton{
			{
				tgbotapi.NewInlineKeyboardButtonData("➕ 添加提及词", "mention:add"),
			},
		}
		
		if len(settings.MentionWords) > 0 {
			rows = append(rows, []tgbotapi.InlineKeyboardButton{
				tgbotapi.NewInlineKeyboardButtonData("➖ 删除提及词", "mention:delete"),
			})
			rows = append(rows, []tgbotapi.InlineKeyboardButton{
				tgbotapi.NewInlineKeyboardButtonData("🔄 重置为默认", "mention:reset"),
			})
		}
		
		rows = append(rows, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("⬅️ 返回", "menu:settings"),
		})
		
		keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
		
		edit := tgbotapi.NewEditMessageText(chatID, messageID, text.String())
		edit.ParseMode = "Markdown"
		edit.ReplyMarkup = &keyboard
		
		_, err = h.bot.Send(edit)
		h.bot.Request(tgbotapi.NewCallback(callbackID, ""))
		return err
	}
	
	return nil
}

// Keyboard creators

func (h *CommandHandler) createMainMenuKeyboard(lang string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🤖 "+h.localizer.Get(lang, "button.models", nil), "menu:models"),
			tgbotapi.NewInlineKeyboardButtonData("⚙️ "+h.localizer.Get(lang, "button.settings", nil), "menu:settings"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🗑 "+h.localizer.Get(lang, "button.clear", nil), "action:clear"),
			tgbotapi.NewInlineKeyboardButtonData("📊 "+h.localizer.Get(lang, "button.stats", nil), "menu:stats"),
		),
	)
}

func (h *CommandHandler) createModelSelectionKeyboard(currentModelID string) tgbotapi.InlineKeyboardMarkup {
	models := h.aiService.GetAvailableModels()
	rows := make([][]tgbotapi.InlineKeyboardButton, 0)
	
	// Group models by endpoint
	endpointModels := make(map[string][]ai.ModelOption)
	for _, model := range models {
		endpointModels[model.EndpointName] = append(endpointModels[model.EndpointName], model)
	}
	
	// Create buttons for each endpoint group
	for endpointName, models := range endpointModels {
		endpoint := h.getEndpointByName(endpointName)
		if endpoint != nil {
			// Add endpoint header
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(
					fmt.Sprintf("📍 %s", endpoint.DisplayName),
					"noop",
				),
			))
			
			// Add model buttons
			for _, model := range models {
				checkmark := ""
				if model.ID == currentModelID {
					checkmark = "✅ "
				}
				
				rows = append(rows, tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData(
						fmt.Sprintf("%s%s", checkmark, model.Name),
						fmt.Sprintf("model:%s", model.ID),
					),
				))
			}
		}
	}
	
	// Add custom model configuration button
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("⚙️ 配置自定义模型", "custom_model:config"),
	))
	
	// Add back button
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("⬅️ Back", "menu:main"),
	))
	
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func (h *CommandHandler) createSettingsKeyboard(currentLang string) tgbotapi.InlineKeyboardMarkup {
	rows := [][]tgbotapi.InlineKeyboardButton{
		{
			tgbotapi.NewInlineKeyboardButtonData("🌐 Language", "noop"),
		},
	}
	
	// Add language options
	for _, lang := range h.config.I18n.Languages {
		checkmark := ""
		if lang == currentLang {
			checkmark = "✅ "
		}
		
		langName := h.localizer.Get(lang, "language.name", nil)
		rows = append(rows, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("%s%s", checkmark, langName),
				fmt.Sprintf("lang:%s", lang),
			),
		})
	}
	
	// Add mention words button
	rows = append(rows, []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("💬 提及词管理", "action:mention_words"),
	})
	
	// Add back button
	rows = append(rows, []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("⬅️ Back", "menu:main"),
	})
	
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func (h *CommandHandler) createBackButtonKeyboard(lang string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⬅️ "+h.localizer.Get(lang, "button.back", nil), "menu:main"),
		),
	)
}

// handleCustomModelCallback handles custom model configuration callbacks
func (h *CommandHandler) handleCustomModelCallback(ctx context.Context, chatID int64, messageID int, userID int64, action string, lang string, callbackID string) error {
	switch action {
	case "config":
		// Show custom model configuration menu
		text := "🔧 自定义模型配置\n\n请选择要配置的端点："
		
		// Create endpoint selection keyboard
		rows := [][]tgbotapi.InlineKeyboardButton{}
		
		// Add buttons for each endpoint
		for _, endpoint := range h.config.Models.Endpoints {
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(
					fmt.Sprintf("📍 %s", endpoint.DisplayName),
					fmt.Sprintf("custom_model:endpoint:%s", endpoint.Name),
				),
			))
		}
		
		// Add new endpoint button
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("➕ 添加新端点", "custom_model:new_endpoint"),
		))
		
		// Add back button
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⬅️ 返回", "menu:models"),
		))
		
		keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
		
		edit := tgbotapi.NewEditMessageText(chatID, messageID, text)
		edit.ParseMode = "Markdown"
		edit.ReplyMarkup = &keyboard
		
		_, err := h.bot.Send(edit)
		h.bot.Request(tgbotapi.NewCallback(callbackID, ""))
		return err
		
	case "new_endpoint":
		// Start new endpoint configuration flow
		text := "📝 添加新端点\n\n请按以下格式发送端点信息：\n\n" +
			"```\n" +
			"名称: my-endpoint\n" +
			"显示名称: My Custom Endpoint\n" +
			"API地址: https://api.example.com/v1\n" +
			"API密钥: sk-xxxxx\n" +
			"模型列表: gpt-3.5-turbo,gpt-4\n" +
			"```\n\n" +
			"💡 提示：模型列表用逗号分隔"
		
		// Store state to track that user is configuring a new endpoint
		h.storage.SetUserState(ctx, userID, "configuring_endpoint", "new")
		
		edit := tgbotapi.NewEditMessageText(chatID, messageID, text)
		edit.ParseMode = "Markdown"
		
		_, err := h.bot.Send(edit)
		h.bot.Request(tgbotapi.NewCallback(callbackID, "配置模式已激活"))
		return err
		
	default:
		// Handle endpoint-specific actions
		if strings.HasPrefix(action, "endpoint:") {
			endpointName := strings.TrimPrefix(action, "endpoint:")
			
			// Find the endpoint
			var endpoint *config.ModelEndpoint
			for i := range h.config.Models.Endpoints {
				if h.config.Models.Endpoints[i].Name == endpointName {
					endpoint = &h.config.Models.Endpoints[i]
					break
				}
			}
			
			if endpoint == nil {
				h.bot.Request(tgbotapi.NewCallback(callbackID, "端点未找到"))
				return nil
			}
			
			// Show endpoint configuration options
			text := fmt.Sprintf("⚙️ 配置端点: %s\n\n当前配置：\n"+
				"API地址: `%s`\n"+
				"模型数量: %d\n\n"+
				"请选择操作：", 
				endpoint.DisplayName, 
				endpoint.BaseURL,
				len(endpoint.Models))
			
			rows := [][]tgbotapi.InlineKeyboardButton{
				{
					tgbotapi.NewInlineKeyboardButtonData("➕ 添加模型", fmt.Sprintf("custom_model:add_model:%s", endpointName)),
				},
				{
					tgbotapi.NewInlineKeyboardButtonData("📝 修改API地址", fmt.Sprintf("custom_model:edit_url:%s", endpointName)),
				},
				{
					tgbotapi.NewInlineKeyboardButtonData("🔑 修改API密钥", fmt.Sprintf("custom_model:edit_key:%s", endpointName)),
				},
				{
					tgbotapi.NewInlineKeyboardButtonData("⬅️ 返回", "custom_model:config"),
				},
			}
			
			keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
			
			edit := tgbotapi.NewEditMessageText(chatID, messageID, text)
			edit.ParseMode = "Markdown"
			edit.ReplyMarkup = &keyboard
			
			_, err := h.bot.Send(edit)
			h.bot.Request(tgbotapi.NewCallback(callbackID, ""))
			return err
		}
		
		// Handle other custom model actions
		if strings.HasPrefix(action, "add_model:") {
			endpointName := strings.TrimPrefix(action, "add_model:")
			
			text := fmt.Sprintf("➕ 添加模型到 %s\n\n"+
				"请发送模型信息，格式如下：\n\n"+
				"```\n"+
				"模型ID: gpt-4-turbo\n"+
				"显示名称: GPT-4 Turbo\n"+
				"```", endpointName)
			
			// Store state
			h.storage.SetUserState(ctx, userID, "adding_model", endpointName)
			
			edit := tgbotapi.NewEditMessageText(chatID, messageID, text)
			edit.ParseMode = "Markdown"
			
			_, err := h.bot.Send(edit)
			h.bot.Request(tgbotapi.NewCallback(callbackID, "请输入模型信息"))
			return err
		}
	}
	
	return nil
}

// Helper methods

func (h *CommandHandler) getEndpointByName(name string) *config.ModelEndpoint {
	for i := range h.config.Models.Endpoints {
		if h.config.Models.Endpoints[i].Name == name {
			return &h.config.Models.Endpoints[i]
		}
	}
	return nil
}