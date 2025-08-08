package config

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/cf-ai-tgbot-go/internal/config"
	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
)

// DynamicConfigService manages runtime configuration changes
type DynamicConfigService struct {
	redis      *redis.Client
	baseConfig *config.Config
	logger     *logrus.Logger
	mu         sync.RWMutex
	listeners  []func(*config.Config)
}

// NewDynamicConfigService creates a new dynamic config service
func NewDynamicConfigService(redisClient *redis.Client, baseConfig *config.Config, logger *logrus.Logger) *DynamicConfigService {
	return &DynamicConfigService{
		redis:      redisClient,
		baseConfig: baseConfig,
		logger:     logger,
		listeners:  make([]func(*config.Config), 0),
	}
}

// GetCurrentConfig returns the current configuration with dynamic updates
func (s *DynamicConfigService) GetCurrentConfig(ctx context.Context) (*config.Config, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Get dynamic endpoints from Redis
	dynamicEndpoints, err := s.getDynamicEndpoints(ctx)
	if err != nil {
		s.logger.WithError(err).Warn("Failed to get dynamic endpoints, using base config")
		return s.baseConfig, nil
	}

	// Create a copy of base config
	currentConfig := *s.baseConfig
	
	// Merge dynamic endpoints with base endpoints
	if len(dynamicEndpoints) > 0 {
		// Combine base endpoints with dynamic ones
		currentConfig.Models.Endpoints = append(s.baseConfig.Models.Endpoints, dynamicEndpoints...)
	}

	return &currentConfig, nil
}

// AddEndpoint adds a new endpoint dynamically
func (s *DynamicConfigService) AddEndpoint(ctx context.Context, endpoint *config.ModelEndpoint) error {
	// Validate endpoint
	if err := s.validateEndpoint(endpoint); err != nil {
		return fmt.Errorf("invalid endpoint: %w", err)
	}

	// Get existing dynamic endpoints
	endpoints, err := s.getDynamicEndpoints(ctx)
	if err != nil && err != redis.Nil {
		return err
	}

	// Check for duplicate
	for _, ep := range endpoints {
		if ep.Name == endpoint.Name {
			return fmt.Errorf("endpoint with name '%s' already exists", endpoint.Name)
		}
	}

	// Add new endpoint
	endpoints = append(endpoints, *endpoint)

	// Save to Redis
	if err := s.saveDynamicEndpoints(ctx, endpoints); err != nil {
		return err
	}

	// Notify listeners
	s.notifyConfigChange()

	s.logger.WithField("endpoint", endpoint.Name).Info("Added new endpoint")
	return nil
}

// UpdateEndpoint updates an existing endpoint
func (s *DynamicConfigService) UpdateEndpoint(ctx context.Context, endpointName string, updates map[string]interface{}) error {
	endpoints, err := s.getDynamicEndpoints(ctx)
	if err != nil && err != redis.Nil {
		return err
	}

	found := false
	for i := range endpoints {
		if endpoints[i].Name == endpointName {
			found = true
			
			// Apply updates
			if displayName, ok := updates["display_name"].(string); ok {
				endpoints[i].DisplayName = displayName
			}
			if baseURL, ok := updates["base_url"].(string); ok {
				endpoints[i].BaseURL = baseURL
			}
			if apiKey, ok := updates["api_key"].(string); ok {
				endpoints[i].APIKey = apiKey
			}
			
			break
		}
	}

	if !found {
		return fmt.Errorf("endpoint '%s' not found", endpointName)
	}

	// Save updated endpoints
	if err := s.saveDynamicEndpoints(ctx, endpoints); err != nil {
		return err
	}

	// Notify listeners
	s.notifyConfigChange()

	return nil
}

// AddModelToEndpoint adds a model to an endpoint
func (s *DynamicConfigService) AddModelToEndpoint(ctx context.Context, endpointName string, model config.ModelInfo) error {
	endpoints, err := s.getDynamicEndpoints(ctx)
	if err != nil && err != redis.Nil {
		return err
	}

	found := false
	for i := range endpoints {
		if endpoints[i].Name == endpointName {
			found = true
			
			// Check for duplicate model
			for _, m := range endpoints[i].Models {
				if m.ID == model.ID {
					return fmt.Errorf("model '%s' already exists in endpoint", model.ID)
				}
			}
			
			// Add model
			endpoints[i].Models = append(endpoints[i].Models, model)
			break
		}
	}

	if !found {
		// Check base config endpoints
		for i := range s.baseConfig.Models.Endpoints {
			if s.baseConfig.Models.Endpoints[i].Name == endpointName {
				// Create a dynamic copy of the base endpoint
				dynamicEndpoint := s.baseConfig.Models.Endpoints[i]
				dynamicEndpoint.Models = append(dynamicEndpoint.Models, model)
				endpoints = append(endpoints, dynamicEndpoint)
				found = true
				break
			}
		}
	}

	if !found {
		return fmt.Errorf("endpoint '%s' not found", endpointName)
	}

	// Save updated endpoints
	if err := s.saveDynamicEndpoints(ctx, endpoints); err != nil {
		return err
	}

	// Notify listeners
	s.notifyConfigChange()

	return nil
}

// TestEndpoint tests if an endpoint is working
func (s *DynamicConfigService) TestEndpoint(ctx context.Context, endpoint *config.ModelEndpoint) error {
	// TODO: Implement endpoint testing
	// This would make a test API call to verify the endpoint is working
	return nil
}

// RegisterConfigChangeListener registers a callback for config changes
func (s *DynamicConfigService) RegisterConfigChangeListener(listener func(*config.Config)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listeners = append(s.listeners, listener)
}

// Private methods

func (s *DynamicConfigService) getDynamicEndpoints(ctx context.Context) ([]config.ModelEndpoint, error) {
	data, err := s.redis.Get(ctx, "dynamic_endpoints").Result()
	if err == redis.Nil {
		return []config.ModelEndpoint{}, nil
	}
	if err != nil {
		return nil, err
	}

	var endpoints []config.ModelEndpoint
	if err := json.Unmarshal([]byte(data), &endpoints); err != nil {
		return nil, err
	}

	return endpoints, nil
}

func (s *DynamicConfigService) saveDynamicEndpoints(ctx context.Context, endpoints []config.ModelEndpoint) error {
	data, err := json.Marshal(endpoints)
	if err != nil {
		return err
	}

	return s.redis.Set(ctx, "dynamic_endpoints", data, 0).Err()
}

func (s *DynamicConfigService) validateEndpoint(endpoint *config.ModelEndpoint) error {
	if endpoint.Name == "" {
		return fmt.Errorf("endpoint name is required")
	}
	if endpoint.DisplayName == "" {
		return fmt.Errorf("display name is required")
	}
	if endpoint.BaseURL == "" {
		return fmt.Errorf("base URL is required")
	}
	if endpoint.APIKey == "" {
		return fmt.Errorf("API key is required")
	}
	return nil
}

func (s *DynamicConfigService) notifyConfigChange() {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	// Get current config
	ctx := context.Background()
	config, err := s.GetCurrentConfig(ctx)
	if err != nil {
		s.logger.WithError(err).Error("Failed to get config for notification")
		return
	}
	
	// Notify all listeners
	for _, listener := range s.listeners {
		go listener(config)
	}
}

// UserEndpoint represents a user-defined endpoint
type UserEndpoint struct {
	Name        string              `json:"name"`
	DisplayName string              `json:"display_name"`
	BaseURL     string              `json:"base_url"`
	APIKey      string              `json:"api_key"`
	Models      []UserModel         `json:"models"`
	UserID      int64               `json:"user_id"`
	CreatedAt   int64               `json:"created_at"`
}

// UserModel represents a user-defined model
type UserModel struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	MaxTokens int    `json:"max_tokens"`
}