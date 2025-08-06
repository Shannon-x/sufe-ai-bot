package i18n

import (
	"encoding/json"
	"fmt"

	"github.com/cf-ai-tgbot-go/internal/config"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

// Localizer manages internationalization
type Localizer struct {
	bundle          *i18n.Bundle
	defaultLanguage string
	localizers      map[string]*i18n.Localizer
}

// NewLocalizer creates a new localizer
func NewLocalizer(cfg *config.I18nConfig) (*Localizer, error) {
	bundle := i18n.NewBundle(language.Chinese)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)

	// Load language files
	for _, lang := range cfg.Languages {
		if _, err := bundle.LoadMessageFile(fmt.Sprintf("configs/i18n/%s.json", lang)); err != nil {
			return nil, fmt.Errorf("failed to load language file %s: %w", lang, err)
		}
	}

	localizers := make(map[string]*i18n.Localizer)
	for _, lang := range cfg.Languages {
		localizers[lang] = i18n.NewLocalizer(bundle, lang)
	}

	return &Localizer{
		bundle:          bundle,
		defaultLanguage: cfg.DefaultLanguage,
		localizers:      localizers,
	}, nil
}

// Get returns localized message
func (l *Localizer) Get(lang, messageID string, data map[string]interface{}) string {
	localizer, exists := l.localizers[lang]
	if !exists {
		localizer = l.localizers[l.defaultLanguage]
	}

	msg, err := localizer.Localize(&i18n.LocalizeConfig{
		MessageID:    messageID,
		TemplateData: data,
	})
	if err != nil {
		return messageID // Fallback to message ID
	}

	return msg
}

// Message IDs
const (
	MsgWelcome           = "welcome"
	MsgHelp              = "help"
	MsgModelChanged      = "model_changed"
	MsgModelInvalid      = "model_invalid"
	MsgCurrentModel      = "current_model"
	MsgContextCleared    = "context_cleared"
	MsgSettings          = "settings"
	MsgStats             = "stats"
	MsgUnknownCommand    = "unknown_command"
	MsgRateLimitExceeded = "rate_limit_exceeded"
	MsgError             = "error"
	MsgProcessing        = "processing"
	MsgNoContext         = "no_context"
	MsgSettingsReset     = "settings_reset"
	MsgBackgroundUpdated = "background_updated"
	MsgBackgroundHelp    = "background_help"
	MsgThinkingEnabled   = "thinking_enabled"
	MsgThinkingDisabled  = "thinking_disabled"
	MsgThinkingStatus    = "thinking_status"
	MsgKeywordsSet       = "keywords_set"
	MsgKeywordsDisabled  = "keywords_disabled"
	MsgCurrentKeywords   = "current_keywords"
)