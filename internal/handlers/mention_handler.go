package handlers

import (
	"context"
	"fmt"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// handleAddMentionWord handles adding a new mention word
func (h *MessageHandler) handleAddMentionWord(ctx context.Context, update *tgbotapi.Update) error {
	userID := update.Message.From.ID
	chatID := update.Message.Chat.ID
	messageText := strings.TrimSpace(update.Message.Text)
	
	// Clear state
	h.storage.DeleteUserState(ctx, userID, "adding_mention")
	
	// Check if it's cancel command
	if messageText == "/cancel" || messageText == "取消" {
		msg := tgbotapi.NewMessage(chatID, "已取消添加提及词")
		_, err := h.bot.Send(msg)
		return err
	}
	
	// Validate mention word
	if len(messageText) == 0 || len(messageText) > 20 {
		msg := tgbotapi.NewMessage(chatID, "❌ 提及词长度应在 1-20 个字符之间")
		_, err := h.bot.Send(msg)
		return err
	}
	
	// Get current settings
	settings, err := h.storage.GetSettings(ctx, chatID)
	if err != nil || settings == nil {
		settings = h.getDefaultSettings()
	}
	
	// Check if already exists
	for _, word := range settings.MentionWords {
		if strings.EqualFold(word, messageText) {
			msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ 提及词 `%s` 已存在", messageText))
			msg.ParseMode = "Markdown"
			_, err := h.bot.Send(msg)
			return err
		}
	}
	
	// Add mention word
	settings.MentionWords = append(settings.MentionWords, messageText)
	
	// Save settings
	if err := h.storage.SaveSettings(ctx, chatID, settings); err != nil {
		h.logger.WithError(err).Error("Failed to save settings")
		msg := tgbotapi.NewMessage(chatID, "❌ 保存失败，请稍后重试")
		_, err := h.bot.Send(msg)
		return err
	}
	
	// Send success message
	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ 已添加提及词: `%s`\n\n当群组消息中包含此词汇时，机器人将自动回复。", messageText))
	msg.ParseMode = "Markdown"
	
	// Add keyboard to return to mention management
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💬 查看所有提及词", "action:mention_words"),
		),
	)
	msg.ReplyMarkup = keyboard
	
	_, err = h.bot.Send(msg)
	return err
}