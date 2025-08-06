package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/cf-ai-tgbot-go/internal/config"
	"github.com/cf-ai-tgbot-go/internal/models"
	"github.com/cf-ai-tgbot-go/internal/services/knowledge"
	"github.com/sirupsen/logrus"
)

// Service represents the AI service interface
type Service interface {
	GetResponse(ctx context.Context, messages []models.Message, modelID string) (string, error)
	GetResponseWithKnowledge(ctx context.Context, messages []models.Message, modelID string, knowledgeService knowledge.Service) (string, error)
	GetAvailableModels() []ModelOption
	GetModelByID(modelID string) (*ModelOption, error)
}

// ModelOption represents a model option with endpoint info
type ModelOption struct {
	ID          string
	Name        string
	EndpointName string
	MaxTokens   int
}

// CustomAI implements AI service using custom endpoints
type CustomAI struct {
	config     *config.ModelsConfig
	endpoints  map[string]*config.ModelEndpoint
	models     map[string]*ModelOption
	httpClient *http.Client
	logger     *logrus.Logger
}

// NewCustomAI creates a new custom AI service
func NewCustomAI(cfg *config.ModelsConfig, logger *logrus.Logger) Service {
	endpoints := make(map[string]*config.ModelEndpoint)
	models := make(map[string]*ModelOption)
	
	logger.WithField("endpointCount", len(cfg.Endpoints)).Info("Loading AI endpoints")
	
	// Build lookup maps
	for i := range cfg.Endpoints {
		endpoint := &cfg.Endpoints[i]
		endpoints[endpoint.Name] = endpoint
		
		logger.WithFields(logrus.Fields{
			"endpoint": endpoint.Name,
			"baseURL":  endpoint.BaseURL,
			"models":   len(endpoint.Models),
		}).Info("Loading endpoint")
		
		for j := range endpoint.Models {
			model := &endpoint.Models[j]
			models[model.ID] = &ModelOption{
				ID:           model.ID,
				Name:         model.Name,
				EndpointName: endpoint.Name,
				MaxTokens:    model.MaxTokens,
			}
			
			logger.WithFields(logrus.Fields{
				"modelID": model.ID,
				"modelName": model.Name,
				"endpoint": endpoint.Name,
			}).Debug("Loaded model")
		}
	}
	
	logger.WithField("totalModels", len(models)).Info("AI service initialized")
	
	return &CustomAI{
		config:    cfg,
		endpoints: endpoints,
		models:    models,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		logger: logger,
	}
}

// GetResponse gets AI response from the appropriate endpoint with retry logic
func (s *CustomAI) GetResponse(ctx context.Context, messages []models.Message, modelID string) (string, error) {
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
			// Exponential backoff: 2s, 4s, 8s
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
func (s *CustomAI) getResponseWithRetry(ctx context.Context, messages []models.Message, modelID string, attempt int) (string, error) {
	s.logger.WithFields(logrus.Fields{
		"modelID": modelID,
		"attempt": attempt,
	}).Debug("Getting AI response")
	
	modelOption, err := s.GetModelByID(modelID)
	if err != nil {
		s.logger.WithError(err).WithField("modelID", modelID).Error("Model not found")
		return "", err
	}
	
	endpoint, exists := s.endpoints[modelOption.EndpointName]
	if !exists {
		s.logger.WithField("endpointName", modelOption.EndpointName).Error("Endpoint not found")
		return "", fmt.Errorf("endpoint not found: %s", modelOption.EndpointName)
	}
	
	s.logger.WithFields(logrus.Fields{
		"endpoint": endpoint.Name,
		"baseURL":  endpoint.BaseURL,
		"modelID":  modelID,
		"attempt": attempt,
	}).Debug("Using endpoint")
	
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
	
	// Create HTTP request with a timeout context for this specific attempt
	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	
	url := fmt.Sprintf("%s/chat/completions", strings.TrimSuffix(endpoint.BaseURL, "/"))
	req, err := http.NewRequestWithContext(reqCtx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", endpoint.APIKey))
	
	// Log request
	s.logger.WithFields(logrus.Fields{
		"model":    modelID,
		"endpoint": endpoint.Name,
		"url":      url,
		"attempt":  attempt,
	}).Debug("Sending AI request")
	
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
		s.logger.WithFields(logrus.Fields{
			"status":  resp.StatusCode,
			"body":    string(body),
			"attempt": attempt,
		}).Error("AI request failed")
		
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
func (s *CustomAI) GetAvailableModels() []ModelOption {
	models := make([]ModelOption, 0, len(s.models))
	for _, model := range s.models {
		models = append(models, *model)
	}
	return models
}

// GetModelByID returns a model by its ID
func (s *CustomAI) GetModelByID(modelID string) (*ModelOption, error) {
	model, exists := s.models[modelID]
	if !exists {
		return nil, fmt.Errorf("model not found: %s", modelID)
	}
	return model, nil
}

// GetResponseWithKnowledge gets AI response with knowledge base context
func (s *CustomAI) GetResponseWithKnowledge(ctx context.Context, messages []models.Message, modelID string, knowledgeService knowledge.Service) (string, error) {
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