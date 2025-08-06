package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/cf-ai-tgbot-go/internal/config"
	"github.com/cf-ai-tgbot-go/internal/models"
	"github.com/patrickmn/go-cache"
	"github.com/sirupsen/logrus"
)

// Service defines cache operations
type Service interface {
	Get(ctx context.Context, question, model string) (string, bool)
	Set(ctx context.Context, question, model, answer string) error
	Clear(ctx context.Context) error
}

// Cache implements caching service
type Cache struct {
	enabled bool
	cache   *cache.Cache
	logger  *logrus.Logger
	maxSize int
}

// NewCache creates a new cache service
func NewCache(cfg *config.Config, logger *logrus.Logger) Service {
	if !cfg.Cache.Enabled {
		return &Cache{enabled: false}
	}

	return &Cache{
		enabled: true,
		cache:   cache.New(cfg.Cache.TTL, cfg.Cache.TTL*2),
		logger:  logger,
		maxSize: cfg.Cache.MaxSize,
	}
}

// Get retrieves a cached response
func (c *Cache) Get(ctx context.Context, question, model string) (string, bool) {
	if !c.enabled {
		return "", false
	}

	key := c.generateKey(question, model)
	if val, found := c.cache.Get(key); found {
		entry := val.(*models.CacheEntry)
		c.logger.WithFields(logrus.Fields{
			"question": question,
			"model":    model,
			"age":      time.Since(entry.CreatedAt),
		}).Debug("Cache hit")
		return entry.Answer, true
	}

	return "", false
}

// Set stores a response in cache
func (c *Cache) Set(ctx context.Context, question, model, answer string) error {
	if !c.enabled {
		return nil
	}

	// Check cache size
	if c.cache.ItemCount() >= c.maxSize {
		c.logger.Warn("Cache size limit reached, clearing old entries")
		c.cache.DeleteExpired()
	}

	key := c.generateKey(question, model)
	entry := &models.CacheEntry{
		Question:  question,
		Answer:    answer,
		Model:     model,
		CreatedAt: time.Now(),
	}

	c.cache.SetDefault(key, entry)
	c.logger.WithFields(logrus.Fields{
		"question": question,
		"model":    model,
	}).Debug("Response cached")

	return nil
}

// Clear removes all cached entries
func (c *Cache) Clear(ctx context.Context) error {
	if !c.enabled {
		return nil
	}

	c.cache.Flush()
	c.logger.Info("Cache cleared")
	return nil
}

// generateKey creates a unique cache key
func (c *Cache) generateKey(question, model string) string {
	data := fmt.Sprintf("%s:%s", model, question)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}