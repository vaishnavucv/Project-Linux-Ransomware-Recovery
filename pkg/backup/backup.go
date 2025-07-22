package backup

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"ransomware-recovery/pkg/config"
	"ransomware-recovery/pkg/logger"
)

// BackupManager handles file backup operations
type BackupManager struct {
	config          *config.Config
	mu              sync.RWMutex
	fileHashes      map[string]string     // Track file hashes to detect changes
	backupDB        map[string]BackupInfo // In-memory backup database
	lastRemoteSync  time.Time             // Last time remote sync was performed
	remoteSyncMutex sync.Mutex            // Mutex for remote sync operations
}

// BackupInfo holds metadata about a backup
type BackupInfo struct {
	OriginalPath string    `json:"original_path"`
	BackupPath   string    `json:"backup_path"`
	Checksum     string    `json:"checksum"`
	Size         int64     `json:"size"`
	CreatedAt    time.Time `json:"created_at"`
	ModifiedAt   time.Time `json:"modified_at"`
	BackupReason string    `json:"backup_reason"` // "new_file", "content_changed", "manual"
	Version      int       `json:"version"`       // Backup version number
	IsDeleted    bool      `json:"is_deleted"`    // Track if original file was deleted
	DeletedAt    time.Time `json:"deleted_at"`    // When the original file was deleted
}

// FileChangeInfo holds information about file changes
type FileChangeInfo struct {
	FilePath     string    `json:"file_path"`
	ChangeType   string    `json:"change_type"` // "created", "modified", "unchanged"
	OldHash      string    `json:"old_hash"`
	NewHash      string    `json:"new_hash"`
	Size         int64     `json:"size"`
	ModTime      time.Time `json:"mod_time"`
	BackupNeeded bool      `json:"backup_needed"`
}

// NewBackupManager creates a new backup manager
func NewBackupManager(cfg *config.Config) *BackupManager {
	bm := &BackupManager{
		config:     cfg,
		fileHashes: make(map[string]string),
		backupDB:   make(map[string]BackupInfo),
	}

	// Load existing backup metadata and file hashes
	bm.loadBackupDatabase()

	return bm
}

// AnalyzeFileChange analyzes if a file needs backup based on hash comparison
func (bm *BackupManager) AnalyzeFileChange(filePath string) (*FileChangeInfo, error) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	// Get current file info
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	// Skip directories
	if fileInfo.IsDir() {
		return &FileChangeInfo{
			FilePath:     filePath,
			ChangeType:   "directory",
			BackupNeeded: false,
		}, nil
	}

	// Calculate current file hash
	currentHash, err := bm.calculateFileHash(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate file hash: %w", err)
	}

	// Get stored hash if exists
	storedHash, exists := bm.fileHashes[filePath]

	changeInfo := &FileChangeInfo{
		FilePath:     filePath,
		NewHash:      currentHash,
		Size:         fileInfo.Size(),
		ModTime:      fileInfo.ModTime(),
		BackupNeeded: false,
	}

	if !exists {
		// New file
		changeInfo.ChangeType = "created"
		changeInfo.BackupNeeded = true
	} else if storedHash != currentHash {
		// File content changed
		changeInfo.ChangeType = "modified"
		changeInfo.OldHash = storedHash
		changeInfo.BackupNeeded = true
	} else {
		// File unchanged
		changeInfo.ChangeType = "unchanged"
		changeInfo.OldHash = storedHash
		changeInfo.BackupNeeded = false
	}

	return changeInfo, nil
}

// BackupFileImmediate creates an immediate backup with change detection
func (bm *BackupManager) BackupFileImmediate(ctx context.Context, filePath string) error {
	// Analyze file change
	changeInfo, err := bm.AnalyzeFileChange(filePath)
	if err != nil {
		logger.LogError("backup", "analyze_file_change", err)
		return fmt.Errorf("failed to analyze file change: %w", err)
	}

	// Log file analysis result
	logger.Log.WithFields(map[string]interface{}{
		"file_path":     changeInfo.FilePath,
		"change_type":   changeInfo.ChangeType,
		"old_hash":      changeInfo.OldHash,
		"new_hash":      changeInfo.NewHash,
		"size":          changeInfo.Size,
		"mod_time":      changeInfo.ModTime.Format("2006-01-02 15:04:05"),
		"backup_needed": changeInfo.BackupNeeded,
		"component":     "backup",
	}).Info("File change analysis completed")

	// Skip backup if file hasn't changed
	if !changeInfo.BackupNeeded {
		logger.Log.WithFields(map[string]interface{}{
			"file_path": filePath,
			"reason":    "file_unchanged",
			"hash":      changeInfo.NewHash,
			"component": "backup",
		}).Debug("Skipping backup - file unchanged")
		return nil
	}

	// Determine backup reason
	backupReason := changeInfo.ChangeType
	if changeInfo.ChangeType == "created" {
		backupReason = "new_file"
	} else if changeInfo.ChangeType == "modified" {
		backupReason = "content_changed"
	}

	// Perform the backup
	return bm.performBackup(ctx, filePath, changeInfo, backupReason)
}

// BackupFile creates a backup of the specified file (legacy method for compatibility)
func (bm *BackupManager) BackupFile(ctx context.Context, filePath string) error {
	return bm.BackupFileImmediate(ctx, filePath)
}

// performBackup executes the actual backup operation
func (bm *BackupManager) performBackup(ctx context.Context, filePath string, changeInfo *FileChangeInfo, reason string) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	// Validate file path
	if err := bm.validateFilePath(filePath); err != nil {
		logger.LogError("backup", "validate_path", err)
		return fmt.Errorf("invalid file path: %w", err)
	}

	startTime := time.Now()

	// Create backup path with versioning
	backupPath, err := bm.createVersionedBackupPath(filePath)
	if err != nil {
		logger.LogError("backup", "create_backup_path", err)
		return fmt.Errorf("failed to create backup path: %w", err)
	}

	// Create backup directory
	backupDir := filepath.Dir(backupPath)
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		logger.LogError("backup", "create_backup_dir", err)
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Copy file with verification
	if err := bm.copyFileWithVerification(ctx, filePath, backupPath); err != nil {
		logger.LogError("backup", "copy_file", err)
		return fmt.Errorf("failed to copy file: %w", err)
	}

	duration := time.Since(startTime)

	// Create backup info
	backupInfo := BackupInfo{
		OriginalPath: filePath,
		BackupPath:   backupPath,
		Checksum:     changeInfo.NewHash,
		Size:         changeInfo.Size,
		CreatedAt:    time.Now(),
		ModifiedAt:   changeInfo.ModTime,
		BackupReason: reason,
		Version:      bm.getNextVersion(filePath),
	}

	// Update tracking data
	bm.fileHashes[filePath] = changeInfo.NewHash
	bm.backupDB[filePath] = backupInfo

	// Save backup database
	if err := bm.saveBackupDatabase(); err != nil {
		logger.LogError("backup", "save_backup_db", err)
	}

	// Log detailed backup timeline event
	logger.LogBackup("create", filePath, backupPath, true)
	logger.Log.WithFields(map[string]interface{}{
		"timestamp":      time.Now().Format("2006-01-02 15:04:05.000"),
		"file_path":      filePath,
		"backup_path":    backupPath,
		"backup_reason":  reason,
		"file_size":      changeInfo.Size,
		"file_hash":      changeInfo.NewHash,
		"old_hash":       changeInfo.OldHash,
		"mod_time":       changeInfo.ModTime.Format("2006-01-02 15:04:05"),
		"backup_version": backupInfo.Version,
		"duration_ms":    duration.Milliseconds(),
		"component":      "backup",
		"event_type":     "backup_timeline",
	}).Info("Backup timeline event")

	// Trigger remote sync if needed
	go bm.triggerRemoteSyncIfNeeded(context.Background())

	return nil
}

// calculateFileHash calculates SHA256 hash of a file
func (bm *BackupManager) calculateFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// createVersionedBackupPath creates a versioned backup path
func (bm *BackupManager) createVersionedBackupPath(filePath string) (string, error) {
	// Get relative path from monitor directory
	relPath, err := filepath.Rel(bm.config.MonitorPath, filePath)
	if err != nil {
		return "", err
	}

	// Create timestamp for versioning
	timestamp := time.Now().Format("20060102_150405")

	// Add timestamp to filename for versioning
	dir := filepath.Dir(relPath)
	filename := filepath.Base(relPath)
	ext := filepath.Ext(filename)
	nameWithoutExt := strings.TrimSuffix(filename, ext)

	versionedFilename := fmt.Sprintf("%s_%s%s", nameWithoutExt, timestamp, ext)
	versionedRelPath := filepath.Join(dir, versionedFilename)

	// Create backup path
	backupPath := filepath.Join(bm.config.BackupPath, versionedRelPath)
	return backupPath, nil
}

// getNextVersion gets the next version number for a file
func (bm *BackupManager) getNextVersion(filePath string) int {
	if info, exists := bm.backupDB[filePath]; exists {
		return info.Version + 1
	}
	return 1
}

// copyFileWithVerification copies a file and verifies the copy
func (bm *BackupManager) copyFileWithVerification(ctx context.Context, src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	// Copy with context cancellation support
	done := make(chan error, 1)
	go func() {
		_, err := io.Copy(destFile, sourceFile)
		done <- err
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		if err != nil {
			return err
		}
	}

	// Verify the copy by comparing hashes
	srcHash, err := bm.calculateFileHash(src)
	if err != nil {
		return fmt.Errorf("failed to calculate source hash: %w", err)
	}

	dstHash, err := bm.calculateFileHash(dst)
	if err != nil {
		return fmt.Errorf("failed to calculate destination hash: %w", err)
	}

	if srcHash != dstHash {
		return fmt.Errorf("backup verification failed: hash mismatch")
	}

	return nil
}

// loadBackupDatabase loads existing backup metadata and file hashes
func (bm *BackupManager) loadBackupDatabase() {
	dbPath := filepath.Join(bm.config.BackupPath, "backup_database.json")

	data, err := os.ReadFile(dbPath)
	if err != nil {
		// Database doesn't exist yet, start fresh
		return
	}

	var dbData struct {
		FileHashes map[string]string     `json:"file_hashes"`
		BackupDB   map[string]BackupInfo `json:"backup_db"`
	}

	if err := json.Unmarshal(data, &dbData); err != nil {
		logger.LogError("backup", "load_backup_db", err)
		return
	}

	bm.fileHashes = dbData.FileHashes
	bm.backupDB = dbData.BackupDB

	logger.Log.WithFields(map[string]interface{}{
		"tracked_files":  len(bm.fileHashes),
		"backup_records": len(bm.backupDB),
		"component":      "backup",
	}).Info("Backup database loaded")
}

// saveBackupDatabase saves backup metadata and file hashes
func (bm *BackupManager) saveBackupDatabase() error {
	dbPath := filepath.Join(bm.config.BackupPath, "backup_database.json")

	// Ensure backup directory exists
	if err := os.MkdirAll(bm.config.BackupPath, 0755); err != nil {
		return err
	}

	dbData := struct {
		FileHashes map[string]string     `json:"file_hashes"`
		BackupDB   map[string]BackupInfo `json:"backup_db"`
		UpdatedAt  time.Time             `json:"updated_at"`
	}{
		FileHashes: bm.fileHashes,
		BackupDB:   bm.backupDB,
		UpdatedAt:  time.Now(),
	}

	data, err := json.MarshalIndent(dbData, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(dbPath, data, 0644)
}

// GetFileBackupInfo returns backup information for a file
func (bm *BackupManager) GetFileBackupInfo(filePath string) (BackupInfo, bool) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	info, exists := bm.backupDB[filePath]
	return info, exists
}

// MarkFileDeleted marks a file as deleted in the backup database
func (bm *BackupManager) MarkFileDeleted(filePath string) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if backupInfo, exists := bm.backupDB[filePath]; exists {
		backupInfo.IsDeleted = true
		backupInfo.DeletedAt = time.Now()
		bm.backupDB[filePath] = backupInfo

		// Save updated database
		if err := bm.saveBackupDatabase(); err != nil {
			return fmt.Errorf("failed to update backup database: %w", err)
		}

		// Log the deletion with backup information
		logger.LogFileDeletion(filePath, "removed",
			fmt.Sprintf("Original file deleted, backup available at: %s", backupInfo.BackupPath),
			true)

		return nil
	}

	// File was deleted but no backup exists - high priority alert
	logger.LogFileDeletion(filePath, "removed", "No backup found for deleted file", false)
	return fmt.Errorf("no backup found for deleted file: %s", filePath)
}

// GetDeletedFiles returns a list of files that have been deleted
func (bm *BackupManager) GetDeletedFiles() []BackupInfo {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	var deletedFiles []BackupInfo
	for _, backupInfo := range bm.backupDB {
		if backupInfo.IsDeleted {
			deletedFiles = append(deletedFiles, backupInfo)
		}
	}

	return deletedFiles
}

// GetBackupStats returns backup statistics
func (bm *BackupManager) GetBackupStats() map[string]interface{} {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	deletedCount := 0
	for _, backupInfo := range bm.backupDB {
		if backupInfo.IsDeleted {
			deletedCount++
		}
	}

	return map[string]interface{}{
		"tracked_files":  len(bm.fileHashes),
		"backup_records": len(bm.backupDB),
		"deleted_files":  deletedCount,
		"active_files":   len(bm.backupDB) - deletedCount,
		"database_path":  filepath.Join(bm.config.BackupPath, "backup_database.json"),
		"remote_sync":    bm.GetRemoteSyncStatus(),
	}
}

// RestoreFile restores a file from backup
func (bm *BackupManager) RestoreFile(ctx context.Context, originalPath string) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	// Get backup info
	backupInfo, exists := bm.backupDB[originalPath]
	if !exists {
		return fmt.Errorf("no backup found for file: %s", originalPath)
	}

	// Check if backup exists
	if _, err := os.Stat(backupInfo.BackupPath); os.IsNotExist(err) {
		return fmt.Errorf("backup file not found: %s", backupInfo.BackupPath)
	}

	// Verify backup integrity
	if bm.config.Backup.EnableChecksums {
		if err := bm.verifyBackupIntegrity(backupInfo.BackupPath, backupInfo.Checksum); err != nil {
			logger.LogError("backup", "verify_integrity", err)
			return fmt.Errorf("backup integrity check failed: %w", err)
		}
	}

	// Create directory for original file
	originalDir := filepath.Dir(originalPath)
	if err := os.MkdirAll(originalDir, 0755); err != nil {
		logger.LogError("backup", "create_original_dir", err)
		return fmt.Errorf("failed to create original directory: %w", err)
	}

	// Copy backup back to original location
	if err := bm.copyFile(ctx, backupInfo.BackupPath, originalPath); err != nil {
		logger.LogError("backup", "restore_file", err)
		return fmt.Errorf("failed to restore file: %w", err)
	}

	// Update file hash after restore
	newHash, err := bm.calculateFileHash(originalPath)
	if err == nil {
		bm.fileHashes[originalPath] = newHash
		bm.saveBackupDatabase()
	}

	logger.LogBackup("restore", backupInfo.BackupPath, originalPath, true)
	return nil
}

// ListBackups returns a list of all backup files
func (bm *BackupManager) ListBackups() ([]string, error) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	var backups []string
	for originalPath := range bm.backupDB {
		backups = append(backups, originalPath)
	}

	return backups, nil
}

// CleanupOldBackups removes backups older than the retention period
func (bm *BackupManager) CleanupOldBackups(ctx context.Context) error {
	cutoffTime := time.Now().AddDate(0, 0, -bm.config.Backup.RetentionDays)

	return filepath.Walk(bm.config.BackupPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && info.ModTime().Before(cutoffTime) && path != filepath.Join(bm.config.BackupPath, "backup_database.json") {
			if err := os.Remove(path); err != nil {
				logger.LogError("backup", "cleanup_old", err)
				return err
			}
			logger.LogBackup("cleanup", path, "", true)
		}

		return nil
	})
}

// copyFile copies a file without checksum calculation
func (bm *BackupManager) copyFile(ctx context.Context, src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	done := make(chan error, 1)
	go func() {
		_, err := io.Copy(destFile, sourceFile)
		done <- err
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}

// validateFilePath validates that the file path is safe
func (bm *BackupManager) validateFilePath(filePath string) error {
	cleanPath := filepath.Clean(filePath)

	if strings.Contains(cleanPath, "..") {
		return fmt.Errorf("path traversal detected: %s", filePath)
	}

	absFilePath, err := filepath.Abs(cleanPath)
	if err != nil {
		return err
	}

	absMonitorPath, err := filepath.Abs(bm.config.MonitorPath)
	if err != nil {
		return err
	}

	if !strings.HasPrefix(absFilePath, absMonitorPath) {
		return fmt.Errorf("file path outside monitor directory: %s", filePath)
	}

	return nil
}

// verifyBackupIntegrity verifies the integrity of a backup file
func (bm *BackupManager) verifyBackupIntegrity(backupPath, expectedHash string) error {
	actualHash, err := bm.calculateFileHash(backupPath)
	if err != nil {
		return err
	}

	if actualHash != expectedHash {
		return fmt.Errorf("hash mismatch: expected %s, got %s", expectedHash, actualHash)
	}

	return nil
}

// saveBackupMetadata saves backup metadata for future reference
func (bm *BackupManager) saveBackupMetadata(info BackupInfo) error {
	logger.Log.WithFields(map[string]interface{}{
		"original_path": info.OriginalPath,
		"backup_path":   info.BackupPath,
		"checksum":      info.Checksum,
		"size":          info.Size,
		"created_at":    info.CreatedAt,
		"backup_reason": info.BackupReason,
		"version":       info.Version,
	}).Debug("Backup metadata saved")

	return nil
}

// RemoteSyncBackups synchronizes local backups to remote server using Rsync over SSH
func (bm *BackupManager) RemoteSyncBackups(ctx context.Context) error {
	if !bm.config.Backup.RemoteBackup.Enabled {
		return nil // Remote backup is disabled
	}

	bm.remoteSyncMutex.Lock()
	defer bm.remoteSyncMutex.Unlock()

	startTime := time.Now()
	remoteConfig := bm.config.Backup.RemoteBackup

	logger.Log.WithFields(map[string]interface{}{
		"remote_host":     remoteConfig.Host,
		"remote_username": remoteConfig.Username,
		"remote_path":     remoteConfig.RemotePath,
		"component":       "backup",
		"event_type":      "remote_sync_start",
	}).Info("Starting remote backup synchronization")

	// Build rsync command
	cmd, err := bm.buildRsyncCommand()
	if err != nil {
		logger.LogError("backup", "build_rsync_cmd", err)
		return fmt.Errorf("failed to build rsync command: %w", err)
	}

	// Execute rsync with retries
	var lastErr error
	for attempt := 1; attempt <= remoteConfig.MaxRetries; attempt++ {
		logger.Log.WithFields(map[string]interface{}{
			"attempt":   attempt,
			"max_tries": remoteConfig.MaxRetries,
			"component": "backup",
		}).Info("Attempting remote sync")

		// Execute command
		cmd.Dir = bm.config.BackupPath
		output, err := cmd.CombinedOutput()

		if err == nil {
			// Success
			duration := time.Since(startTime)
			bm.lastRemoteSync = time.Now()

			logger.Log.WithFields(map[string]interface{}{
				"duration":    duration.String(),
				"duration_ms": duration.Milliseconds(),
				"attempt":     attempt,
				"output_size": len(output),
				"component":   "backup",
				"event_type":  "remote_sync_success",
			}).Info("Remote backup synchronization completed successfully")

			return nil
		}

		lastErr = err
		logger.Log.WithFields(map[string]interface{}{
			"attempt": attempt,
			"error":   err.Error(),
			"output":  string(output),
		}).Warn("Remote sync attempt failed")

		// Wait before retry (except on last attempt)
		if attempt < remoteConfig.MaxRetries {
			retryDelay := time.Duration(remoteConfig.RetryDelay) * time.Second
			logger.Log.WithField("retry_delay", retryDelay.String()).Info("Waiting before retry")

			select {
			case <-time.After(retryDelay):
				// Continue to next attempt
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	// All attempts failed
	duration := time.Since(startTime)
	logger.Log.WithFields(map[string]interface{}{
		"duration":    duration.String(),
		"attempts":    remoteConfig.MaxRetries,
		"final_error": lastErr.Error(),
		"component":   "backup",
		"event_type":  "remote_sync_failed",
	}).Error("Remote backup synchronization failed after all retries")

	return fmt.Errorf("remote sync failed after %d attempts: %w", remoteConfig.MaxRetries, lastErr)
}

// buildRsyncCommand builds the rsync command with proper SSH options
func (bm *BackupManager) buildRsyncCommand() (*exec.Cmd, error) {
	remoteConfig := bm.config.Backup.RemoteBackup

	// Base rsync options
	args := make([]string, 0)

	// Add configured rsync options
	args = append(args, remoteConfig.RsyncOptions...)

	// Add SSH options
	sshOptions := []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "ConnectTimeout=" + strconv.Itoa(remoteConfig.ConnectionTimeout),
		"-p", strconv.Itoa(remoteConfig.Port),
	}

	// Add SSH key if specified
	if remoteConfig.SSHKeyPath != "" {
		sshOptions = append(sshOptions, "-i", remoteConfig.SSHKeyPath)
	}

	// Add bandwidth limit if specified
	if remoteConfig.BandwidthLimit > 0 {
		args = append(args, "--bwlimit="+strconv.Itoa(remoteConfig.BandwidthLimit))
	}

	// Build SSH command string
	sshCmd := "ssh " + strings.Join(sshOptions, " ")
	args = append(args, "-e", sshCmd)

	// Add source and destination
	args = append(args, "./") // Source: current directory (backup path)
	destination := fmt.Sprintf("%s@%s:%s/", remoteConfig.Username, remoteConfig.Host, remoteConfig.RemotePath)
	args = append(args, destination)

	logger.Log.WithFields(map[string]interface{}{
		"command":     "rsync",
		"args":        args,
		"destination": destination,
		"component":   "backup",
	}).Debug("Built rsync command")

	return exec.Command("rsync", args...), nil
}

// ShouldPerformRemoteSync checks if remote sync should be performed based on interval
func (bm *BackupManager) ShouldPerformRemoteSync() bool {
	if !bm.config.Backup.RemoteBackup.Enabled {
		return false
	}

	// If sync interval is 0, sync after every backup
	if bm.config.Backup.RemoteBackup.SyncInterval == 0 {
		return true
	}

	// Check if enough time has passed since last sync
	syncInterval := time.Duration(bm.config.Backup.RemoteBackup.SyncInterval) * time.Minute
	return time.Since(bm.lastRemoteSync) >= syncInterval
}

// StartPeriodicRemoteSync starts a goroutine for periodic remote synchronization
func (bm *BackupManager) StartPeriodicRemoteSync(ctx context.Context) {
	if !bm.config.Backup.RemoteBackup.Enabled || bm.config.Backup.RemoteBackup.SyncInterval == 0 {
		return // Remote backup disabled or immediate sync mode
	}

	go func() {
		syncInterval := time.Duration(bm.config.Backup.RemoteBackup.SyncInterval) * time.Minute
		ticker := time.NewTicker(syncInterval)
		defer ticker.Stop()

		logger.Log.WithFields(map[string]interface{}{
			"sync_interval": syncInterval.String(),
			"component":     "backup",
		}).Info("Started periodic remote backup sync")

		for {
			select {
			case <-ctx.Done():
				logger.Log.Info("Stopping periodic remote backup sync")
				return
			case <-ticker.C:
				if err := bm.RemoteSyncBackups(ctx); err != nil {
					logger.LogError("backup", "periodic_remote_sync", err)
				}
			}
		}
	}()
}

// GetRemoteSyncStatus returns the status of remote synchronization
func (bm *BackupManager) GetRemoteSyncStatus() map[string]interface{} {
	if !bm.config.Backup.RemoteBackup.Enabled {
		return map[string]interface{}{
			"enabled": false,
			"status":  "disabled",
		}
	}

	status := map[string]interface{}{
		"enabled":       true,
		"host":          bm.config.Backup.RemoteBackup.Host,
		"username":      bm.config.Backup.RemoteBackup.Username,
		"remote_path":   bm.config.Backup.RemoteBackup.RemotePath,
		"sync_interval": bm.config.Backup.RemoteBackup.SyncInterval,
		"last_sync":     bm.lastRemoteSync.Format("2006-01-02 15:04:05"),
	}

	if bm.lastRemoteSync.IsZero() {
		status["status"] = "never_synced"
		status["last_sync"] = "never"
	} else {
		timeSinceSync := time.Since(bm.lastRemoteSync)
		status["time_since_last_sync"] = timeSinceSync.String()

		if bm.config.Backup.RemoteBackup.SyncInterval > 0 {
			syncInterval := time.Duration(bm.config.Backup.RemoteBackup.SyncInterval) * time.Minute
			if timeSinceSync < syncInterval {
				status["status"] = "up_to_date"
			} else {
				status["status"] = "sync_needed"
			}
		} else {
			status["status"] = "immediate_sync_mode"
		}
	}

	return status
}

// triggerRemoteSyncIfNeeded triggers remote sync if conditions are met
func (bm *BackupManager) triggerRemoteSyncIfNeeded(ctx context.Context) {
	if bm.ShouldPerformRemoteSync() {
		if err := bm.RemoteSyncBackups(ctx); err != nil {
			logger.LogError("backup", "trigger_remote_sync", err)
		}
	}
}
