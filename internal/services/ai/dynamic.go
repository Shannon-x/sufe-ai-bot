package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/cf-ai-tgbot-go/internal/config"
	"github.com/cf-ai-tgbot-go/internal/models"
	"github.com/cf-ai-tgbot-go/internal/services/knowledge"
	dynamicconfig "github.com/cf-ai-tgbot-go/internal/services/config"
	"github.com/sirupsen/logrus"
)

// DynamicAI implements AI service with dynamic configuration support
type DynamicAI struct {
	configService    *dynamicconfig.DynamicConfigService
	httpClient       *http.Client
	logger           *logrus.Logger
	mu               sync.RWMutex
	cachedEndpoints  map[string]*config.ModelEndpoint
	cachedModels     map[string]*ModelOption
}

// NewDynamicAI creates a new dynamic AI service
func NewDynamicAI(configService *dynamicconfig.DynamicConfigService, logger *logrus.Logger) Service {
	ai := &DynamicAI{
		configService:   configService,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		logger:          logger,
		cachedEndpoints: make(map[string]*config.ModelEndpoint),
		cachedModels:    make(map[string]*ModelOption),
	}

	// Register config change listener
	configService.RegisterConfigChangeListener(func(cfg *config.Config) {
		ai.updateCache(cfg)
	})

	// Initial cache update
	ctx := context.Background()
	if cfg, err := configService.GetCurrentConfig(ctx); err == nil {
		ai.updateCache(cfg)
	}

	return ai
}

// updateCache updates the internal cache when config changes
func (s *DynamicAI) updateCache(cfg *config.Config) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clear existing cache
	s.cachedEndpoints = make(map[string]*config.ModelEndpoint)
	s.cachedModels = make(map[string]*ModelOption)

	// Rebuild cache
	for i := range cfg.Models.Endpoints {
		endpoint := &cfg.Models.Endpoints[i]
		s.cachedEndpoints[endpoint.Name] = endpoint

		for j := range endpoint.Models {
			model := &endpoint.Models[j]
			s.cachedModels[model.ID] = &ModelOption{
				ID:           model.ID,
				Name:         model.Name,
				EndpointName: endpoint.Name,
				MaxTokens:    model.MaxTokens,
			}
		}
	}

	s.logger.WithFields(logrus.Fields{
		"endpoints": len(s.cachedEndpoints),
		"models":    len(s.cachedModels),
	}).Info("AI service cache updated")
}

// GetResponse gets AI response with retry logic
func (s *DynamicAI) GetResponse(ctx context.Context, messages []models.Message, modelID string) (string, error) {
	const maxRetries = 3
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		response, err := s.getResponseWithRetry(ctx, messages, modelID, attempt)
		if err == nil {
			return response, nil
		}

		lastErr = err
		s.logger.WithFields(logrus.Fields{
			"attempt": attempt,
			"error":   err.Error(),
			"modelID": modelID,
		}).Warn("AI request failed, retrying...")

		if attempt < maxRetries {
			waitTime := time.Duration(2<<uint(attempt-1)) * time.Second
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(waitTime):
				// Continue to next retry
			}
		}
	}

	return "", fmt.Errorf("all retry attempts failed: %w", lastErr)
}

// getResponseWithRetry performs a single request attempt
func (s *DynamicAI) getResponseWithRetry(ctx context.Context, messages []models.Message, modelID string, attempt int) (string, error) {
	s.mu.RLock()
	modelOption, exists := s.cachedModels[modelID]
	if !exists {
		s.mu.RUnlock()
		return "", fmt.Errorf("model not found: %s", modelID)
	}

	endpoint, exists := s.cachedEndpoints[modelOption.EndpointName]
	if !exists {
		s.mu.RUnlock()
		return "", fmt.Errorf("endpoint not found: %s", modelOption.EndpointName)
	}
	s.mu.RUnlock()

	// Convert messages to OpenAI format
	openAIMessages := make([]map[string]string, len(messages))
	for i, msg := range messages {
		openAIMessages[i] = map[string]string{
			"role":    msg.Role,
			"content": msg.Content,
		}
	}

	// Build request
	reqBody := map[string]interface{}{
		"model":       modelID,
		"messages":    openAIMessages,
		"max_tokens":  modelOption.MaxTokens,
		"temperature": 0.7,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := fmt.Sprintf("%s/chat/completions", strings.TrimSuffix(endpoint.BaseURL, "/"))
	req, err := http.NewRequestWithContext(reqCtx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", endpoint.APIKey))

	// Send request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// Don't retry for client errors (4xx)
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return "", fmt.Errorf("AI request failed with client error %d: %s", resp.StatusCode, string(body))
		}
		return "", fmt.Errorf("AI request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Error.Message != "" {
		return "", fmt.Errorf("AI error: %s", result.Error.Message)
	}

	if len(result.Choices) == 0 || result.Choices[0].Message.Content == "" {
		return "", fmt.Errorf("no response from AI")
	}

	return result.Choices[0].Message.Content, nil
}

// GetAvailableModels returns all available models
func (s *DynamicAI) GetAvailableModels() []ModelOption {
	s.mu.RLock()
	defer s.mu.RUnlock()

	models := make([]ModelOption, 0, len(s.cachedModels))
	for _, model := range s.cachedModels {
		models = append(models, *model)
	}
	return models
}

// GetModelByID returns a model by its ID
func (s *DynamicAI) GetModelByID(modelID string) (*ModelOption, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	model, exists := s.cachedModels[modelID]
	if !exists {
		return nil, fmt.Errorf("model not found: %s", modelID)
	}
	return model, nil
}

// GetResponseWithKnowledge gets AI response with knowledge base context
func (s *DynamicAI) GetResponseWithKnowledge(ctx context.Context, messages []models.Message, modelID string, knowledgeService knowledge.Service) (string, error) {
	// Extract user's query from the last message
	if len(messages) == 0 {
		return s.GetResponse(ctx, messages, modelID)
	}

	userQuery := ""
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			userQuery = messages[i].Content
			break
		}
	}

	if userQuery == "" || knowledgeService == nil {
		return s.GetResponse(ctx, messages, modelID)
	}

	// Search knowledge base
	s.logger.WithField("query", userQuery).Debug("Searching knowledge base")
	relevantDocs, err := knowledgeService.SearchDocuments(ctx, userQuery, 3)
	if err != nil {
		s.logger.WithError(err).Warn("Failed to search knowledge base")
		return s.GetResponse(ctx, messages, modelID)
	}

	// If no relevant documents found, proceed without knowledge
	if len(relevantDocs) == 0 {
		s.logger.Debug("No relevant documents found in knowledge base")
		return s.GetResponse(ctx, messages, modelID)
	}

	// Build knowledge context
	var knowledgeContext strings.Builder
	knowledgeContext.WriteString("根据知识库中的相关信息：\n\n")

	for i, doc := range relevantDocs {
		knowledgeContext.WriteString(fmt.Sprintf("【文档 %d: %s】\n", i+1, doc.Title))

		// Include relevant sections
		content := doc.Content
		if len(content) > 1000 {
			// Truncate long content
			content = content[:1000] + "..."
		}
		knowledgeContext.WriteString(content)
		knowledgeContext.WriteString("\n\n")
	}

	knowledgeContext.WriteString("请基于以上知识库信息和对话历史回答用户的问题。如果知识库中没有相关信息，请根据你的知识回答。\n\n")

	// Create modified messages with knowledge context
	modifiedMessages := make([]models.Message, 0, len(messages)+1)

	// Keep system message if exists
	if len(messages) > 0 && messages[0].Role == "system" {
		modifiedMessages = append(modifiedMessages, messages[0])
		modifiedMessages = append(modifiedMessages, models.Message{
			Role:    "system",
			Content: knowledgeContext.String(),
		})
		modifiedMessages = append(modifiedMessages, messages[1:]...)
	} else {
		modifiedMessages = append(modifiedMessages, models.Message{
			Role:    "system",
			Content: knowledgeContext.String(),
		})
		modifiedMessages = append(modifiedMessages, messages...)
	}

	s.logger.WithFields(logrus.Fields{
		"docsFound": len(relevantDocs),
		"modelID":   modelID,
	}).Info("Sending request with knowledge context")

	return s.GetResponse(ctx, modifiedMessages, modelID)
}