package handlers

import (
	"context"
	"fmt"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// handleKnowledgeSearch handles knowledge base search from message
func (h *MessageHandler) handleKnowledgeSearch(ctx context.Context, update *tgbotapi.Update) error {
	userID := update.Message.From.ID
	chatID := update.Message.Chat.ID
	messageText := update.Message.Text
	
	// Clear search state
	h.storage.DeleteUserState(ctx, userID, "knowledge_search")
	
	if h.knowledgeService == nil {
		msg := tgbotapi.NewMessage(chatID, "❌ 知识库服务未启用")
		_, err := h.bot.Send(msg)
		return err
	}
	
	// Search documents
	docs, err := h.knowledgeService.SearchDocuments(ctx, messageText, 5)
	if err != nil {
		h.logger.WithError(err).Error("Failed to search knowledge base")
		msg := tgbotapi.NewMessage(chatID, "❌ 搜索失败："+err.Error())
		_, err := h.bot.Send(msg)
		return err
	}
	
	if len(docs) == 0 {
		msg := tgbotapi.NewMessage(chatID, "🔍 未找到相关文档")
		_, err := h.bot.Send(msg)
		return err
	}
	
	var result strings.Builder
	result.WriteString("🔍 搜索结果：\n\n")
	
	for i, doc := range docs {
		result.WriteString(fmt.Sprintf("%d. 📄 **%s**\n", i+1, doc.Title))
		
		// Show preview
		preview := doc.Content
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		result.WriteString(fmt.Sprintf("   %s\n\n", preview))
	}
	
	msg := tgbotapi.NewMessage(chatID, result.String())
	msg.ParseMode = "Markdown"
	_, err = h.bot.Send(msg)
	return err
}