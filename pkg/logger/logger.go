package logger

import (
	"io"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

var Log *logrus.Logger

// InitLogger initializes the logger with file output and structured formatting
func InitLogger(logDir string) error {
	Log = logrus.New()

	// Create logs directory if it doesn't exist
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}

	// Create journal.log file
	journalPath := filepath.Join(logDir, "journal.log")
	file, err := os.OpenFile(journalPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return err
	}

	// Set up multi-writer to write to both file and stdout
	multiWriter := io.MultiWriter(os.Stdout, file)
	Log.SetOutput(multiWriter)

	// Set JSON formatter for structured logging
	Log.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: "2006-01-02 15:04:05",
	})

	// Set log level
	Log.SetLevel(logrus.InfoLevel)

	Log.Info("Logger initialized successfully")
	return nil
}

// LogFileEvent logs file system events
func LogFileEvent(eventType, filePath, details string) {
	Log.WithFields(logrus.Fields{
		"event_type": eventType,
		"file_path":  filePath,
		"details":    details,
		"component":  "agent",
	}).Info("File system event")
}

// LogDetection logs suspicious pattern detection
func LogDetection(pattern, filePath, severity string) {
	Log.WithFields(logrus.Fields{
		"pattern":   pattern,
		"file_path": filePath,
		"severity":  severity,
		"component": "detector",
	}).Warn("Suspicious pattern detected")
}

// LogBackup logs backup operations
func LogBackup(operation, sourcePath, backupPath string, success bool) {
	level := logrus.InfoLevel
	if !success {
		level = logrus.ErrorLevel
	}

	Log.WithFields(logrus.Fields{
		"operation":   operation,
		"source_path": sourcePath,
		"backup_path": backupPath,
		"success":     success,
		"component":   "backup",
	}).Log(level, "Backup operation")
}

// LogRollback logs rollback operations
func LogRollback(operation, filePath string, success bool) {
	level := logrus.InfoLevel
	if !success {
		level = logrus.ErrorLevel
	}

	Log.WithFields(logrus.Fields{
		"operation": operation,
		"file_path": filePath,
		"success":   success,
		"component": "rollback",
	}).Log(level, "Rollback operation")
}

// LogError logs general errors
func LogError(component, operation string, err error) {
	Log.WithFields(logrus.Fields{
		"component": component,
		"operation": operation,
		"error":     err.Error(),
	}).Error("Operation failed")
} 