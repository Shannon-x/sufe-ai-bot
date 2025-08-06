package models

import (
	"time"
)

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatContext represents a chat's conversation context
type ChatContext struct {
	ChatID       int64
	Messages     []Message
	LastActivity time.Time
	Settings     ChatSettings
}

// ChatSettings represents per-chat settings
type ChatSettings struct {
	ShowThink     bool
	Model         string
	SystemPrompt  string
	Keywords      []string
	MentionWords  []string // 提及词列表
	Language      string
}

// UserSettings represents user-specific settings
type UserSettings struct {
	UserID   int64
	Language string
	Model    string
}

// UserStats represents user statistics
type UserStats struct {
	UserID        int64
	TotalMessages int
	TotalSessions int
}

// User represents a user with rate limiting info
type User struct {
	ID            int64
	RequestCount  int
	LastResetTime time.Time
}

// CacheEntry represents a cached response
type CacheEntry struct {
	Question  string
	Answer    string
	Model     string
	CreatedAt time.Time
}