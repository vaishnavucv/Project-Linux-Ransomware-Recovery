package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

var (
	Log *logrus.Logger
	// Statistics for periodic summary logging
	stats struct {
		mu               sync.RWMutex
		fileEvents       int64
		backupOperations int64
		detectionEvents  int64
		rollbackEvents   int64
		errors           int64
		lastSummaryTime  time.Time
		startTime        time.Time
	}

	// Configuration for optimized logging
	config struct {
		LongTermMode    bool
		SummaryInterval time.Duration
		MaxLogSize      int64 // Max log file size in bytes (50MB default)
		MaxLogFiles     int   // Max number of rotated log files
	}
)

// LogConfig holds logging configuration
type LogConfig struct {
	LongTermMode    bool          `json:"long_term_mode"`
	SummaryInterval time.Duration `json:"summary_interval"`
	MaxLogSize      int64         `json:"max_log_size"`
	MaxLogFiles     int           `json:"max_log_files"`
}

// InitLogger initializes the logger with file output and structured formatting
func InitLogger(logDir string) error {
	return InitLoggerWithConfig(logDir, LogConfig{
		LongTermMode:    false,
		SummaryInterval: 5 * time.Minute,
		MaxLogSize:      50 * 1024 * 1024, // 50MB
		MaxLogFiles:     5,
	})
}

// InitLoggerWithConfig initializes the logger with custom configuration
func InitLoggerWithConfig(logDir string, logConfig LogConfig) error {
	Log = logrus.New()

	// Set configuration
	config.LongTermMode = logConfig.LongTermMode
	config.SummaryInterval = logConfig.SummaryInterval
	config.MaxLogSize = logConfig.MaxLogSize
	config.MaxLogFiles = logConfig.MaxLogFiles

	// Initialize statistics
	stats.startTime = time.Now()
	stats.lastSummaryTime = time.Now()

	// Create logs directory if it doesn't exist
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}

	// Create journal.log file with rotation
	journalPath := filepath.Join(logDir, "journal.log")

	// Check if log rotation is needed
	if err := rotateLogIfNeeded(journalPath); err != nil {
		return fmt.Errorf("failed to rotate log: %w", err)
	}

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

	// Set log level based on mode
	if config.LongTermMode {
		Log.SetLevel(logrus.WarnLevel) // Only warnings and errors for long-term operation
		Log.Warn("Logger initialized in long-term mode - minimal logging enabled")
	} else {
		Log.SetLevel(logrus.InfoLevel)
		Log.Info("Logger initialized in standard mode")
	}

	// Start periodic summary logging if in long-term mode
	if config.LongTermMode {
		go periodicSummaryLogger()
	}

	return nil
}

// rotateLogIfNeeded rotates the log file if it exceeds the maximum size
func rotateLogIfNeeded(logPath string) error {
	info, err := os.Stat(logPath)
	if os.IsNotExist(err) {
		return nil // File doesn't exist, no rotation needed
	}
	if err != nil {
		return err
	}

	if info.Size() < config.MaxLogSize {
		return nil // File is still within size limits
	}

	// Rotate existing log files
	for i := config.MaxLogFiles - 1; i > 0; i-- {
		oldPath := fmt.Sprintf("%s.%d", logPath, i)
		newPath := fmt.Sprintf("%s.%d", logPath, i+1)

		if i == config.MaxLogFiles-1 {
			// Remove the oldest log file
			os.Remove(newPath)
		}

		if _, err := os.Stat(oldPath); err == nil {
			os.Rename(oldPath, newPath)
		}
	}

	// Move current log to .1
	rotatedPath := fmt.Sprintf("%s.1", logPath)
	if err := os.Rename(logPath, rotatedPath); err != nil {
		return err
	}

	return nil
}

// periodicSummaryLogger logs periodic summaries in long-term mode
func periodicSummaryLogger() {
	ticker := time.NewTicker(config.SummaryInterval)
	defer ticker.Stop()

	for range ticker.C {
		logPeriodicSummary()
	}
}

// logPeriodicSummary logs a summary of system activity
func logPeriodicSummary() {
	stats.mu.RLock()
	defer stats.mu.RUnlock()

	uptime := time.Since(stats.startTime)
	timeSinceLastSummary := time.Since(stats.lastSummaryTime)

	Log.WithFields(logrus.Fields{
		"uptime":            uptime.String(),
		"period":            timeSinceLastSummary.String(),
		"file_events":       stats.fileEvents,
		"backup_operations": stats.backupOperations,
		"detection_events":  stats.detectionEvents,
		"rollback_events":   stats.rollbackEvents,
		"errors":            stats.errors,
		"events_per_minute": float64(stats.fileEvents) / timeSinceLastSummary.Minutes(),
	}).Info("System activity summary")

	// Reset counters for next period
	stats.fileEvents = 0
	stats.backupOperations = 0
	stats.detectionEvents = 0
	stats.rollbackEvents = 0
	stats.errors = 0
	stats.lastSummaryTime = time.Now()
}

// LogFileEvent logs file system events (optimized for long-term operation)
func LogFileEvent(eventType, filePath, details string) {
	stats.mu.Lock()
	stats.fileEvents++
	stats.mu.Unlock()

	// In long-term mode, only log critical file events
	if config.LongTermMode {
		// Only log suspicious file extensions or critical events
		if containsSuspiciousExtension(filePath) {
			Log.WithFields(logrus.Fields{
				"event_type": eventType,
				"file_path":  filePath,
				"details":    details,
				"component":  "agent",
			}).Warn("Suspicious file event detected")
		}
		return
	}

	// Standard mode - log all events
	Log.WithFields(logrus.Fields{
		"event_type": eventType,
		"file_path":  filePath,
		"details":    details,
		"component":  "agent",
	}).Info("File system event")
}

// containsSuspiciousExtension checks if a file path contains suspicious extensions
func containsSuspiciousExtension(filePath string) bool {
	suspiciousExts := []string{".locked", ".encrypted", ".crypto", ".crypt", ".enc"}
	for _, ext := range suspiciousExts {
		if filepath.Ext(filePath) == ext {
			return true
		}
	}
	return false
}

// LogDetection logs suspicious pattern detection (always logged as it's critical)
func LogDetection(pattern, filePath, severity string) {
	stats.mu.Lock()
	stats.detectionEvents++
	stats.mu.Unlock()

	Log.WithFields(logrus.Fields{
		"pattern":   pattern,
		"file_path": filePath,
		"severity":  severity,
		"component": "detector",
	}).Warn("Suspicious pattern detected")
}

// LogBackup logs backup operations (optimized for long-term operation)
func LogBackup(operation, sourcePath, backupPath string, success bool) {
	stats.mu.Lock()
	stats.backupOperations++
	if !success {
		stats.errors++
	}
	stats.mu.Unlock()

	// In long-term mode, only log failed backup operations
	if config.LongTermMode {
		if !success {
			Log.WithFields(logrus.Fields{
				"operation":   operation,
				"source_path": sourcePath,
				"backup_path": backupPath,
				"success":     success,
				"component":   "backup",
			}).Error("Backup operation failed")
		}
		return
	}

	// Standard mode - log all backup operations
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

// LogRollback logs rollback operations (always logged as it's critical)
func LogRollback(operation, filePath string, success bool) {
	stats.mu.Lock()
	stats.rollbackEvents++
	if !success {
		stats.errors++
	}
	stats.mu.Unlock()

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

// LogError logs general errors (always logged)
func LogError(component, operation string, err error) {
	stats.mu.Lock()
	stats.errors++
	stats.mu.Unlock()

	Log.WithFields(logrus.Fields{
		"component": component,
		"operation": operation,
		"error":     err.Error(),
	}).Error("Operation failed")
}

// LogFileDeletion logs file deletion events with high alert priority (always logged)
func LogFileDeletion(filePath, deletionType, details string, hasBackup bool) {
	stats.mu.Lock()
	stats.fileEvents++
	if !hasBackup {
		stats.errors++ // Count deletions without backup as errors
	}
	stats.mu.Unlock()

	// Always log file deletions regardless of logging mode
	Log.WithFields(logrus.Fields{
		"alert_level":   "HIGH",
		"event_type":    "FILE_DELETION",
		"file_path":     filePath,
		"deletion_type": deletionType, // "removed", "purged", "missing"
		"details":       details,
		"has_backup":    hasBackup,
		"component":     "agent",
		"timestamp":     time.Now().Format("2006-01-02 15:04:05.000"),
	}).Warn("🚨 HIGH ALERT: File deletion detected")
}

// LogMassFileDeletion logs when multiple files are deleted in a short time (critical alert)
func LogMassFileDeletion(deletedFiles []string, timeWindow time.Duration, hasBackups int) {
	stats.mu.Lock()
	stats.fileEvents += int64(len(deletedFiles))
	stats.errors += int64(len(deletedFiles) - hasBackups)
	stats.mu.Unlock()

	// Always log mass deletions as critical alerts
	Log.WithFields(logrus.Fields{
		"alert_level":          "CRITICAL",
		"event_type":           "MASS_FILE_DELETION",
		"deleted_files_count":  len(deletedFiles),
		"files_with_backup":    hasBackups,
		"files_without_backup": len(deletedFiles) - hasBackups,
		"time_window":          timeWindow.String(),
		"deleted_files":        deletedFiles,
		"component":            "agent",
		"timestamp":            time.Now().Format("2006-01-02 15:04:05.000"),
	}).Error("🚨🚨 CRITICAL ALERT: Mass file deletion detected - Possible ransomware attack!")
}

// LogSuspiciousDeletion logs deletions that match suspicious patterns
func LogSuspiciousDeletion(filePath, reason string, backupInfo map[string]interface{}) {
	stats.mu.Lock()
	stats.fileEvents++
	stats.detectionEvents++
	stats.mu.Unlock()

	// Always log suspicious deletions
	Log.WithFields(logrus.Fields{
		"alert_level": "CRITICAL",
		"event_type":  "SUSPICIOUS_DELETION",
		"file_path":   filePath,
		"reason":      reason,
		"backup_info": backupInfo,
		"component":   "detector",
		"timestamp":   time.Now().Format("2006-01-02 15:04:05.000"),
	}).Error("🚨🚨 CRITICAL ALERT: Suspicious file deletion detected")
}

// GetLogStats returns current logging statistics
func GetLogStats() map[string]interface{} {
	stats.mu.RLock()
	defer stats.mu.RUnlock()

	return map[string]interface{}{
		"uptime":            time.Since(stats.startTime).String(),
		"file_events":       stats.fileEvents,
		"backup_operations": stats.backupOperations,
		"detection_events":  stats.detectionEvents,
		"rollback_events":   stats.rollbackEvents,
		"errors":            stats.errors,
		"long_term_mode":    config.LongTermMode,
		"summary_interval":  config.SummaryInterval.String(),
		"max_log_size":      config.MaxLogSize,
		"max_log_files":     config.MaxLogFiles,
	}
}

// EnableLongTermMode enables optimized logging for 24/7 operation
func EnableLongTermMode() {
	config.LongTermMode = true
	Log.SetLevel(logrus.WarnLevel)
	Log.Warn("Switched to long-term logging mode - minimal logging enabled")

	// Start periodic summary logging if not already running
	go periodicSummaryLogger()
}

// DisableLongTermMode disables long-term mode and returns to standard logging
func DisableLongTermMode() {
	config.LongTermMode = false
	Log.SetLevel(logrus.InfoLevel)
	Log.Info("Switched to standard logging mode")
}
