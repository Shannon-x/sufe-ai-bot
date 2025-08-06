package handlers

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/cf-ai-tgbot-go/internal/models"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// handleMentionCallback handles mention word management callbacks
func (h *CommandHandler) handleMentionCallback(ctx context.Context, chatID int64, messageID int, userID int64, action string, lang string, callbackID string) error {
	switch action {
	case "add":
		// Set user state to adding mention word
		h.storage.SetUserState(ctx, userID, "adding_mention", "true")
		
		text := "➕ **添加提及词**\n\n" +
			"请发送要添加的提及词（可以是中文或英文）：\n\n" +
			"例如：`小助手`、`助理`、`bot` 等\n\n" +
			"_发送 /cancel 取消操作_"
		
		edit := tgbotapi.NewEditMessageText(chatID, messageID, text)
		edit.ParseMode = "Markdown"
		
		_, err := h.bot.Send(edit)
		h.bot.Request(tgbotapi.NewCallback(callbackID, "请输入提及词"))
		return err
		
	case "delete":
		// Get current settings
		settings, err := h.storage.GetSettings(ctx, chatID)
		if err != nil || settings == nil || len(settings.MentionWords) == 0 {
			h.bot.Request(tgbotapi.NewCallback(callbackID, "没有可删除的提及词"))
			return nil
		}
		
		// Build delete menu
		var text strings.Builder
		text.WriteString("➖ **删除提及词**\n\n")
		text.WriteString("请选择要删除的提及词：\n\n")
		
		rows := [][]tgbotapi.InlineKeyboardButton{}
		
		for i, word := range settings.MentionWords {
			rows = append(rows, []tgbotapi.InlineKeyboardButton{
				tgbotapi.NewInlineKeyboardButtonData(
					fmt.Sprintf("🗑 %s", word),
					fmt.Sprintf("mention_del:%d", i),
				),
			})
		}
		
		// Add back button
		rows = append(rows, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("⬅️ 返回", "action:mention_words"),
		})
		
		keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
		
		edit := tgbotapi.NewEditMessageText(chatID, messageID, text.String())
		edit.ParseMode = "Markdown"
		edit.ReplyMarkup = &keyboard
		
		_, err = h.bot.Send(edit)
		h.bot.Request(tgbotapi.NewCallback(callbackID, ""))
		return err
		
	case "reset":
		// Reset to default mention words
		settings, err := h.storage.GetSettings(ctx, chatID)
		if err != nil || settings == nil {
			settings = &models.ChatSettings{}
		}
		
		// Use default mention words from config
		defaultMentionWords := h.config.Context.DefaultMentionWords
		if len(defaultMentionWords) == 0 {
			defaultMentionWords = []string{"小菲", "小菲ai", "小菲AI", "ai", "AI"}
		}
		settings.MentionWords = defaultMentionWords
		
		if err := h.storage.SaveSettings(ctx, chatID, settings); err != nil {
			h.logger.WithError(err).Error("Failed to save settings")
			h.bot.Request(tgbotapi.NewCallback(callbackID, "重置失败"))
			return err
		}
		
		h.bot.Request(tgbotapi.NewCallback(callbackID, "已重置为默认提及词"))
		
		// Refresh the mention words display
		return h.handleActionCallback(ctx, chatID, messageID, userID, "mention_words", lang, "")
		
	default:
		// Handle delete specific word
		if strings.HasPrefix(action, "del:") {
			indexStr := strings.TrimPrefix(action, "del:")
			index, err := strconv.Atoi(indexStr)
			if err != nil {
				h.bot.Request(tgbotapi.NewCallback(callbackID, "无效的索引"))
				return nil
			}
			
			// Get current settings
			settings, err := h.storage.GetSettings(ctx, chatID)
			if err != nil || settings == nil || index < 0 || index >= len(settings.MentionWords) {
				h.bot.Request(tgbotapi.NewCallback(callbackID, "无效的索引"))
				return nil
			}
			
			// Remove the word
			deletedWord := settings.MentionWords[index]
			settings.MentionWords = append(settings.MentionWords[:index], settings.MentionWords[index+1:]...)
			
			// Save settings
			if err := h.storage.SaveSettings(ctx, chatID, settings); err != nil {
				h.logger.WithError(err).Error("Failed to save settings")
				h.bot.Request(tgbotapi.NewCallback(callbackID, "删除失败"))
				return err
			}
			
			h.bot.Request(tgbotapi.NewCallback(callbackID, fmt.Sprintf("已删除: %s", deletedWord)))
			
			// Refresh the mention words display
			return h.handleActionCallback(ctx, chatID, messageID, userID, "mention_words", lang, "")
		}
	}
	
	return nil
}