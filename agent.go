package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"ransomware-recovery/pkg/backup"
	"ransomware-recovery/pkg/config"
	"ransomware-recovery/pkg/logger"

	"github.com/fsnotify/fsnotify"
)

// Agent handles file system monitoring and backup operations
type Agent struct {
	config          *config.Config
	watcher         *fsnotify.Watcher
	backupManager   *backup.BackupManager
	eventChan       chan FileEvent
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	deletionTracker *DeletionTracker
}

// DeletionTracker tracks file deletions for mass deletion detection
type DeletionTracker struct {
	mu              sync.RWMutex
	recentDeletions []DeletionEvent
	alertThreshold  int           // Number of deletions to trigger alert
	timeWindow      time.Duration // Time window to check for mass deletions
}

// DeletionEvent represents a file deletion event
type DeletionEvent struct {
	FilePath   string    `json:"file_path"`
	DeletedAt  time.Time `json:"deleted_at"`
	HasBackup  bool      `json:"has_backup"`
	FileSize   int64     `json:"file_size"`
	BackupPath string    `json:"backup_path,omitempty"`
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

	// Initialize deletion tracker with configuration
	deletionTracker := &DeletionTracker{
		recentDeletions: make([]DeletionEvent, 0),
		alertThreshold:  cfg.Detection.DeletionAlert.MassThreshold,
		timeWindow:      time.Duration(cfg.Detection.DeletionAlert.TimeWindow) * time.Minute,
	}

	agent := &Agent{
		config:          cfg,
		watcher:         watcher,
		backupManager:   backup.NewBackupManager(cfg),
		eventChan:       make(chan FileEvent, cfg.System.BufferSize),
		ctx:             ctx,
		cancel:          cancel,
		deletionTracker: deletionTracker,
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

	// Perform initial scan and backup of existing files
	if err := a.performInitialScan(); err != nil {
		logger.LogError("agent", "initial_scan", err)
		// Don't fail startup for initial scan errors
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

	// Start periodic file scanning if enabled
	if a.config.System.PeriodicScan.Enabled {
		a.wg.Add(1)
		go a.periodicFileScanner()
	}

	// Start periodic remote backup sync if enabled
	a.backupManager.StartPeriodicRemoteSync(a.ctx)

	logger.Log.Info("File system monitoring agent started successfully")
	return nil
}

// performInitialScan scans existing files in demo-folder and backs up new or changed files
func (a *Agent) performInitialScan() error {
	logger.Log.Info("Starting initial file scan and backup")

	startTime := time.Now()
	scannedFiles := 0
	backedUpFiles := 0

	// Walk through all files in the monitor directory
	err := filepath.Walk(a.config.MonitorPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			logger.LogError("agent", "walk_file", err)
			return nil // Continue walking despite errors
		}

		// Skip directories and backup path
		if info.IsDir() || strings.HasPrefix(path, a.config.BackupPath) {
			return nil
		}

		scannedFiles++

		// Analyze if file needs backup
		changeInfo, err := a.backupManager.AnalyzeFileChange(path)
		if err != nil {
			logger.LogError("agent", "analyze_file", err)
			return nil // Continue despite analysis errors
		}

		// Log file analysis for timeline
		logger.Log.WithFields(map[string]interface{}{
			"file_path":     path,
			"change_type":   changeInfo.ChangeType,
			"backup_needed": changeInfo.BackupNeeded,
			"file_size":     changeInfo.Size,
			"mod_time":      changeInfo.ModTime.Format("2006-01-02 15:04:05"),
			"component":     "agent",
			"event_type":    "initial_scan",
		}).Debug("Initial scan file analysis")

		// Backup file if needed
		if changeInfo.BackupNeeded {
			ctx, cancel := context.WithTimeout(a.ctx, a.config.GetOperationTimeout())

			if err := a.backupManager.BackupFileImmediate(ctx, path); err != nil {
				logger.LogError("agent", "initial_backup", err)
				cancel()
				return nil // Continue despite backup errors
			}

			backedUpFiles++
			cancel()
		}

		return nil
	})

	duration := time.Since(startTime)

	// Log initial scan results
	logger.Log.WithFields(map[string]interface{}{
		"scanned_files":   scannedFiles,
		"backed_up_files": backedUpFiles,
		"duration":        duration.String(),
		"duration_ms":     duration.Milliseconds(),
		"component":       "agent",
		"event_type":      "initial_scan_complete",
	}).Info("Initial file scan and backup completed")

	if err != nil {
		return fmt.Errorf("initial scan encountered errors: %w", err)
	}

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

	// Log the event with timeline information
	logger.LogFileEvent(fileEvent.Operation, fileEvent.Path,
		fmt.Sprintf("size: %d bytes, timestamp: %s", fileEvent.Size, fileEvent.Timestamp.Format("2006-01-02 15:04:05.000")))

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

// handleFileRemove handles file removal events with high-alert logging
func (a *Agent) handleFileRemove(filePath string) {
	// Get file size before it was deleted (if available in backup)
	var fileSize int64
	var backupPath string
	hasBackup := false

	// Check if we have backup information for this file
	if backupInfo, exists := a.backupManager.GetFileBackupInfo(filePath); exists {
		hasBackup = true
		fileSize = backupInfo.Size
		backupPath = backupInfo.BackupPath
	}

	// Create deletion event
	deletionEvent := DeletionEvent{
		FilePath:   filePath,
		DeletedAt:  time.Now(),
		HasBackup:  hasBackup,
		FileSize:   fileSize,
		BackupPath: backupPath,
	}

	// Track the deletion
	a.trackDeletion(deletionEvent)

	// Mark file as deleted in backup database
	if err := a.backupManager.MarkFileDeleted(filePath); err != nil {
		logger.LogError("agent", "mark_file_deleted", err)
	}

	// Check for suspicious deletion patterns
	a.checkSuspiciousDeletion(filePath, deletionEvent)

	// Note: We keep the backup even if the original file is removed
	logger.Log.WithFields(map[string]interface{}{
		"file_path":   filePath,
		"has_backup":  hasBackup,
		"backup_path": backupPath,
		"file_size":   fileSize,
		"component":   "agent",
		"event_type":  "file_removed",
	}).Info("File removal processed")
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

// backupFileAsync backs up a file asynchronously with immediate change detection
func (a *Agent) backupFileAsync(filePath string) {
	go func() {
		ctx, cancel := context.WithTimeout(a.ctx, a.config.GetOperationTimeout())
		defer cancel()

		// Use immediate backup with hash-based change detection
		if err := a.backupManager.BackupFileImmediate(ctx, filePath); err != nil {
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

	// Get backup database statistics
	backupStats := a.backupManager.GetBackupStats()

	return map[string]interface{}{
		"monitor_path":   a.config.MonitorPath,
		"backup_path":    a.config.BackupPath,
		"backup_count":   backupCount,
		"tracked_files":  backupStats["tracked_files"],
		"backup_records": backupStats["backup_records"],
		"deleted_files":  backupStats["deleted_files"],
		"worker_count":   a.config.System.WorkerCount,
		"buffer_size":    a.config.System.BufferSize,
		"auto_backup":    a.config.Backup.AutoBackup,
		"database_path":  backupStats["database_path"],
		"periodic_scan": map[string]interface{}{
			"enabled":             a.config.System.PeriodicScan.Enabled,
			"random_intervals":    a.config.System.PeriodicScan.RandomIntervals,
			"deep_analysis":       a.config.System.PeriodicScan.DeepAnalysis,
			"max_files_per_batch": a.config.System.PeriodicScan.MaxFilesPerBatch,
			"scan_timeout_ms":     a.config.System.PeriodicScan.ScanTimeoutMs,
		},
	}
}

// periodicFileScanner performs periodic file scanning at random intervals
func (a *Agent) periodicFileScanner() {
	defer a.wg.Done()

	logger.Log.WithFields(map[string]interface{}{
		"enabled":             a.config.System.PeriodicScan.Enabled,
		"random_intervals":    a.config.System.PeriodicScan.RandomIntervals,
		"deep_analysis":       a.config.System.PeriodicScan.DeepAnalysis,
		"max_files_per_batch": a.config.System.PeriodicScan.MaxFilesPerBatch,
	}).Info("Starting periodic file scanner")

	// Initialize random seed
	rand.Seed(time.Now().UnixNano())

	for {
		// Select random interval from configured options
		intervals := a.config.System.PeriodicScan.RandomIntervals
		if len(intervals) == 0 {
			logger.LogError("agent", "periodic_scanner", fmt.Errorf("no random intervals configured"))
			return
		}

		randomInterval := intervals[rand.Intn(len(intervals))]
		nextScanTime := time.Duration(randomInterval) * time.Second

		logger.Log.WithFields(map[string]interface{}{
			"next_scan_in_seconds": randomInterval,
			"next_scan_in":         nextScanTime.String(),
			"component":            "agent",
			"event_type":           "periodic_scan_scheduled",
		}).Info("Scheduled next periodic file scan")

		// Wait for the random interval or context cancellation
		select {
		case <-a.ctx.Done():
			logger.Log.Info("Periodic file scanner stopped")
			return
		case <-time.After(nextScanTime):
			// Perform the scan
			a.performPeriodicScan()
		}
	}
}

// performPeriodicScan scans all files for changes and takes action if needed
func (a *Agent) performPeriodicScan() {
	scanStartTime := time.Now()
	scannedFiles := 0
	changedFiles := 0
	suspiciousFiles := 0
	errors := 0

	logger.Log.WithFields(map[string]interface{}{
		"component":     "agent",
		"event_type":    "periodic_scan_start",
		"scan_time":     scanStartTime.Format("2006-01-02 15:04:05"),
		"deep_analysis": a.config.System.PeriodicScan.DeepAnalysis,
	}).Info("Starting periodic file scan")

	// Create context with timeout for the entire scan
	scanTimeout := time.Duration(a.config.System.PeriodicScan.ScanTimeoutMs) * time.Millisecond *
		time.Duration(a.config.System.PeriodicScan.MaxFilesPerBatch)
	ctx, cancel := context.WithTimeout(a.ctx, scanTimeout)
	defer cancel()

	// Walk through all files in the monitor directory
	err := filepath.Walk(a.config.MonitorPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			errors++
			logger.LogError("agent", "periodic_scan_walk", err)
			return nil // Continue walking despite errors
		}

		// Skip directories and backup path
		if info.IsDir() || strings.HasPrefix(path, a.config.BackupPath) {
			return nil
		}

		// Check if we've exceeded the batch limit
		if scannedFiles >= a.config.System.PeriodicScan.MaxFilesPerBatch {
			logger.Log.WithField("max_files_reached", a.config.System.PeriodicScan.MaxFilesPerBatch).
				Debug("Reached maximum files per batch limit")
			return filepath.SkipDir
		}

		scannedFiles++

		// Check context for cancellation
		select {
		case <-ctx.Done():
			return fmt.Errorf("scan timeout reached")
		default:
		}

		// Analyze file for changes
		if changed, suspicious := a.analyzeFileForChanges(ctx, path, info); changed {
			changedFiles++
			if suspicious {
				suspiciousFiles++
			}
		}

		return nil
	})

	scanDuration := time.Since(scanStartTime)

	// Log scan completion results
	logger.Log.WithFields(map[string]interface{}{
		"component":        "agent",
		"event_type":       "periodic_scan_complete",
		"scanned_files":    scannedFiles,
		"changed_files":    changedFiles,
		"suspicious_files": suspiciousFiles,
		"errors":           errors,
		"duration":         scanDuration.String(),
		"duration_ms":      scanDuration.Milliseconds(),
		"scan_success":     err == nil,
	}).Info("Periodic file scan completed")

	if err != nil {
		logger.LogError("agent", "periodic_scan", err)
	}
}

// analyzeFileForChanges analyzes a file for changes and suspicious patterns
func (a *Agent) analyzeFileForChanges(ctx context.Context, filePath string, info os.FileInfo) (changed bool, suspicious bool) {
	// Create file timeout context
	fileTimeout := time.Duration(a.config.System.PeriodicScan.ScanTimeoutMs) * time.Millisecond
	fileCtx, cancel := context.WithTimeout(ctx, fileTimeout)
	defer cancel()

	// Analyze if file needs backup (detects changes via hash)
	changeInfo, err := a.backupManager.AnalyzeFileChange(filePath)
	if err != nil {
		logger.LogError("agent", "analyze_file_change", err)
		return false, false
	}

	// Log detailed analysis for timeline tracking
	logger.Log.WithFields(map[string]interface{}{
		"file_path":     filePath,
		"change_type":   changeInfo.ChangeType,
		"backup_needed": changeInfo.BackupNeeded,
		"file_size":     changeInfo.Size,
		"mod_time":      changeInfo.ModTime.Format("2006-01-02 15:04:05"),
		"component":     "agent",
		"event_type":    "periodic_file_analysis",
		"scan_type":     "periodic",
	}).Debug("Periodic file analysis completed")

	// If file changed, backup it immediately
	if changeInfo.BackupNeeded {
		if err := a.backupManager.BackupFileImmediate(fileCtx, filePath); err != nil {
			logger.LogError("agent", "periodic_backup", err)
		} else {
			logger.Log.WithFields(map[string]interface{}{
				"file_path":     filePath,
				"change_type":   changeInfo.ChangeType,
				"backup_reason": "periodic_scan_detected_change",
				"component":     "agent",
				"event_type":    "periodic_backup_success",
			}).Info("File backed up during periodic scan")
		}
		changed = true
	}

	// Perform deep analysis if enabled
	if a.config.System.PeriodicScan.DeepAnalysis {
		suspicious = a.performDeepAnalysis(fileCtx, filePath, info)
	}

	// Check for suspicious file extensions
	ext := strings.ToLower(filepath.Ext(filePath))
	if a.config.IsSuspiciousExtension(ext) {
		suspicious = true
		logger.Log.WithFields(map[string]interface{}{
			"file_path":        filePath,
			"suspicious_ext":   ext,
			"component":        "agent",
			"event_type":       "periodic_suspicious_file",
			"detection_method": "extension_check",
		}).Warn("Suspicious file detected during periodic scan")
	}

	return changed, suspicious
}

// performDeepAnalysis performs deep content analysis on a file
func (a *Agent) performDeepAnalysis(ctx context.Context, filePath string, info os.FileInfo) bool {
	// Only analyze small text files to avoid performance issues
	if info.Size() > 10*1024 { // Skip files larger than 10KB
		return false
	}

	// Check context for cancellation
	select {
	case <-ctx.Done():
		return false
	default:
	}

	file, err := os.Open(filePath)
	if err != nil {
		return false
	}
	defer file.Close()

	// Read first 1KB of content
	buffer := make([]byte, 1024)
	n, err := file.Read(buffer)
	if err != nil && err != fmt.Errorf("EOF") {
		return false
	}

	content := string(buffer[:n])
	if a.config.HasSuspiciousContent(content) {
		logger.Log.WithFields(map[string]interface{}{
			"file_path":        filePath,
			"component":        "agent",
			"event_type":       "periodic_suspicious_content",
			"detection_method": "content_analysis",
			"content_preview":  content[:min(100, len(content))], // First 100 chars
		}).Warn("Suspicious content detected during periodic deep analysis")
		return true
	}

	return false
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// trackDeletion adds a deletion event to the tracker and checks for mass deletions
func (a *Agent) trackDeletion(deletion DeletionEvent) {
	a.deletionTracker.mu.Lock()
	defer a.deletionTracker.mu.Unlock()

	// Add the new deletion
	a.deletionTracker.recentDeletions = append(a.deletionTracker.recentDeletions, deletion)

	// Clean up old deletions outside the time window
	now := time.Now()
	cutoff := now.Add(-a.deletionTracker.timeWindow)

	var recentDeletions []DeletionEvent
	for _, del := range a.deletionTracker.recentDeletions {
		if del.DeletedAt.After(cutoff) {
			recentDeletions = append(recentDeletions, del)
		}
	}
	a.deletionTracker.recentDeletions = recentDeletions

	// Check for mass deletion alert
	if len(a.deletionTracker.recentDeletions) >= a.deletionTracker.alertThreshold {
		a.triggerMassDeletionAlert()
	}
}

// triggerMassDeletionAlert triggers a critical alert for mass file deletion
func (a *Agent) triggerMassDeletionAlert() {
	deletedFiles := make([]string, 0, len(a.deletionTracker.recentDeletions))
	hasBackupCount := 0

	for _, deletion := range a.deletionTracker.recentDeletions {
		deletedFiles = append(deletedFiles, deletion.FilePath)
		if deletion.HasBackup {
			hasBackupCount++
		}
	}

	// Log the mass deletion alert
	logger.LogMassFileDeletion(deletedFiles, a.deletionTracker.timeWindow, hasBackupCount)
}

// checkSuspiciousDeletion checks if a deletion matches suspicious patterns
func (a *Agent) checkSuspiciousDeletion(filePath string, deletion DeletionEvent) {
	// Skip if deletion alerts are disabled
	if !a.config.Detection.DeletionAlert.Enabled {
		return
	}

	var reasons []string
	backupInfo := map[string]interface{}{
		"has_backup":  deletion.HasBackup,
		"backup_path": deletion.BackupPath,
		"file_size":   deletion.FileSize,
		"deleted_at":  deletion.DeletedAt.Format("2006-01-02 15:04:05"),
	}

	// Check for suspicious patterns
	if strings.Contains(strings.ToLower(filePath), "important") ||
		strings.Contains(strings.ToLower(filePath), "critical") ||
		strings.Contains(strings.ToLower(filePath), "backup") {
		reasons = append(reasons, "critical_file_deleted")
	}

	// Check for valuable file types (if enabled)
	if a.config.Detection.DeletionAlert.AlertOnValueableFiles {
		ext := strings.ToLower(filepath.Ext(filePath))
		if ext == ".doc" || ext == ".docx" || ext == ".pdf" || ext == ".xlsx" ||
			ext == ".pptx" || ext == ".jpg" || ext == ".png" || ext == ".mp4" {
			reasons = append(reasons, "valuable_file_type_deleted")
		}
	}

	// Check if file was large (potential data loss)
	largeFileBytes := int64(a.config.Detection.DeletionAlert.LargeFileThreshold) * 1024 * 1024
	if deletion.FileSize > largeFileBytes {
		reasons = append(reasons, fmt.Sprintf("large_file_deleted (>%dMB)", a.config.Detection.DeletionAlert.LargeFileThreshold))
	}

	// Check if file was deleted without backup (if enabled)
	if a.config.Detection.DeletionAlert.AlertOnNoBackup && !deletion.HasBackup {
		reasons = append(reasons, "no_backup_available")
	}

	// Log suspicious deletions
	if len(reasons) > 0 {
		reason := strings.Join(reasons, ", ")
		logger.LogSuspiciousDeletion(filePath, reason, backupInfo)
	}
}
