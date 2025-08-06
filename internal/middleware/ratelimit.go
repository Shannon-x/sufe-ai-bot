package middleware

import (
	"fmt"
	"sync"
	"time"

	"github.com/cf-ai-tgbot-go/internal/config"
	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

// RateLimiter interface for rate limiting
type RateLimiter interface {
	Allow(userID int64) bool
	Reset(userID int64)
}

// UserRateLimiter implements per-user rate limiting
type UserRateLimiter struct {
	enabled   bool
	limiters  map[int64]*rate.Limiter
	mu        sync.RWMutex
	rpm       int
	burst     int
	logger    *logrus.Logger
	cleanupInterval time.Duration
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(cfg *config.Config, logger *logrus.Logger) RateLimiter {
	if !cfg.RateLimit.Enabled {
		return &UserRateLimiter{enabled: false}
	}

	rl := &UserRateLimiter{
		enabled:   true,
		limiters:  make(map[int64]*rate.Limiter),
		rpm:       cfg.RateLimit.RequestsPerMinute,
		burst:     cfg.RateLimit.Burst,
		logger:    logger,
		cleanupInterval: 1 * time.Hour,
	}

	// Start cleanup goroutine
	go rl.cleanup()

	return rl
}

// Allow checks if a user is allowed to make a request
func (r *UserRateLimiter) Allow(userID int64) bool {
	if !r.enabled {
		return true
	}

	limiter := r.getLimiter(userID)
	allowed := limiter.Allow()

	if !allowed {
		r.logger.WithFields(logrus.Fields{
			"user_id": userID,
		}).Warn("Rate limit exceeded")
	}

	return allowed
}

// Reset resets the rate limiter for a user
func (r *UserRateLimiter) Reset(userID int64) {
	if !r.enabled {
		return
	}

	r.mu.Lock()
	delete(r.limiters, userID)
	r.mu.Unlock()
}

// getLimiter gets or creates a rate limiter for a user
func (r *UserRateLimiter) getLimiter(userID int64) *rate.Limiter {
	r.mu.RLock()
	limiter, exists := r.limiters[userID]
	r.mu.RUnlock()

	if exists {
		return limiter
	}

	// Create new limiter
	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check after acquiring write lock
	if limiter, exists := r.limiters[userID]; exists {
		return limiter
	}

	// Rate per second = RPM / 60
	rps := float64(r.rpm) / 60.0
	limiter = rate.NewLimiter(rate.Limit(rps), r.burst)
	r.limiters[userID] = limiter

	return limiter
}

// cleanup removes inactive limiters
func (r *UserRateLimiter) cleanup() {
	ticker := time.NewTicker(r.cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		r.mu.Lock()
		// In production, you might want to track last access time
		// and remove limiters that haven't been used recently
		if len(r.limiters) > 10000 { // Arbitrary threshold
			r.logger.Warn("Rate limiter map size exceeded threshold, clearing")
			r.limiters = make(map[int64]*rate.Limiter)
		}
		r.mu.Unlock()
	}
}

// SecurityMiddleware provides security checks
type SecurityMiddleware struct {
	logger *logrus.Logger
}

// NewSecurityMiddleware creates security middleware
func NewSecurityMiddleware(logger *logrus.Logger) *SecurityMiddleware {
	return &SecurityMiddleware{
		logger: logger,
	}
}

// ValidateInput performs input validation
func (s *SecurityMiddleware) ValidateInput(text string) error {
	// Check message length
	if len(text) > 4096 {
		return fmt.Errorf("message too long: %d bytes", len(text))
	}

	// Add more validation as needed
	// - Check for malicious patterns
	// - Validate encoding
	// - etc.

	return nil
}

// SanitizeOutput sanitizes AI responses
func (s *SecurityMiddleware) SanitizeOutput(text string) string {
	// Remove any potentially harmful content
	// This is a placeholder - implement based on your security requirements
	return text
}