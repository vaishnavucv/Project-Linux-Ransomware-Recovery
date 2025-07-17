package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"ransomware-recovery/pkg/backup"
	"ransomware-recovery/pkg/config"
	"ransomware-recovery/pkg/logger"
)

// Agent handles file system monitoring and backup operations
type Agent struct {
	config        *config.Config
	watcher       *fsnotify.Watcher
	backupManager *backup.BackupManager
	eventChan     chan FileEvent
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
}

// FileEvent represents a file system event
type FileEvent struct {
	Path      string
	Operation string
	Timestamp time.Time
	Size      int64
}

// NewAgent creates a new file system monitoring agent
func NewAgent(cfg *config.Config) (*Agent, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create watcher: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	agent := &Agent{
		config:        cfg,
		watcher:       watcher,
		backupManager: backup.NewBackupManager(cfg),
		eventChan:     make(chan FileEvent, cfg.System.BufferSize),
		ctx:           ctx,
		cancel:        cancel,
	}

	return agent, nil
}

// Start begins monitoring the specified directory
func (a *Agent) Start() error {
	logger.Log.Info("Starting file system monitoring agent")

	// Add the monitor path to the watcher
	if err := a.addWatchPath(a.config.MonitorPath); err != nil {
		return fmt.Errorf("failed to add watch path: %w", err)
	}

	// Start worker goroutines for processing events
	for i := 0; i < a.config.System.WorkerCount; i++ {
		a.wg.Add(1)
		go a.eventWorker(i)
	}

	// Start the main event processing loop
	a.wg.Add(1)
	go a.processEvents()

	// Start periodic cleanup of old backups
	a.wg.Add(1)
	go a.periodicCleanup()

	logger.Log.Info("File system monitoring agent started successfully")
	return nil
}

// Stop stops the monitoring agent
func (a *Agent) Stop() error {
	logger.Log.Info("Stopping file system monitoring agent")

	// Cancel context to stop all goroutines
	a.cancel()

	// Close the watcher
	if err := a.watcher.Close(); err != nil {
		logger.LogError("agent", "close_watcher", err)
	}

	// Close the event channel
	close(a.eventChan)

	// Wait for all goroutines to finish
	a.wg.Wait()

	logger.Log.Info("File system monitoring agent stopped")
	return nil
}

// GetEventChannel returns the event channel for external consumption
func (a *Agent) GetEventChannel() <-chan FileEvent {
	return a.eventChan
}

// addWatchPath recursively adds a path and its subdirectories to the watcher
func (a *Agent) addWatchPath(path string) error {
	return filepath.Walk(path, func(walkPath string, info os.FileInfo, err error) error {
		if err != nil {
			logger.LogError("agent", "walk_path", err)
			return err
		}

		if info.IsDir() {
			if err := a.watcher.Add(walkPath); err != nil {
				logger.LogError("agent", "add_watcher", err)
				return fmt.Errorf("failed to add watcher for %s: %w", walkPath, err)
			}
			logger.Log.WithField("path", walkPath).Debug("Added directory to watcher")
		}

		return nil
	})
}

// processEvents processes file system events from fsnotify
func (a *Agent) processEvents() {
	defer a.wg.Done()

	for {
		select {
		case <-a.ctx.Done():
			return

		case event, ok := <-a.watcher.Events:
			if !ok {
				return
			}

			a.handleFSEvent(event)

		case err, ok := <-a.watcher.Errors:
			if !ok {
				return
			}
			logger.LogError("agent", "watcher_error", err)
		}
	}
}

// handleFSEvent handles individual file system events
func (a *Agent) handleFSEvent(event fsnotify.Event) {
	// Skip events for backup directory to avoid infinite loops
	if strings.HasPrefix(event.Name, a.config.BackupPath) {
		return
	}

	// Get file info
	var fileSize int64
	if info, err := os.Stat(event.Name); err == nil && !info.IsDir() {
		fileSize = info.Size()
	}

	// Create file event
	fileEvent := FileEvent{
		Path:      event.Name,
		Operation: event.Op.String(),
		Timestamp: time.Now(),
		Size:      fileSize,
	}

	// Log the event
	logger.LogFileEvent(fileEvent.Operation, fileEvent.Path, 
		fmt.Sprintf("size: %d bytes", fileEvent.Size))

	// Send event to channel for external processing
	select {
	case a.eventChan <- fileEvent:
	case <-a.ctx.Done():
		return
	default:
		// Channel is full, log warning
		logger.Log.Warn("Event channel full, dropping event")
	}

	// Handle specific operations
	switch {
	case event.Op&fsnotify.Create == fsnotify.Create:
		a.handleFileCreate(event.Name)
	case event.Op&fsnotify.Write == fsnotify.Write:
		a.handleFileWrite(event.Name)
	case event.Op&fsnotify.Remove == fsnotify.Remove:
		a.handleFileRemove(event.Name)
	case event.Op&fsnotify.Rename == fsnotify.Rename:
		a.handleFileRename(event.Name)
	case event.Op&fsnotify.Chmod == fsnotify.Chmod:
		a.handleFileChmod(event.Name)
	}
}

// handleFileCreate handles file creation events
func (a *Agent) handleFileCreate(filePath string) {
	// Check if it's a directory
	if info, err := os.Stat(filePath); err == nil && info.IsDir() {
		// Add new directory to watcher
		if err := a.watcher.Add(filePath); err != nil {
			logger.LogError("agent", "add_new_dir", err)
		} else {
			logger.Log.WithField("path", filePath).Info("Added new directory to watcher")
		}
		return
	}

	// Backup the file if auto-backup is enabled
	if a.config.Backup.AutoBackup {
		a.backupFileAsync(filePath)
	}
}

// handleFileWrite handles file write events
func (a *Agent) handleFileWrite(filePath string) {
	// Skip if it's a directory
	if info, err := os.Stat(filePath); err != nil || info.IsDir() {
		return
	}

	// Backup the file if auto-backup is enabled
	if a.config.Backup.AutoBackup {
		a.backupFileAsync(filePath)
	}
}

// handleFileRemove handles file removal events
func (a *Agent) handleFileRemove(filePath string) {
	logger.Log.WithField("path", filePath).Info("File removed")
	// Note: We keep the backup even if the original file is removed
}

// handleFileRename handles file rename events
func (a *Agent) handleFileRename(filePath string) {
	logger.Log.WithField("path", filePath).Info("File renamed")
	// For renames, we might want to update backup paths in the future
}

// handleFileChmod handles file permission change events
func (a *Agent) handleFileChmod(filePath string) {
	logger.Log.WithField("path", filePath).Debug("File permissions changed")
}

// backupFileAsync backs up a file asynchronously
func (a *Agent) backupFileAsync(filePath string) {
	go func() {
		ctx, cancel := context.WithTimeout(a.ctx, a.config.GetOperationTimeout())
		defer cancel()

		if err := a.backupManager.BackupFile(ctx, filePath); err != nil {
			logger.LogError("agent", "backup_file", err)
			logger.LogBackup("create", filePath, "", false)
		}
	}()
}

// eventWorker processes events from the event channel
func (a *Agent) eventWorker(workerID int) {
	defer a.wg.Done()

	logger.Log.WithField("worker_id", workerID).Info("Event worker started")

	for {
		select {
		case <-a.ctx.Done():
			return

		case event, ok := <-a.eventChan:
			if !ok {
				return
			}

			// Process the event (this is where external consumers would handle events)
			a.processFileEvent(event)
		}
	}
}

// processFileEvent processes individual file events
func (a *Agent) processFileEvent(event FileEvent) {
	// This method can be extended to perform additional processing
	// For now, it just logs the event processing
	logger.Log.WithFields(map[string]interface{}{
		"path":      event.Path,
		"operation": event.Operation,
		"size":      event.Size,
		"timestamp": event.Timestamp,
	}).Debug("Processing file event")
}

// periodicCleanup performs periodic cleanup of old backups
func (a *Agent) periodicCleanup() {
	defer a.wg.Done()

	// Run cleanup every 24 hours
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	// Run initial cleanup after 1 hour
	initialTimer := time.NewTimer(1 * time.Hour)
	defer initialTimer.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return

		case <-initialTimer.C:
			a.runCleanup()

		case <-ticker.C:
			a.runCleanup()
		}
	}
}

// runCleanup runs the backup cleanup process
func (a *Agent) runCleanup() {
	logger.Log.Info("Starting periodic backup cleanup")

	ctx, cancel := context.WithTimeout(a.ctx, a.config.GetOperationTimeout())
	defer cancel()

	if err := a.backupManager.CleanupOldBackups(ctx); err != nil {
		logger.LogError("agent", "cleanup_backups", err)
	} else {
		logger.Log.Info("Periodic backup cleanup completed successfully")
	}
}

// GetStats returns statistics about the agent
func (a *Agent) GetStats() map[string]interface{} {
	backups, err := a.backupManager.ListBackups()
	backupCount := len(backups)
	if err != nil {
		backupCount = -1
	}

	return map[string]interface{}{
		"monitor_path":  a.config.MonitorPath,
		"backup_path":   a.config.BackupPath,
		"backup_count":  backupCount,
		"worker_count":  a.config.System.WorkerCount,
		"buffer_size":   a.config.System.BufferSize,
		"auto_backup":   a.config.Backup.AutoBackup,
	}
} 