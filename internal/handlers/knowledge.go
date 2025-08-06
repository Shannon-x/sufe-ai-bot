package handlers

import (
	"context"
	"fmt"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// handleKnowledge handles knowledge base management
func (h *CommandHandler) handleKnowledge(ctx context.Context, chatID int64, userID int64, lang string) error {
	// Check if knowledge service is available
	if h.knowledgeService == nil {
		msg := tgbotapi.NewMessage(chatID, "❌ 知识库服务未启用")
		_, err := h.bot.Send(msg)
		return err
	}
	
	// Get knowledge base stats
	docs := h.knowledgeService.GetAllDocuments()
	
	text := fmt.Sprintf("📚 知识库管理\n\n"+
		"📊 当前状态：\n"+
		"• 文档数量：%d\n\n"+
		"请选择操作：", len(docs))
	
	rows := [][]tgbotapi.InlineKeyboardButton{
		{
			tgbotapi.NewInlineKeyboardButtonData("📋 查看文档列表", "knowledge:list"),
		},
		{
			tgbotapi.NewInlineKeyboardButtonData("🔄 刷新知识库", "knowledge:refresh"),
		},
		{
			tgbotapi.NewInlineKeyboardButtonData("📄 添加文档", "knowledge:add"),
		},
		{
			tgbotapi.NewInlineKeyboardButtonData("🔍 搜索测试", "knowledge:search"),
		},
	}
	
	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = keyboard
	
	_, err := h.bot.Send(msg)
	return err
}

// handleKnowledgeCallback handles knowledge base callbacks
func (h *CommandHandler) handleKnowledgeCallback(ctx context.Context, chatID int64, messageID int, userID int64, action string, lang string, callbackID string) error {
	switch action {
	case "list":
		// List all documents
		docs := h.knowledgeService.GetAllDocuments()
		
		if len(docs) == 0 {
			text := "📚 知识库为空\n\n请添加 .md 文件到 knowledge 目录"
			edit := tgbotapi.NewEditMessageText(chatID, messageID, text)
			_, err := h.bot.Send(edit)
			h.bot.Request(tgbotapi.NewCallback(callbackID, ""))
			return err
		}
		
		var text strings.Builder
		text.WriteString("📚 知识库文档列表：\n\n")
		
		for i, doc := range docs {
			text.WriteString(fmt.Sprintf("%d. 📄 %s\n", i+1, doc.Title))
			text.WriteString(fmt.Sprintf("   ID: %s\n", doc.ID))
			text.WriteString(fmt.Sprintf("   大小: %d 字符\n\n", len(doc.Content)))
		}
		
		// Add back button
		rows := [][]tgbotapi.InlineKeyboardButton{
			{
				tgbotapi.NewInlineKeyboardButtonData("⬅️ 返回", "knowledge:menu"),
			},
		}
		keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
		
		edit := tgbotapi.NewEditMessageText(chatID, messageID, text.String())
		edit.ReplyMarkup = &keyboard
		
		_, err := h.bot.Send(edit)
		h.bot.Request(tgbotapi.NewCallback(callbackID, ""))
		return err
		
	case "refresh":
		// Refresh knowledge base
		h.bot.Request(tgbotapi.NewCallback(callbackID, "正在刷新知识库..."))
		
		err := h.knowledgeService.RefreshKnowledgeBase(ctx)
		if err != nil {
			h.logger.WithError(err).Error("Failed to refresh knowledge base")
			edit := tgbotapi.NewEditMessageText(chatID, messageID, "❌ 刷新失败："+err.Error())
			h.bot.Send(edit)
			return err
		}
		
		docs := h.knowledgeService.GetAllDocuments()
		text := fmt.Sprintf("✅ 知识库刷新成功！\n\n已加载 %d 个文档", len(docs))
		
		// Add back button
		rows := [][]tgbotapi.InlineKeyboardButton{
			{
				tgbotapi.NewInlineKeyboardButtonData("⬅️ 返回", "knowledge:menu"),
			},
		}
		keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
		
		edit := tgbotapi.NewEditMessageText(chatID, messageID, text)
		edit.ReplyMarkup = &keyboard
		
		_, err = h.bot.Send(edit)
		return err
		
	case "add":
		// Instructions for adding documents
		text := "📄 添加文档到知识库\n\n" +
			"请将 Markdown (.md) 文件放置到以下目录：\n" +
			"`/knowledge/`\n\n" +
			"文件格式示例：\n" +
			"```markdown\n" +
			"# 文档标题\n\n" +
			"## 章节1\n" +
			"内容...\n\n" +
			"## 章节2\n" +
			"内容...\n" +
			"```\n\n" +
			"添加文件后，点击「刷新知识库」按钮加载新文档。"
		
		// Add back button
		rows := [][]tgbotapi.InlineKeyboardButton{
			{
				tgbotapi.NewInlineKeyboardButtonData("🔄 刷新知识库", "knowledge:refresh"),
			},
			{
				tgbotapi.NewInlineKeyboardButtonData("⬅️ 返回", "knowledge:menu"),
			},
		}
		keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
		
		edit := tgbotapi.NewEditMessageText(chatID, messageID, text)
		edit.ParseMode = "Markdown"
		edit.ReplyMarkup = &keyboard
		
		_, err := h.bot.Send(edit)
		h.bot.Request(tgbotapi.NewCallback(callbackID, ""))
		return err
		
	case "search":
		// Search test prompt
		text := "🔍 搜索测试\n\n" +
			"请发送要搜索的关键词，我将在知识库中查找相关文档。\n\n" +
			"例如：「如何配置」「API 文档」等"
		
		// Store state
		h.storage.SetUserState(ctx, userID, "knowledge_search", "true")
		
		// Add back button
		rows := [][]tgbotapi.InlineKeyboardButton{
			{
				tgbotapi.NewInlineKeyboardButtonData("❌ 取消", "knowledge:cancel_search"),
			},
		}
		keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
		
		edit := tgbotapi.NewEditMessageText(chatID, messageID, text)
		edit.ReplyMarkup = &keyboard
		
		_, err := h.bot.Send(edit)
		h.bot.Request(tgbotapi.NewCallback(callbackID, "请输入搜索关键词"))
		return err
		
	case "cancel_search":
		// Cancel search
		h.storage.DeleteUserState(ctx, userID, "knowledge_search")
		return h.handleKnowledge(ctx, chatID, userID, lang)
		
	case "menu":
		// Return to knowledge menu
		return h.handleKnowledge(ctx, chatID, userID, lang)
	}
	
	return nil
}