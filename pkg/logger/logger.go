package logger

import (
	"os"
	"path/filepath"

	"github.com/cf-ai-tgbot-go/internal/config"
	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

// NewLogger creates a new logger instance
func NewLogger(cfg *config.LoggingConfig) (*logrus.Logger, error) {
	logger := logrus.New()

	// Set log level
	level, err := logrus.ParseLevel(cfg.Level)
	if err != nil {
		return nil, err
	}
	logger.SetLevel(level)

	// Set formatter
	if cfg.Format == "json" {
		logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
			FieldMap: logrus.FieldMap{
				logrus.FieldKeyTime:  "timestamp",
				logrus.FieldKeyLevel: "level",
				logrus.FieldKeyMsg:   "message",
			},
		})
	} else {
		logger.SetFormatter(&logrus.TextFormatter{
			TimestampFormat: "2006-01-02 15:04:05",
			FullTimestamp:   true,
		})
	}

	// Set output
	switch cfg.Output {
	case "stdout":
		logger.SetOutput(os.Stdout)
	case "file":
		// Create log directory if it doesn't exist
		logDir := filepath.Dir(cfg.File.Path)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return nil, err
		}

		// Use lumberjack for log rotation
		logger.SetOutput(&lumberjack.Logger{
			Filename:   cfg.File.Path,
			MaxSize:    cfg.File.MaxSize,    // megabytes
			MaxBackups: cfg.File.MaxBackups,
			MaxAge:     cfg.File.MaxAge,     // days
			Compress:   true,
		})
	default:
		logger.SetOutput(os.Stdout)
	}

	return logger, nil
}

// WithContext adds common fields to logger
func WithContext(logger *logrus.Logger, chatID int64, userID int64) *logrus.Entry {
	return logger.WithFields(logrus.Fields{
		"chat_id": chatID,
		"user_id": userID,
	})
}