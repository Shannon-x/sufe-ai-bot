package handlers

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/cf-ai-tgbot-go/internal/config"
	"github.com/cf-ai-tgbot-go/internal/i18n"
	"github.com/cf-ai-tgbot-go/internal/middleware"
	"github.com/cf-ai-tgbot-go/internal/models"
	"github.com/cf-ai-tgbot-go/internal/services/ai"
	"github.com/cf-ai-tgbot-go/internal/services/cache"
	"github.com/cf-ai-tgbot-go/internal/services/knowledge"
	"github.com/cf-ai-tgbot-go/internal/services/storage"
	"github.com/cf-ai-tgbot-go/pkg/markdown"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/sirupsen/logrus"
)

// MessageHandler handles regular messages
type MessageHandler struct {
	config           *config.Config
	bot              *tgbotapi.BotAPI
	aiService        ai.Service
	knowledgeService knowledge.Service
	storage          *storage.Manager
	cache            cache.Service
	rateLimiter      middleware.RateLimiter
	security         *middleware.SecurityMiddleware
	localizer        *i18n.Localizer
	logger           *logrus.Logger
}

// NewMessageHandler creates a new message handler
func NewMessageHandler(
	cfg *config.Config,
	bot *tgbotapi.BotAPI,
	aiService ai.Service,
	knowledgeService knowledge.Service,
	storage *storage.Manager,
	cache cache.Service,
	rateLimiter middleware.RateLimiter,
	localizer *i18n.Localizer,
	logger *logrus.Logger,
) *MessageHandler {
	return &MessageHandler{
		config:           cfg,
		bot:              bot,
		aiService:        aiService,
		knowledgeService: knowledgeService,
		storage:          storage,
		cache:            cache,
		rateLimiter:      rateLimiter,
		security:         middleware.NewSecurityMiddleware(logger),
		localizer:        localizer,
		logger:           logger,
	}
}

// HandleMessage processes regular messages
func (h *MessageHandler) HandleMessage(ctx context.Context, update *tgbotapi.Update) error {
	if update.Message == nil || update.Message.IsCommand() {
		return nil
	}

	// Ignore bot's own messages
	if update.Message.From.ID == h.bot.Self.ID {
		return nil
	}

	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID
	messageText := update.Message.Text

	// Check if user is in configuration state
	configuringEndpoint, err := h.storage.GetUserState(ctx, userID, "configuring_endpoint")
	if err == nil && configuringEndpoint != "" {
		// Handle endpoint configuration
		return h.handleEndpointConfiguration(ctx, update, configuringEndpoint)
	}
	
	addingModel, err := h.storage.GetUserState(ctx, userID, "adding_model")
	if err == nil && addingModel != "" {
		// Handle model addition
		return h.handleModelAddition(ctx, update, addingModel)
	}

	// Check if adding mention word
	addingMention, err := h.storage.GetUserState(ctx, userID, "adding_mention")
	if err == nil && addingMention == "true" {
		// Handle adding mention word
		return h.handleAddMentionWord(ctx, update)
	}
	
	// Check if searching in knowledge base
	searchingKnowledge, err := h.storage.GetUserState(ctx, userID, "knowledge_search")
	if err == nil && searchingKnowledge == "true" {
		// Handle knowledge search
		return h.handleKnowledgeSearch(ctx, update)
	}
	
	// Check if bot should respond
	shouldRespond, err := h.shouldRespond(ctx, update)
	if err != nil {
		h.logger.WithError(err).Error("Failed to check if should respond")
		return err
	}
	
	h.logger.WithFields(logrus.Fields{
		"chatID":        chatID,
		"userID":        userID,
		"shouldRespond": shouldRespond,
		"isGroup":       !update.Message.Chat.IsPrivate(),
		"messageText":   messageText,
	}).Debug("Checking if should respond")

	if !shouldRespond {
		return nil
	}

	// Check rate limit
	if !h.rateLimiter.Allow(userID) {
		lang := h.getUserLanguage(ctx, chatID)
		msg := tgbotapi.NewMessage(chatID, h.localizer.Get(lang, i18n.MsgRateLimitExceeded, nil))
		msg.ReplyToMessageID = update.Message.MessageID
		if _, err := h.bot.Send(msg); err != nil {
			h.logger.WithError(err).Error("Failed to send rate limit message")
		}
		return nil
	}

	// Validate input
	if err := h.security.ValidateInput(messageText); err != nil {
		h.logger.WithError(err).Warn("Input validation failed")
		return nil
	}

	// Send thinking message
	lang := h.getUserLanguage(ctx, chatID)
	thinkingMsg := tgbotapi.NewMessage(chatID, h.localizer.Get(lang, i18n.MsgProcessing, nil))
	thinkingMsg.ReplyToMessageID = update.Message.MessageID
	sentMsg, err := h.bot.Send(thinkingMsg)
	if err != nil {
		h.logger.WithError(err).Error("Failed to send thinking message")
		return err
	}

	// Process message in background
	go h.processMessage(ctx, update, sentMsg.MessageID, lang)

	return nil
}

func (h *MessageHandler) processMessage(ctx context.Context, update *tgbotapi.Update, thinkingMsgID int, lang string) {
	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID
	messageText := update.Message.Text

	// Clean message text (remove bot mention)
	cleanedMessage := h.cleanMessage(messageText)
	
	// Check if triggered by mention word
	triggeredByMention := false
	if !update.Message.Chat.IsPrivate() {
		settings, _ := h.storage.GetSettings(ctx, chatID)
		if settings != nil && len(settings.MentionWords) > 0 {
			messageLower := strings.ToLower(messageText)
			for _, mention := range settings.MentionWords {
				if strings.Contains(messageLower, strings.ToLower(mention)) {
					triggeredByMention = true
					// Add a friendly greeting when triggered by mention
					cleanedMessage = h.addMentionGreeting(cleanedMessage, mention, update)
					break
				}
			}
		}
	}

	// Get or create context
	chatCtx, err := h.getOrCreateContext(ctx, chatID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get chat context")
		h.sendError(chatID, thinkingMsgID, lang)
		return
	}
	
	// Get user settings and update model if different
	userSettings, err := h.storage.GetUserSettings(ctx, userID)
	if err == nil && userSettings != nil && userSettings.Model != "" {
		chatCtx.Settings.Model = userSettings.Model
		h.logger.WithFields(logrus.Fields{
			"userID": userID,
			"model": userSettings.Model,
		}).Debug("Updated chat context with user's model")
	}

	// Get settings
	settings := &chatCtx.Settings

	// Check cache
	cachedResponse, found := h.cache.Get(ctx, cleanedMessage, settings.Model)
	if found {
		h.sendResponse(chatID, thinkingMsgID, cachedResponse, lang)
		return
	}

	// Add user message to context
	chatCtx.Messages = append(chatCtx.Messages, models.Message{
		Role:    "user",
		Content: cleanedMessage,
	})
	
	// If triggered by mention, add a hint to the system prompt
	if triggeredByMention && len(chatCtx.Messages) > 0 && chatCtx.Messages[0].Role == "system" {
		originalPrompt := chatCtx.Messages[0].Content
		chatCtx.Messages[0].Content = originalPrompt + "\n\n注意：用户刚刚通过提及词呼叫了你。请保持自然、友善的语气回应，不要过度热情或使用尴尬的表达。如果用户只是打招呼，简单友好地回应即可。如果有具体问题，直接回答问题。避免使用'哈哈'、'您又来啦'这类过于随意的表达。"
		defer func() {
			// Restore original system prompt after getting response
			if len(chatCtx.Messages) > 0 && chatCtx.Messages[0].Role == "system" {
				chatCtx.Messages[0].Content = originalPrompt
			}
		}()
	}

	// Trim context if needed
	h.trimContext(chatCtx)

	// Get AI response with knowledge base
	aiCtx, cancel := context.WithTimeout(ctx, 2*time.Minute) // Add timeout for AI request
	defer cancel()
	
	var aiResponse string
	if h.knowledgeService != nil && h.config.Knowledge.Enabled {
		aiResponse, err = h.aiService.GetResponseWithKnowledge(aiCtx, chatCtx.Messages, settings.Model, h.knowledgeService)
	} else {
		aiResponse, err = h.aiService.GetResponse(aiCtx, chatCtx.Messages, settings.Model)
	}
	
	if err != nil {
		h.logger.WithError(err).WithFields(logrus.Fields{
			"chatID": chatID,
			"userID": userID,
			"model":  settings.Model,
		}).Error("Failed to get AI response")
		h.sendError(chatID, thinkingMsgID, lang)
		return
	}

	// Process thinking tags
	processedResponse := h.processThinkingTags(aiResponse, settings.ShowThink)

	// Add AI response to context
	chatCtx.Messages = append(chatCtx.Messages, models.Message{
		Role:    "assistant",
		Content: aiResponse,
	})

	// Update last activity
	chatCtx.LastActivity = time.Now()

	// Save context
	if err := h.storage.SaveContext(ctx, chatCtx); err != nil {
		h.logger.WithError(err).Error("Failed to save context")
	}

	// Cache response
	if err := h.cache.Set(ctx, cleanedMessage, settings.Model, processedResponse); err != nil {
		h.logger.WithError(err).Warn("Failed to cache response")
	}

	// Send response
	h.sendResponse(chatID, thinkingMsgID, processedResponse, lang)
}

func (h *MessageHandler) shouldRespond(ctx context.Context, update *tgbotapi.Update) (bool, error) {
	message := update.Message
	chatID := message.Chat.ID
	messageText := strings.ToLower(message.Text)
	
	h.logger.WithFields(logrus.Fields{
		"chatID":      chatID,
		"isPrivate":   message.Chat.IsPrivate(),
		"messageText": messageText,
	}).Debug("shouldRespond check started")

	// Always respond in private chat
	if message.Chat.IsPrivate() {
		h.logger.Debug("Responding: private chat")
		return true, nil
	}

	// Check if bot is mentioned
	botUsername := "@" + h.bot.Self.UserName
	if strings.Contains(messageText, strings.ToLower(botUsername)) {
		h.logger.Debug("Responding: bot mentioned")
		return true, nil
	}

	// Check if replying to bot
	if message.ReplyToMessage != nil && message.ReplyToMessage.From.ID == h.bot.Self.ID {
		h.logger.Debug("Responding: reply to bot")
		return true, nil
	}

	// Check keywords and mention words
	settings, err := h.storage.GetSettings(ctx, chatID)
	if err != nil {
		h.logger.WithError(err).Warn("Failed to get settings")
		// Continue with defaults if error
	}
	
	// If no settings exist for this chat, create default settings
	if settings == nil && !message.Chat.IsPrivate() {
		settings = h.getDefaultSettings()
		// Save default settings for this group
		if err := h.storage.SaveSettings(ctx, chatID, settings); err != nil {
			h.logger.WithError(err).Warn("Failed to save default settings")
		}
	}
	
	// If settings exist but no mention words are set for groups, use defaults
	if settings != nil && !message.Chat.IsPrivate() && len(settings.MentionWords) == 0 {
		defaultMentionWords := h.config.Context.DefaultMentionWords
		if len(defaultMentionWords) == 0 {
			defaultMentionWords = []string{"小菲", "小菲ai", "小菲AI", "ai", "AI"}
		}
		settings.MentionWords = defaultMentionWords
		// Save updated settings
		if err := h.storage.SaveSettings(ctx, chatID, settings); err != nil {
			h.logger.WithError(err).Warn("Failed to save updated settings with default mention words")
		}
	}
	
	keywordsCount := 0
	mentionWordsCount := 0
	if settings != nil {
		keywordsCount = len(settings.Keywords)
		mentionWordsCount = len(settings.MentionWords)
	}
	
	h.logger.WithFields(logrus.Fields{
		"chatID":            chatID,
		"hasSettings":       settings != nil,
		"keywordsCount":     keywordsCount,
		"mentionWordsCount": mentionWordsCount,
	}).Debug("Checking keywords and mention words")

	if settings != nil {
		// Check keywords
		if len(settings.Keywords) > 0 {
			for _, keyword := range settings.Keywords {
				if strings.Contains(messageText, keyword) {
					h.logger.WithField("keyword", keyword).Debug("Responding: keyword match")
					return true, nil
				}
			}
		}
		
		// Check mention words
		if len(settings.MentionWords) > 0 {
			for _, mention := range settings.MentionWords {
				if strings.Contains(messageText, strings.ToLower(mention)) {
					h.logger.WithField("mention", mention).Debug("Responding: mention word match")
					return true, nil
				}
			}
		}
	}

	h.logger.Debug("Not responding: no match")
	return false, nil
}

func (h *MessageHandler) getOrCreateContext(ctx context.Context, chatID int64) (*models.ChatContext, error) {
	chatCtx, err := h.storage.GetContext(ctx, chatID)
	if err != nil {
		return nil, err
	}

	if chatCtx == nil {
		// Create new context
		settings, err := h.storage.GetSettings(ctx, chatID)
		if err != nil {
			return nil, err
		}

		if settings == nil {
			settings = h.getDefaultSettings()
			if err := h.storage.SaveSettings(ctx, chatID, settings); err != nil {
				return nil, err
			}
		}

		chatCtx = &models.ChatContext{
			ChatID:       chatID,
			Messages:     []models.Message{{Role: "system", Content: settings.SystemPrompt}},
			LastActivity: time.Now(),
			Settings:     *settings,
		}
	}

	// Ensure system prompt is up to date
	if len(chatCtx.Messages) > 0 && chatCtx.Messages[0].Role == "system" {
		chatCtx.Messages[0].Content = chatCtx.Settings.SystemPrompt
	}

	return chatCtx, nil
}

func (h *MessageHandler) trimContext(chatCtx *models.ChatContext) {
	maxMessages := h.config.Context.MaxMessages + 1 // +1 for system message
	if len(chatCtx.Messages) > maxMessages {
		// Keep system message and remove oldest messages
		chatCtx.Messages = append(chatCtx.Messages[:1], chatCtx.Messages[len(chatCtx.Messages)-maxMessages+1:]...)
	}
}

func (h *MessageHandler) cleanMessage(text string) string {
	// Remove bot mention
	botUsername := "@" + h.bot.Self.UserName
	cleaned := strings.ReplaceAll(text, botUsername, "")
	return strings.TrimSpace(cleaned)
}

func (h *MessageHandler) processThinkingTags(response string, showThink bool) string {
	if showThink {
		return response
	}

	// Remove content within <think></think> tags
	const thinkEndTag = "</think>"
	lastIndex := strings.LastIndex(response, thinkEndTag)
	if lastIndex != -1 {
		return strings.TrimSpace(response[lastIndex+len(thinkEndTag):])
	}

	return response
}

func (h *MessageHandler) sendResponse(chatID int64, messageID int, response, lang string) {
	// Convert markdown to HTML
	htmlResponse := markdown.ToTelegramHTML(response)

	// Sanitize output
	htmlResponse = h.security.SanitizeOutput(htmlResponse)

	editMsg := tgbotapi.NewEditMessageText(chatID, messageID, htmlResponse)
	editMsg.ParseMode = "HTML"

	if _, err := h.bot.Send(editMsg); err != nil {
		// If HTML parsing fails, try plain text
		h.logger.WithError(err).Warn("Failed to send HTML response, trying plain text")
		editMsg.ParseMode = ""
		editMsg.Text = response
		if _, err := h.bot.Send(editMsg); err != nil {
			h.logger.WithError(err).Error("Failed to send response")
		}
	}
}

func (h *MessageHandler) sendError(chatID int64, messageID int, lang string) {
	editMsg := tgbotapi.NewEditMessageText(chatID, messageID, h.localizer.Get(lang, i18n.MsgError, nil))
	if _, err := h.bot.Send(editMsg); err != nil {
		h.logger.WithError(err).Error("Failed to send error message")
	}
}

func (h *MessageHandler) getUserLanguage(ctx context.Context, chatID int64) string {
	settings, err := h.storage.GetSettings(ctx, chatID)
	if err != nil || settings == nil {
		return h.config.I18n.DefaultLanguage
	}
	return settings.Language
}

func (h *MessageHandler) getDefaultSettings() *models.ChatSettings {
	// Use default mention words from config if available
	defaultMentionWords := h.config.Context.DefaultMentionWords
	if len(defaultMentionWords) == 0 {
		defaultMentionWords = []string{"小菲", "小菲ai", "小菲AI", "ai", "AI"}
	}
	
	return &models.ChatSettings{
		ShowThink:    false,
		Model:        h.config.Models.Default,
		SystemPrompt: h.config.Context.DefaultSystemPrompt,
		Keywords:     []string{},
		MentionWords: defaultMentionWords,
		Language:     h.config.I18n.DefaultLanguage,
	}
}

func (h *MessageHandler) handleEndpointConfiguration(ctx context.Context, update *tgbotapi.Update, configuringType string) error {
	userID := update.Message.From.ID
	chatID := update.Message.Chat.ID
	messageText := update.Message.Text
	
	// Parse the configuration text
	lines := strings.Split(messageText, "\n")
	configData := make(map[string]string)
	
	for _, line := range lines {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			configData[key] = value
		}
	}
	
	// Validate required fields
	requiredFields := []string{"名称", "显示名称", "API地址", "API密钥", "模型列表"}
	for _, field := range requiredFields {
		if _, ok := configData[field]; !ok {
			msg := tgbotapi.NewMessage(chatID, "❌ 缺少必要字段: "+field+"\n\n请按照示例格式重新发送。")
			h.bot.Send(msg)
			return nil
		}
	}
	
	// Update the .env file
	if err := h.updateEnvFile(configData); err != nil {
		h.logger.WithError(err).Error("Failed to update .env file")
		msg := tgbotapi.NewMessage(chatID, "❌ 更新配置失败，请稍后重试。")
		h.bot.Send(msg)
		return err
	}
	
	// Clear the state
	h.storage.DeleteUserState(ctx, userID, "configuring_endpoint")
	
	// Send success message
	msg := tgbotapi.NewMessage(chatID, "✅ 端点配置成功！\n\n"+
		"名称: "+configData["名称"]+"\n"+
		"显示名称: "+configData["显示名称"]+"\n"+
		"API地址: "+configData["API地址"]+"\n"+
		"模型: "+configData["模型列表"]+"\n\n"+
		"⚠️ 请重启机器人以使配置生效。")
	msg.ParseMode = "Markdown"
	h.bot.Send(msg)
	
	return nil
}

func (h *MessageHandler) handleModelAddition(ctx context.Context, update *tgbotapi.Update, endpointName string) error {
	userID := update.Message.From.ID
	chatID := update.Message.Chat.ID
	messageText := update.Message.Text
	
	// Parse the model info
	lines := strings.Split(messageText, "\n")
	modelData := make(map[string]string)
	
	for _, line := range lines {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			modelData[key] = value
		}
	}
	
	// Validate required fields
	if modelData["模型ID"] == "" || modelData["显示名称"] == "" {
		msg := tgbotapi.NewMessage(chatID, "❌ 缺少必要字段！请提供模型ID和显示名称。")
		h.bot.Send(msg)
		return nil
	}
	
	// Add model to the endpoint in config
	if err := h.addModelToEndpoint(endpointName, modelData["模型ID"], modelData["显示名称"]); err != nil {
		h.logger.WithError(err).Error("Failed to add model")
		msg := tgbotapi.NewMessage(chatID, "❌ 添加模型失败，请稍后重试。")
		h.bot.Send(msg)
		return err
	}
	
	// Clear the state
	h.storage.DeleteUserState(ctx, userID, "adding_model")
	
	// Send success message
	msg := tgbotapi.NewMessage(chatID, "✅ 模型添加成功！\n\n"+
		"端点: "+endpointName+"\n"+
		"模型ID: "+modelData["模型ID"]+"\n"+
		"显示名称: "+modelData["显示名称"]+"\n\n"+
		"⚠️ 请重启机器人以使配置生效。")
	msg.ParseMode = "Markdown"
	h.bot.Send(msg)
	
	return nil
}

func (h *MessageHandler) updateEnvFile(configData map[string]string) error {
	envPath := "/.env"  // 修改为容器内的路径
	
	// Read current .env file
	envContent, err := os.ReadFile(envPath)
	if err != nil {
		return err
	}
	
	// Parse existing env vars
	lines := strings.Split(string(envContent), "\n")
	envVars := make(map[string]string)
	var newLines []string
	
	for _, line := range lines {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			newLines = append(newLines, line)
			continue
		}
		
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			envVars[key] = value
		}
	}
	
	// Generate new endpoint environment variables
	endpointName := strings.ToUpper(strings.ReplaceAll(configData["名称"], "-", "_"))
	envVars[endpointName+"_API_KEY"] = configData["API密钥"]
	envVars[endpointName+"_BASE_URL"] = configData["API地址"]
	envVars[endpointName+"_MODELS"] = configData["模型列表"]
	
	// Add custom endpoint to list if not already present
	customEndpoints := envVars["CUSTOM_ENDPOINTS"]
	if customEndpoints == "" {
		customEndpoints = configData["名称"]
	} else {
		endpoints := strings.Split(customEndpoints, ",")
		found := false
		for _, ep := range endpoints {
			if strings.TrimSpace(ep) == configData["名称"] {
				found = true
				break
			}
		}
		if !found {
			customEndpoints = customEndpoints + "," + configData["名称"]
		}
	}
	envVars["CUSTOM_ENDPOINTS"] = customEndpoints
	
	// Write back to file
	var output []string
	written := make(map[string]bool)
	
	// Keep comments and empty lines
	for _, line := range lines {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			output = append(output, line)
			continue
		}
		
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			if newVal, ok := envVars[key]; ok {
				output = append(output, key+"="+newVal)
				written[key] = true
			} else {
				output = append(output, line)
			}
		}
	}
	
	// Add new variables that weren't in the original file
	for key, value := range envVars {
		if !written[key] {
			output = append(output, key+"="+value)
		}
	}
	
	// Write file
	return os.WriteFile(envPath, []byte(strings.Join(output, "\n")), 0644)
}

func (h *MessageHandler) addModelToEndpoint(endpointName, modelID, displayName string) error {
	// For simplicity, we'll update the environment variable
	// This requires the config to read models from env vars
	envPath := "/.env"  // 修改为容器内的路径
	
	// Read current .env file
	envContent, err := os.ReadFile(envPath)
	if err != nil {
		return err
	}
	
	// Parse existing env vars
	lines := strings.Split(string(envContent), "\n")
	envVars := make(map[string]string)
	
	for _, line := range lines {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			envVars[key] = value
		}
	}
	
	// Update models list for the endpoint
	endpointKey := strings.ToUpper(strings.ReplaceAll(endpointName, "-", "_")) + "_MODELS"
	currentModels := envVars[endpointKey]
	
	if currentModels == "" {
		currentModels = modelID + ":" + displayName
	} else {
		// Check if model already exists
		models := strings.Split(currentModels, ",")
		found := false
		for _, m := range models {
			if strings.HasPrefix(m, modelID+":") {
				found = true
				break
			}
		}
		if !found {
			currentModels = currentModels + "," + modelID + ":" + displayName
		}
	}
	
	envVars[endpointKey] = currentModels
	
	// Write back to file
	var output []string
	written := make(map[string]bool)
	
	for _, line := range lines {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			output = append(output, line)
			continue
		}
		
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			if newVal, ok := envVars[key]; ok {
				output = append(output, key+"="+newVal)
				written[key] = true
			} else {
				output = append(output, line)
			}
		}
	}
	
	// Add new variables that weren't in the original file
	for key, value := range envVars {
		if !written[key] {
			output = append(output, key+"="+value)
		}
	}
	
	// Write file
	return os.WriteFile(envPath, []byte(strings.Join(output, "\n")), 0644)
}