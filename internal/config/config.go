package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Bot        BotConfig        `mapstructure:"bot"`
	Models     ModelsConfig     `mapstructure:"models"`
	Storage    StorageConfig    `mapstructure:"storage"`
	Cache      CacheConfig      `mapstructure:"cache"`
	RateLimit  RateLimitConfig  `mapstructure:"rate_limit"`
	Context    ContextConfig    `mapstructure:"context"`
	Logging    LoggingConfig    `mapstructure:"logging"`
	Monitoring MonitoringConfig `mapstructure:"monitoring"`
	I18n       I18nConfig       `mapstructure:"i18n"`
	Knowledge  KnowledgeConfig  `mapstructure:"knowledge"`
}

type BotConfig struct {
	Token   string        `mapstructure:"token"`
	Webhook WebhookConfig `mapstructure:"webhook"`
	UpdateTimeout int    `mapstructure:"update_timeout"`
}

type WebhookConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	URL     string `mapstructure:"url"`
	Port    int    `mapstructure:"port"`
}


type ModelsConfig struct {
	Default   string           `mapstructure:"default"`
	Endpoints []ModelEndpoint  `mapstructure:"endpoints"`
}

type ModelEndpoint struct {
	Name        string       `mapstructure:"name"`
	DisplayName string       `mapstructure:"display_name"`
	BaseURL     string       `mapstructure:"base_url"`
	APIKey      string       `mapstructure:"api_key"`
	Models      []ModelInfo  `mapstructure:"models"`
}

type ModelInfo struct {
	ID        string `mapstructure:"id"`
	Name      string `mapstructure:"name"`
	MaxTokens int    `mapstructure:"max_tokens"`
}

type StorageConfig struct {
	Type   string       `mapstructure:"type"`
	Redis  RedisConfig  `mapstructure:"redis"`
	Memory MemoryConfig `mapstructure:"memory"`
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type MemoryConfig struct {
	DefaultExpiration time.Duration `mapstructure:"default_expiration"`
	CleanupInterval   time.Duration `mapstructure:"cleanup_interval"`
}

type CacheConfig struct {
	Enabled bool          `mapstructure:"enabled"`
	TTL     time.Duration `mapstructure:"ttl"`
	MaxSize int           `mapstructure:"max_size"`
}

type RateLimitConfig struct {
	Enabled            bool `mapstructure:"enabled"`
	RequestsPerMinute  int  `mapstructure:"requests_per_minute"`
	Burst              int  `mapstructure:"burst"`
}

type ContextConfig struct {
	MaxMessages         int      `mapstructure:"max_messages"`
	DefaultSystemPrompt string   `mapstructure:"default_system_prompt"`
	DefaultMentionWords []string `mapstructure:"default_mention_words"`
	BotPersonality      string   `mapstructure:"bot_personality"`
}

type LoggingConfig struct {
	Level  string     `mapstructure:"level"`
	Format string     `mapstructure:"format"`
	Output string     `mapstructure:"output"`
	File   FileConfig `mapstructure:"file"`
}

type FileConfig struct {
	Path       string `mapstructure:"path"`
	MaxSize    int    `mapstructure:"max_size"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAge     int    `mapstructure:"max_age"`
}

type MonitoringConfig struct {
	Metrics MetricsConfig `mapstructure:"metrics"`
}

type MetricsConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Port    int    `mapstructure:"port"`
	Path    string `mapstructure:"path"`
}

type I18nConfig struct {
	DefaultLanguage string   `mapstructure:"default_language"`
	Languages       []string `mapstructure:"languages"`
}

type KnowledgeConfig struct {
	Enabled   bool   `mapstructure:"enabled"`
	Directory string `mapstructure:"directory"`
}

// LoadConfig loads configuration from file and environment variables
func LoadConfig(configPath string) (*Config, error) {
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")
	
	// Enable environment variable substitution
	viper.AutomaticEnv()
	
	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	
	// Set environment variable overrides
	viper.SetEnvPrefix("") // No prefix
	viper.BindEnv("bot.token", "BOT_TOKEN")
	viper.BindEnv("storage.redis.addr", "REDIS_HOST", "REDIS_PORT")
	viper.BindEnv("storage.redis.password", "REDIS_PASSWORD")
	viper.BindEnv("storage.redis.db", "REDIS_DB")
	
	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	
	// Handle Redis address special case
	if redisHost := viper.GetString("REDIS_HOST"); redisHost != "" {
		redisPort := viper.GetString("REDIS_PORT")
		if redisPort == "" {
			redisPort = "6379"
		}
		config.Storage.Redis.Addr = fmt.Sprintf("%s:%s", redisHost, redisPort)
	}
	
	// Load custom endpoints from environment variables
	if customEndpoints := os.Getenv("CUSTOM_ENDPOINTS"); customEndpoints != "" {
		endpoints := strings.Split(customEndpoints, ",")
		for _, endpointName := range endpoints {
			endpointName = strings.TrimSpace(endpointName)
			if endpointName == "" {
				continue
			}
			
			// Convert endpoint name to env var prefix
			envPrefix := strings.ToUpper(strings.ReplaceAll(endpointName, "-", "_"))
			
			// Get endpoint configuration from env vars
			baseURL := os.Getenv(envPrefix + "_BASE_URL")
			apiKey := os.Getenv(envPrefix + "_API_KEY")
			modelsStr := os.Getenv(envPrefix + "_MODELS")
			
			if baseURL == "" || apiKey == "" {
				continue
			}
			
			// Create new endpoint
			endpoint := ModelEndpoint{
				Name:        endpointName,
				DisplayName: endpointName,
				BaseURL:     baseURL,
				APIKey:      apiKey,
				Models:      []ModelInfo{},
			}
			
			// Parse models
			if modelsStr != "" {
				modelList := strings.Split(modelsStr, ",")
				for _, modelStr := range modelList {
					modelStr = strings.TrimSpace(modelStr)
					if modelStr == "" {
						continue
					}
					
					// Check if model has display name
					parts := strings.SplitN(modelStr, ":", 2)
					modelID := parts[0]
					modelName := modelID
					if len(parts) == 2 {
						modelName = parts[1]
					}
					
					endpoint.Models = append(endpoint.Models, ModelInfo{
						ID:   modelID,
						Name: modelName,
					})
				}
			}
			
			// Add endpoint to config
			config.Models.Endpoints = append(config.Models.Endpoints, endpoint)
		}
	}
	
	// Validate required fields
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}
	
	return &config, nil
}

func validateConfig(cfg *Config) error {
	if cfg.Bot.Token == "" {
		return fmt.Errorf("bot token is required")
	}
	if len(cfg.Models.Endpoints) == 0 {
		return fmt.Errorf("at least one model endpoint is required")
	}
	return nil
}