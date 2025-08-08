package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cf-ai-tgbot-go/internal/config"
	"github.com/cf-ai-tgbot-go/internal/models"
	"github.com/go-redis/redis/v8"
	"github.com/patrickmn/go-cache"
	"github.com/sirupsen/logrus"
)

// Storage interface defines storage operations
type Storage interface {
	// Context operations
	GetContext(ctx context.Context, chatID int64) (*models.ChatContext, error)
	SaveContext(ctx context.Context, chatCtx *models.ChatContext) error
	DeleteContext(ctx context.Context, chatID int64) error
	ClearContext(ctx context.Context, userID int64) error
	
	// Settings operations
	GetSettings(ctx context.Context, chatID int64) (*models.ChatSettings, error)
	SaveSettings(ctx context.Context, chatID int64, settings *models.ChatSettings) error
	
	// User settings operations
	GetUserSettings(ctx context.Context, userID int64) (*models.UserSettings, error)
	SaveUserSettings(ctx context.Context, userID int64, settings *models.UserSettings) error
	
	// User stats operations
	GetUserStats(ctx context.Context, userID int64) (*models.UserStats, error)
	IncrementUserStats(ctx context.Context, userID int64) error
	
	// User state operations
	GetUserState(ctx context.Context, userID int64, key string) (string, error)
	SetUserState(ctx context.Context, userID int64, key string, value string) error
	DeleteUserState(ctx context.Context, userID int64, key string) error
	
	// Cleanup operations
	CleanupExpiredContexts(ctx context.Context, expiration time.Duration) error
}

// Manager manages different storage backends
type Manager struct {
	storage Storage
	logger  *logrus.Logger
	redisClient *redis.Client // Store redis client reference
}

// NewManager creates a new storage manager
func NewManager(cfg *config.Config, logger *logrus.Logger) (*Manager, error) {
	var storage Storage
	
	manager := &Manager{
		storage: storage,
		logger:  logger,
	}
	
	switch cfg.Storage.Type {
	case "redis":
		redisStorage, err := NewRedisStorage(cfg, logger)
		if err != nil {
			return nil, err
		}
		storage = redisStorage
		// Store redis client reference
		manager.redisClient = redisStorage.client
	case "memory":
		storage = NewMemoryStorage(cfg, logger)
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", cfg.Storage.Type)
	}

	manager.storage = storage

	// Start cleanup goroutine
	go manager.startCleanup(cfg.Storage.Memory.CleanupInterval, cfg.Storage.Memory.DefaultExpiration)

	return manager, nil
}

func (m *Manager) startCleanup(interval, expiration time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		if err := m.storage.CleanupExpiredContexts(ctx, expiration); err != nil {
			m.logger.WithError(err).Error("Failed to cleanup expired contexts")
		}
		cancel()
	}
}

// Delegate methods to underlying storage
func (m *Manager) GetContext(ctx context.Context, chatID int64) (*models.ChatContext, error) {
	return m.storage.GetContext(ctx, chatID)
}

func (m *Manager) SaveContext(ctx context.Context, chatCtx *models.ChatContext) error {
	return m.storage.SaveContext(ctx, chatCtx)
}

func (m *Manager) DeleteContext(ctx context.Context, chatID int64) error {
	return m.storage.DeleteContext(ctx, chatID)
}

func (m *Manager) GetSettings(ctx context.Context, chatID int64) (*models.ChatSettings, error) {
	return m.storage.GetSettings(ctx, chatID)
}

func (m *Manager) SaveSettings(ctx context.Context, chatID int64, settings *models.ChatSettings) error {
	return m.storage.SaveSettings(ctx, chatID, settings)
}

func (m *Manager) ClearContext(ctx context.Context, userID int64) error {
	return m.storage.ClearContext(ctx, userID)
}

func (m *Manager) GetUserSettings(ctx context.Context, userID int64) (*models.UserSettings, error) {
	return m.storage.GetUserSettings(ctx, userID)
}

func (m *Manager) SaveUserSettings(ctx context.Context, userID int64, settings *models.UserSettings) error {
	return m.storage.SaveUserSettings(ctx, userID, settings)
}

func (m *Manager) GetUserStats(ctx context.Context, userID int64) (*models.UserStats, error) {
	return m.storage.GetUserStats(ctx, userID)
}

func (m *Manager) IncrementUserStats(ctx context.Context, userID int64) error {
	return m.storage.IncrementUserStats(ctx, userID)
}

func (m *Manager) GetUserState(ctx context.Context, userID int64, key string) (string, error) {
	return m.storage.GetUserState(ctx, userID, key)
}

func (m *Manager) SetUserState(ctx context.Context, userID int64, key string, value string) error {
	return m.storage.SetUserState(ctx, userID, key, value)
}

func (m *Manager) DeleteUserState(ctx context.Context, userID int64, key string) error {
	return m.storage.DeleteUserState(ctx, userID, key)
}

// GetRedisClient returns the Redis client if available
func (m *Manager) GetRedisClient() *redis.Client {
	return m.redisClient
}

// RedisStorage implements storage using Redis
type RedisStorage struct {
	client *redis.Client
	logger *logrus.Logger
}

func NewRedisStorage(cfg *config.Config, logger *logrus.Logger) (*RedisStorage, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Storage.Redis.Addr,
		Password: cfg.Storage.Redis.Password,
		DB:       cfg.Storage.Redis.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return &RedisStorage{
		client: client,
		logger: logger,
	}, nil
}

func (r *RedisStorage) GetContext(ctx context.Context, chatID int64) (*models.ChatContext, error) {
	key := fmt.Sprintf("context:%d", chatID)
	data, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var chatCtx models.ChatContext
	if err := json.Unmarshal([]byte(data), &chatCtx); err != nil {
		return nil, err
	}

	return &chatCtx, nil
}

func (r *RedisStorage) SaveContext(ctx context.Context, chatCtx *models.ChatContext) error {
	key := fmt.Sprintf("context:%d", chatCtx.ChatID)
	data, err := json.Marshal(chatCtx)
	if err != nil {
		return err
	}

	return r.client.Set(ctx, key, data, 24*time.Hour).Err()
}

func (r *RedisStorage) DeleteContext(ctx context.Context, chatID int64) error {
	key := fmt.Sprintf("context:%d", chatID)
	return r.client.Del(ctx, key).Err()
}

func (r *RedisStorage) GetSettings(ctx context.Context, chatID int64) (*models.ChatSettings, error) {
	key := fmt.Sprintf("settings:%d", chatID)
	data, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var settings models.ChatSettings
	if err := json.Unmarshal([]byte(data), &settings); err != nil {
		return nil, err
	}

	return &settings, nil
}

func (r *RedisStorage) SaveSettings(ctx context.Context, chatID int64, settings *models.ChatSettings) error {
	key := fmt.Sprintf("settings:%d", chatID)
	data, err := json.Marshal(settings)
	if err != nil {
		return err
	}

	return r.client.Set(ctx, key, data, 0).Err() // No expiration for settings
}

func (r *RedisStorage) CleanupExpiredContexts(ctx context.Context, expiration time.Duration) error {
	// Redis handles expiration automatically
	return nil
}

func (r *RedisStorage) ClearContext(ctx context.Context, userID int64) error {
	// Note: This is a simplified implementation
	// In a real app, you might want to track user->chat associations
	key := fmt.Sprintf("context:%d", userID)
	return r.client.Del(ctx, key).Err()
}

func (r *RedisStorage) GetUserSettings(ctx context.Context, userID int64) (*models.UserSettings, error) {
	key := fmt.Sprintf("user_settings:%d", userID)
	data, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var settings models.UserSettings
	if err := json.Unmarshal([]byte(data), &settings); err != nil {
		return nil, err
	}

	return &settings, nil
}

func (r *RedisStorage) SaveUserSettings(ctx context.Context, userID int64, settings *models.UserSettings) error {
	key := fmt.Sprintf("user_settings:%d", userID)
	data, err := json.Marshal(settings)
	if err != nil {
		return err
	}

	return r.client.Set(ctx, key, data, 0).Err()
}

func (r *RedisStorage) GetUserStats(ctx context.Context, userID int64) (*models.UserStats, error) {
	key := fmt.Sprintf("user_stats:%d", userID)
	data, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return &models.UserStats{UserID: userID}, nil
	}
	if err != nil {
		return nil, err
	}

	var stats models.UserStats
	if err := json.Unmarshal([]byte(data), &stats); err != nil {
		return nil, err
	}

	return &stats, nil
}

func (r *RedisStorage) IncrementUserStats(ctx context.Context, userID int64) error {
	stats, err := r.GetUserStats(ctx, userID)
	if err != nil {
		return err
	}

	stats.TotalMessages++
	key := fmt.Sprintf("user_stats:%d", userID)
	data, err := json.Marshal(stats)
	if err != nil {
		return err
	}

	return r.client.Set(ctx, key, data, 0).Err()
}

func (r *RedisStorage) GetUserState(ctx context.Context, userID int64, key string) (string, error) {
	stateKey := fmt.Sprintf("user_state:%d:%s", userID, key)
	value, err := r.client.Get(ctx, stateKey).Result()
	if err == redis.Nil {
		return "", nil
	}
	return value, err
}

func (r *RedisStorage) SetUserState(ctx context.Context, userID int64, key string, value string) error {
	stateKey := fmt.Sprintf("user_state:%d:%s", userID, key)
	// Set with 1 hour expiration for state data
	return r.client.Set(ctx, stateKey, value, time.Hour).Err()
}

func (r *RedisStorage) DeleteUserState(ctx context.Context, userID int64, key string) error {
	stateKey := fmt.Sprintf("user_state:%d:%s", userID, key)
	return r.client.Del(ctx, stateKey).Err()
}

// MemoryStorage implements storage using in-memory cache
type MemoryStorage struct {
	contexts     *cache.Cache
	settings     *cache.Cache
	userSettings *cache.Cache
	userStats    *cache.Cache
	userStates   *cache.Cache
	logger       *logrus.Logger
}

func NewMemoryStorage(cfg *config.Config, logger *logrus.Logger) *MemoryStorage {
	return &MemoryStorage{
		contexts:     cache.New(cfg.Storage.Memory.DefaultExpiration, cfg.Storage.Memory.CleanupInterval),
		settings:     cache.New(cache.NoExpiration, cache.NoExpiration),
		userSettings: cache.New(cache.NoExpiration, cache.NoExpiration),
		userStats:    cache.New(cache.NoExpiration, cache.NoExpiration),
		userStates:   cache.New(time.Hour, 10*time.Minute),
		logger:       logger,
	}
}

func (m *MemoryStorage) GetContext(ctx context.Context, chatID int64) (*models.ChatContext, error) {
	key := fmt.Sprintf("context:%d", chatID)
	if val, found := m.contexts.Get(key); found {
		return val.(*models.ChatContext), nil
	}
	return nil, nil
}

func (m *MemoryStorage) SaveContext(ctx context.Context, chatCtx *models.ChatContext) error {
	key := fmt.Sprintf("context:%d", chatCtx.ChatID)
	m.contexts.SetDefault(key, chatCtx)
	return nil
}

func (m *MemoryStorage) DeleteContext(ctx context.Context, chatID int64) error {
	key := fmt.Sprintf("context:%d", chatID)
	m.contexts.Delete(key)
	return nil
}

func (m *MemoryStorage) GetSettings(ctx context.Context, chatID int64) (*models.ChatSettings, error) {
	key := fmt.Sprintf("settings:%d", chatID)
	if val, found := m.settings.Get(key); found {
		return val.(*models.ChatSettings), nil
	}
	return nil, nil
}

func (m *MemoryStorage) SaveSettings(ctx context.Context, chatID int64, settings *models.ChatSettings) error {
	key := fmt.Sprintf("settings:%d", chatID)
	m.settings.Set(key, settings, cache.NoExpiration)
	return nil
}

func (m *MemoryStorage) CleanupExpiredContexts(ctx context.Context, expiration time.Duration) error {
	// go-cache handles cleanup automatically
	return nil
}

func (m *MemoryStorage) ClearContext(ctx context.Context, userID int64) error {
	key := fmt.Sprintf("context:%d", userID)
	m.contexts.Delete(key)
	return nil
}

func (m *MemoryStorage) GetUserSettings(ctx context.Context, userID int64) (*models.UserSettings, error) {
	key := fmt.Sprintf("user_settings:%d", userID)
	if val, found := m.userSettings.Get(key); found {
		return val.(*models.UserSettings), nil
	}
	return nil, nil
}

func (m *MemoryStorage) SaveUserSettings(ctx context.Context, userID int64, settings *models.UserSettings) error {
	key := fmt.Sprintf("user_settings:%d", userID)
	m.userSettings.Set(key, settings, cache.NoExpiration)
	return nil
}

func (m *MemoryStorage) GetUserStats(ctx context.Context, userID int64) (*models.UserStats, error) {
	key := fmt.Sprintf("user_stats:%d", userID)
	if val, found := m.userStats.Get(key); found {
		return val.(*models.UserStats), nil
	}
	return &models.UserStats{UserID: userID}, nil
}

func (m *MemoryStorage) IncrementUserStats(ctx context.Context, userID int64) error {
	stats, err := m.GetUserStats(ctx, userID)
	if err != nil {
		return err
	}
	
	stats.TotalMessages++
	key := fmt.Sprintf("user_stats:%d", userID)
	m.userStats.Set(key, stats, cache.NoExpiration)
	return nil
}

func (m *MemoryStorage) GetUserState(ctx context.Context, userID int64, key string) (string, error) {
	stateKey := fmt.Sprintf("user_state:%d:%s", userID, key)
	if val, found := m.userStates.Get(stateKey); found {
		return val.(string), nil
	}
	return "", nil
}

func (m *MemoryStorage) SetUserState(ctx context.Context, userID int64, key string, value string) error {
	stateKey := fmt.Sprintf("user_state:%d:%s", userID, key)
	m.userStates.SetDefault(stateKey, value)
	return nil
}

func (m *MemoryStorage) DeleteUserState(ctx context.Context, userID int64, key string) error {
	stateKey := fmt.Sprintf("user_state:%d:%s", userID, key)
	m.userStates.Delete(stateKey)
	return nil
}