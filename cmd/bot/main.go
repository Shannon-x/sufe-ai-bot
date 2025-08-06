package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cf-ai-tgbot-go/internal/config"
	"github.com/cf-ai-tgbot-go/internal/handlers"
	"github.com/cf-ai-tgbot-go/internal/i18n"
	"github.com/cf-ai-tgbot-go/internal/middleware"
	"github.com/cf-ai-tgbot-go/internal/services/ai"
	"github.com/cf-ai-tgbot-go/internal/services/cache"
	"github.com/cf-ai-tgbot-go/internal/services/knowledge"
	"github.com/cf-ai-tgbot-go/internal/services/storage"
	"github.com/cf-ai-tgbot-go/pkg/logger"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "configs/config.yaml", "Path to configuration file")
	envFile := flag.String("env", ".env", "Path to .env file")
	flag.Parse()

	// Load .env file if exists
	if err := godotenv.Load(*envFile); err != nil {
		// It's okay if .env doesn't exist
		fmt.Printf("Warning: .env file not found: %v\n", err)
	}

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	log, err := logger.NewLogger(&cfg.Logging)
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	log.Info("Starting Telegram Bot...")
	
	// Debug: Log token length (not the actual token for security)
	log.WithField("token_length", len(cfg.Bot.Token)).Info("Bot token loaded")

	// Initialize bot
	bot, err := tgbotapi.NewBotAPI(cfg.Bot.Token)
	if err != nil {
		log.WithError(err).Fatal("Failed to create bot")
	}

	bot.Debug = cfg.Logging.Level == "debug"
	log.WithField("username", bot.Self.UserName).Info("Bot authorized")

	// Initialize services
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize storage
	storageManager, err := storage.NewManager(cfg, log)
	if err != nil {
		log.WithError(err).Fatal("Failed to initialize storage")
	}

	// Initialize AI service
	aiService := ai.NewCustomAI(&cfg.Models, log)

	// Initialize knowledge service
	var knowledgeService knowledge.Service
	if cfg.Knowledge.Enabled {
		knowledgeService = knowledge.NewVectorKnowledgeService(log)
		if err := knowledgeService.LoadKnowledgeBase(ctx, cfg.Knowledge.Directory); err != nil {
			log.WithError(err).Error("Failed to load knowledge base")
			// Continue without knowledge base
			knowledgeService = nil
		} else {
			docs := knowledgeService.GetAllDocuments()
			log.WithField("documents", len(docs)).Info("Knowledge base loaded")
		}
	}

	// Initialize cache
	cacheService := cache.NewCache(cfg, log)

	// Initialize rate limiter
	rateLimiter := middleware.NewRateLimiter(cfg, log)

	// Initialize i18n
	localizer, err := i18n.NewLocalizer(&cfg.I18n)
	if err != nil {
		log.WithError(err).Fatal("Failed to initialize i18n")
	}

	// Initialize metrics
	metrics := middleware.NewMetrics()

	// Start metrics server if enabled
	if cfg.Monitoring.Metrics.Enabled {
		go func() {
			log.WithFields(logrus.Fields{
				"port": cfg.Monitoring.Metrics.Port,
				"path": cfg.Monitoring.Metrics.Path,
			}).Info("Starting metrics server")
			
			if err := middleware.StartMetricsServer(cfg.Monitoring.Metrics.Port, cfg.Monitoring.Metrics.Path); err != nil {
				log.WithError(err).Error("Metrics server failed")
			}
		}()
	}

	// Initialize handlers
	commandHandler := handlers.NewCommandHandler(
		bot,
		cfg,
		aiService,
		knowledgeService,
		storageManager,
		cacheService,
		rateLimiter,
		localizer,
		log,
	)
	
	messageHandler := handlers.NewMessageHandler(
		cfg,
		bot,
		aiService,
		knowledgeService,
		storageManager,
		cacheService,
		rateLimiter,
		localizer,
		log,
	)

	// Setup update channel
	var updates tgbotapi.UpdatesChannel

	if cfg.Bot.Webhook.Enabled {
		// Setup webhook
		webhookURL := fmt.Sprintf("%s/%s", cfg.Bot.Webhook.URL, bot.Token)
		webhook, err := tgbotapi.NewWebhook(webhookURL)
		if err != nil {
			log.WithError(err).Fatal("Failed to create webhook")
		}

		if _, err := bot.Request(webhook); err != nil {
			log.WithError(err).Fatal("Failed to set webhook")
		}

		updates = bot.ListenForWebhook("/" + bot.Token)
		log.WithField("url", webhookURL).Info("Webhook set")
	} else {
		// Use long polling
		u := tgbotapi.NewUpdate(0)
		u.Timeout = cfg.Bot.UpdateTimeout

		updates = bot.GetUpdatesChan(u)
		log.Info("Using long polling")
	}

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Main bot loop
	go func() {
		for update := range updates {
			// Handle callback queries
			if update.CallbackQuery != nil {
				if err := commandHandler.HandleCallbackQuery(ctx, update.CallbackQuery); err != nil {
					log.WithError(err).Error("Failed to handle callback query")
				}
				continue
			}
			
			// Skip if no message
			if update.Message == nil {
				continue
			}

			// Record metrics
			chatType := "private"
			if update.Message.Chat.IsGroup() || update.Message.Chat.IsSuperGroup() {
				chatType = "group"
			}
			metrics.RecordMessageReceived(chatType)

			// Handle commands
			if update.Message.IsCommand() {
				metrics.RecordCommandExecuted(update.Message.Command())
				
				if err := commandHandler.HandleCommand(ctx, update.Message); err != nil {
					log.WithError(err).Error("Failed to handle command")
					metrics.RecordMessageProcessed("error")
				} else {
					metrics.RecordMessageProcessed("success")
				}
				continue
			}

			// Handle regular messages
			if err := messageHandler.HandleMessage(ctx, &update); err != nil {
				log.WithError(err).Error("Failed to handle message")
				metrics.RecordMessageProcessed("error")
			} else {
				metrics.RecordMessageProcessed("success")
			}
		}
	}()

	// Start periodic tasks
	go startPeriodicTasks(ctx, storageManager, metrics, log)

	// Wait for shutdown signal
	<-sigChan
	log.Info("Shutdown signal received")

	// Cleanup
	if cfg.Bot.Webhook.Enabled {
		if _, err := bot.Request(tgbotapi.DeleteWebhookConfig{}); err != nil {
			log.WithError(err).Error("Failed to delete webhook")
		}
	}

	// Cancel context to stop all goroutines
	cancel()

	// Give goroutines time to finish
	time.Sleep(2 * time.Second)

	log.Info("Bot stopped")
}

// startPeriodicTasks starts periodic background tasks
func startPeriodicTasks(ctx context.Context, storage *storage.Manager, metrics *middleware.Metrics, log *logrus.Logger) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Update active users/chats metrics
			// This is a placeholder - in production you'd query storage
			// to get actual counts
			metrics.SetActiveUsers(0)
			metrics.SetActiveChats(0)
		}
	}
}