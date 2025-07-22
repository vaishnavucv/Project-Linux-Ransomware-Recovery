package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"ransomware-recovery/pkg/backup"
	"ransomware-recovery/pkg/config"
	"ransomware-recovery/pkg/logger"
)

// RollbackManager handles file restoration operations
type RollbackManager struct {
	config        *config.Config
	backupManager *backup.BackupManager
	mu            sync.RWMutex
}

// RollbackResult represents the result of a rollback operation
type RollbackResult struct {
	Success       bool          `json:"success"`
	TotalFiles    int           `json:"total_files"`
	RestoredFiles int           `json:"restored_files"`
	FailedFiles   int           `json:"failed_files"`
	Duration      time.Duration `json:"duration"`
	Errors        []string      `json:"errors"`
	RestoredPaths []string      `json:"restored_paths"`
	FailedPaths   []string      `json:"failed_paths"`
	Timestamp     time.Time     `json:"timestamp"`
	// Remote rollback specific fields
	RollbackSource string `json:"rollback_source"` // "local", "remote", "hybrid"
	RemoteHost     string `json:"remote_host,omitempty"`
	TempStaging    string `json:"temp_staging,omitempty"`
}

// NewRollbackManager creates a new rollback manager
func NewRollbackManager(cfg *config.Config) *RollbackManager {
	return &RollbackManager{
		config:        cfg,
		backupManager: backup.NewBackupManager(cfg),
	}
}

// TriggerRollback initiates a full rollback operation including complete file structure restoration
func (rm *RollbackManager) TriggerRollback(ctx context.Context) (*RollbackResult, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	logger.Log.Warn("Initiating complete rollback operation with file structure restoration")
	startTime := time.Now()

	result := &RollbackResult{
		Success:        false,
		TotalFiles:     0,
		RestoredFiles:  0,
		FailedFiles:    0,
		Errors:         make([]string, 0),
		RestoredPaths:  make([]string, 0),
		FailedPaths:    make([]string, 0),
		Timestamp:      startTime,
		RollbackSource: "local", // Default to local
	}

	// Determine rollback source based on configuration and availability
	rollbackSource := rm.determineRollbackSource()
	result.RollbackSource = rollbackSource

	logger.Log.WithFields(map[string]interface{}{
		"rollback_source": rollbackSource,
		"component":       "rollback",
		"event_type":      "rollback_source_determined",
	}).Info("Determined rollback source")

	// Execute rollback based on source
	switch rollbackSource {
	case "remote":
		return rm.executeRemoteRollback(ctx, result)
	case "hybrid":
		return rm.executeHybridRollback(ctx, result)
	default:
		return rm.executeLocalRollback(ctx, result)
	}
}

// restoreResult represents the result of a single file restoration
type restoreResult struct {
	FilePath string
	Success  bool
	Error    string
}

// restoreWorker processes file restoration in parallel
func (rm *RollbackManager) restoreWorker(ctx context.Context, fileChan <-chan string, resultChan chan<- restoreResult, wg *sync.WaitGroup) {
	defer wg.Done()

	for filePath := range fileChan {
		select {
		case <-ctx.Done():
			return
		default:
			result := rm.restoreFile(ctx, filePath)
			resultChan <- result
		}
	}
}

// restoreFile restores a single file from backup
func (rm *RollbackManager) restoreFile(ctx context.Context, filePath string) restoreResult {
	// Create context with timeout for individual file restoration
	fileCtx, cancel := context.WithTimeout(ctx, rm.config.GetOperationTimeout())
	defer cancel()

	err := rm.backupManager.RestoreFile(fileCtx, filePath)
	if err != nil {
		return restoreResult{
			FilePath: filePath,
			Success:  false,
			Error:    err.Error(),
		}
	}

	return restoreResult{
		FilePath: filePath,
		Success:  true,
		Error:    "",
	}
}

// RestoreSpecificFiles restores only specific files from backup
func (rm *RollbackManager) RestoreSpecificFiles(ctx context.Context, filePaths []string) (*RollbackResult, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	logger.Log.WithField("file_count", len(filePaths)).Info("Starting selective file restoration")
	startTime := time.Now()

	result := &RollbackResult{
		Success:       false,
		TotalFiles:    len(filePaths),
		RestoredFiles: 0,
		FailedFiles:   0,
		Errors:        make([]string, 0),
		RestoredPaths: make([]string, 0),
		FailedPaths:   make([]string, 0),
		Timestamp:     startTime,
	}

	// Restore each file
	for _, filePath := range filePaths {
		select {
		case <-ctx.Done():
			result.Duration = time.Since(startTime)
			return result, ctx.Err()
		default:
			restoreRes := rm.restoreFile(ctx, filePath)
			if restoreRes.Success {
				result.RestoredFiles++
				result.RestoredPaths = append(result.RestoredPaths, restoreRes.FilePath)
				logger.LogRollback("restore", restoreRes.FilePath, true)
			} else {
				result.FailedFiles++
				result.FailedPaths = append(result.FailedPaths, restoreRes.FilePath)
				result.Errors = append(result.Errors, restoreRes.Error)
				logger.LogRollback("restore", restoreRes.FilePath, false)
			}
		}
	}

	result.Duration = time.Since(startTime)
	result.Success = result.RestoredFiles > 0

	logger.Log.WithFields(map[string]interface{}{
		"total_files":    result.TotalFiles,
		"restored_files": result.RestoredFiles,
		"failed_files":   result.FailedFiles,
		"duration":       result.Duration,
		"success":        result.Success,
	}).Info("Selective restoration completed")

	return result, nil
}

// RestoreDirectory restores all files in a specific directory from backup
func (rm *RollbackManager) RestoreDirectory(ctx context.Context, dirPath string) (*RollbackResult, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	logger.Log.WithField("directory", dirPath).Info("Starting directory restoration")
	startTime := time.Now()

	result := &RollbackResult{
		Success:       false,
		TotalFiles:    0,
		RestoredFiles: 0,
		FailedFiles:   0,
		Errors:        make([]string, 0),
		RestoredPaths: make([]string, 0),
		FailedPaths:   make([]string, 0),
		Timestamp:     startTime,
	}

	// Get all backup files
	allBackups, err := rm.backupManager.ListBackups()
	if err != nil {
		errorMsg := fmt.Sprintf("failed to list backup files: %v", err)
		logger.LogError("rollback", "list_backups", err)
		result.Errors = append(result.Errors, errorMsg)
		result.Duration = time.Since(startTime)
		return result, fmt.Errorf(errorMsg)
	}

	// Filter files in the specified directory
	var dirFiles []string
	for _, filePath := range allBackups {
		if strings.HasPrefix(filePath, dirPath) {
			dirFiles = append(dirFiles, filePath)
		}
	}

	if len(dirFiles) == 0 {
		logger.Log.WithField("directory", dirPath).Warn("No backup files found in directory")
		result.Duration = time.Since(startTime)
		return result, fmt.Errorf("no backup files found in directory: %s", dirPath)
	}

	// Restore the filtered files
	return rm.RestoreSpecificFiles(ctx, dirFiles)
}

// VerifyRestoration verifies that restored files match their backups
func (rm *RollbackManager) VerifyRestoration(ctx context.Context, filePaths []string) (map[string]bool, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	logger.Log.WithField("file_count", len(filePaths)).Info("Starting restoration verification")

	results := make(map[string]bool)

	for _, filePath := range filePaths {
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
			// For now, just check if the file exists and is readable
			// In a full implementation, you would compare checksums
			if _, err := os.Stat(filePath); err == nil {
				results[filePath] = true
			} else {
				results[filePath] = false
			}
		}
	}

	successCount := 0
	for _, success := range results {
		if success {
			successCount++
		}
	}

	logger.Log.WithFields(map[string]interface{}{
		"total_files":    len(filePaths),
		"verified_files": successCount,
		"failed_files":   len(filePaths) - successCount,
	}).Info("Restoration verification completed")

	return results, nil
}

// GetRollbackStats returns statistics about available backups
func (rm *RollbackManager) GetRollbackStats() (map[string]interface{}, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	backups, err := rm.backupManager.ListBackups()
	if err != nil {
		return nil, err
	}

	// Calculate backup sizes and count by directory
	totalSize := int64(0)
	dirCounts := make(map[string]int)

	for _, backupPath := range backups {
		// Get file size
		if info, err := os.Stat(backupPath); err == nil {
			totalSize += info.Size()
		}

		// Count by directory
		dir := filepath.Dir(backupPath)
		dirCounts[dir]++
	}

	return map[string]interface{}{
		"total_backups":     len(backups),
		"total_size_bytes":  totalSize,
		"directories":       dirCounts,
		"backup_path":       rm.config.BackupPath,
		"monitor_path":      rm.config.MonitorPath,
		"retention_days":    rm.config.Backup.RetentionDays,
		"checksums_enabled": rm.config.Backup.EnableChecksums,
	}, nil
}

// EmergencyStop forcefully stops any ongoing rollback operations
func (rm *RollbackManager) EmergencyStop() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	logger.Log.Warn("Emergency stop requested for rollback operations")
	// In a full implementation, you would cancel ongoing operations
	// For now, we just log the request
}

// IsRollbackInProgress checks if a rollback operation is currently running
func (rm *RollbackManager) IsRollbackInProgress() bool {
	// Try to acquire lock with timeout
	lockChan := make(chan bool, 1)
	go func() {
		rm.mu.Lock()
		rm.mu.Unlock()
		lockChan <- true
	}()

	select {
	case <-lockChan:
		return false
	case <-time.After(100 * time.Millisecond):
		return true
	}
}

// cleanupMaliciousFiles removes suspicious files before restoration
func (rm *RollbackManager) cleanupMaliciousFiles(ctx context.Context) error {
	logger.Log.Info("Cleaning up malicious files before restoration")

	suspiciousExtensions := []string{".locked", ".encrypted", ".crypto", ".crypt", ".enc"}
	suspiciousPatterns := []string{"RANSOMWARE", "DECRYPT", "BITCOIN", "PAYMENT"}

	cleanedCount := 0

	// Walk through monitor directory to find malicious files
	err := filepath.Walk(rm.config.MonitorPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue despite errors
		}

		// Skip directories and backup path
		if info.IsDir() || strings.HasPrefix(path, rm.config.BackupPath) {
			return nil
		}

		// Check for suspicious extensions
		isSuspiciousExt := false
		for _, ext := range suspiciousExtensions {
			if strings.HasSuffix(strings.ToLower(path), ext) {
				isSuspiciousExt = true
				break
			}
		}

		// Check for suspicious content in small files
		isSuspiciousContent := false
		if !isSuspiciousExt && info.Size() < 10*1024 { // Only check files < 10KB
			if content, err := os.ReadFile(path); err == nil {
				contentStr := string(content)
				for _, pattern := range suspiciousPatterns {
					if strings.Contains(strings.ToUpper(contentStr), pattern) {
						isSuspiciousContent = true
						break
					}
				}
			}
		}

		// Remove suspicious files
		if isSuspiciousExt || isSuspiciousContent {
			if err := os.Remove(path); err != nil {
				logger.LogError("rollback", "remove_malicious", err)
				logger.Log.WithFields(map[string]interface{}{
					"file_path": path,
					"error":     err.Error(),
				}).Warn("Failed to remove malicious file")
			} else {
				cleanedCount++
				logger.Log.WithFields(map[string]interface{}{
					"file_path":          path,
					"reason":             "suspicious_file",
					"suspicious_ext":     isSuspiciousExt,
					"suspicious_content": isSuspiciousContent,
				}).Info("Removed malicious file")
			}
		}

		return nil
	})

	logger.Log.WithFields(map[string]interface{}{
		"cleaned_files": cleanedCount,
		"component":     "rollback",
	}).Info("Malicious file cleanup completed")

	return err
}

// recreateDirectoryStructure ensures all necessary directories exist
func (rm *RollbackManager) recreateDirectoryStructure(ctx context.Context) error {
	logger.Log.Info("Recreating directory structure from backups")

	// Get all backup files to understand directory structure
	backupFiles, err := rm.backupManager.ListBackups()
	if err != nil {
		return fmt.Errorf("failed to list backup files: %w", err)
	}

	// Extract unique directories that need to be created
	dirSet := make(map[string]bool)
	for _, filePath := range backupFiles {
		dir := filepath.Dir(filePath)
		// Add all parent directories
		for dir != "." && dir != "/" && dir != rm.config.MonitorPath {
			dirSet[dir] = true
			dir = filepath.Dir(dir)
		}
	}

	// Create directories
	createdDirs := 0
	for dir := range dirSet {
		if err := os.MkdirAll(dir, 0755); err != nil {
			logger.LogError("rollback", "create_directory", err)
			logger.Log.WithFields(map[string]interface{}{
				"directory": dir,
				"error":     err.Error(),
			}).Warn("Failed to create directory")
		} else {
			createdDirs++
			logger.Log.WithField("directory", dir).Debug("Created directory")
		}
	}

	logger.Log.WithFields(map[string]interface{}{
		"created_directories": createdDirs,
		"total_directories":   len(dirSet),
		"component":           "rollback",
	}).Info("Directory structure recreation completed")

	return nil
}

// determineRollbackSource determines the best rollback source based on configuration and availability
func (rm *RollbackManager) determineRollbackSource() string {
	remoteConfig := rm.config.Backup.RemoteBackup

	// Check if remote rollback is enabled and configured
	if !remoteConfig.Enabled || !remoteConfig.RemoteRollback.Enabled {
		return "local"
	}

	// Check if we can connect to remote server
	if !rm.isRemoteAvailable() {
		logger.Log.Warn("Remote server not available, falling back to local rollback")
		return "local"
	}

	// Check if we prefer remote rollback
	if remoteConfig.RemoteRollback.PreferRemote {
		return "remote"
	}

	// Check if we have local backups
	localBackups, err := rm.backupManager.ListBackups()
	if err != nil || len(localBackups) == 0 {
		logger.Log.Info("No local backups available, using remote rollback")
		return "remote"
	}

	// Use hybrid approach (remote + local verification)
	return "hybrid"
}

// isRemoteAvailable checks if the remote backup server is accessible
func (rm *RollbackManager) isRemoteAvailable() bool {
	remoteConfig := rm.config.Backup.RemoteBackup

	// Build a simple SSH test command
	sshCmd := fmt.Sprintf("ssh -o ConnectTimeout=5 -o StrictHostKeyChecking=no")
	if remoteConfig.SSHKeyPath != "" {
		sshCmd += fmt.Sprintf(" -i %s", remoteConfig.SSHKeyPath)
	}
	sshCmd += fmt.Sprintf(" -p %d %s@%s", remoteConfig.Port, remoteConfig.Username, remoteConfig.Host)
	sshCmd += " 'echo test'"

	cmd := exec.Command("sh", "-c", sshCmd)
	err := cmd.Run()

	available := err == nil
	logger.Log.WithFields(map[string]interface{}{
		"remote_host": remoteConfig.Host,
		"available":   available,
		"component":   "rollback",
	}).Debug("Remote server availability check")

	return available
}

// executeLocalRollback performs rollback using local backups
func (rm *RollbackManager) executeLocalRollback(ctx context.Context, result *RollbackResult) (*RollbackResult, error) {
	startTime := result.Timestamp

	logger.Log.WithFields(map[string]interface{}{
		"component":  "rollback",
		"event_type": "local_rollback_start",
	}).Info("Starting local rollback operation")

	// Step 1: Clean up any malicious files before restoration
	if err := rm.cleanupMaliciousFiles(ctx); err != nil {
		logger.LogError("rollback", "cleanup_malicious", err)
		// Continue with rollback even if cleanup fails
	}

	// Step 2: Recreate directory structure
	if err := rm.recreateDirectoryStructure(ctx); err != nil {
		logger.LogError("rollback", "recreate_directories", err)
		// Continue with rollback even if directory creation fails
	}

	// Step 3: Get list of backup files
	backupFiles, err := rm.backupManager.ListBackups()
	if err != nil {
		logger.LogError("rollback", "list_backups", err)
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to list backup files: %v", err))
		result.Duration = time.Since(startTime)
		return result, err
	}

	result.TotalFiles = len(backupFiles)
	logger.Log.WithField("total_files", result.TotalFiles).Info("Found backup files for rollback")

	if result.TotalFiles == 0 {
		logger.Log.Warn("No backup files found for rollback")
		result.Duration = time.Since(startTime)
		return result, fmt.Errorf("no backup files found")
	}

	// Step 4: Restore files using parallel workers
	fileChan := make(chan string, len(backupFiles))
	resultChan := make(chan restoreResult, len(backupFiles))

	// Start worker goroutines
	var wg sync.WaitGroup
	workerCount := rm.config.System.WorkerCount
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go rm.restoreWorker(ctx, fileChan, resultChan, &wg)
	}

	// Send files to workers
	for _, filePath := range backupFiles {
		select {
		case fileChan <- filePath:
		case <-ctx.Done():
			close(fileChan)
			wg.Wait()
			result.Duration = time.Since(startTime)
			return result, ctx.Err()
		}
	}
	close(fileChan)

	// Wait for all workers to complete
	wg.Wait()
	close(resultChan)

	// Collect results
	for restoreRes := range resultChan {
		if restoreRes.Success {
			result.RestoredFiles++
			result.RestoredPaths = append(result.RestoredPaths, restoreRes.FilePath)
		} else {
			result.FailedFiles++
			result.FailedPaths = append(result.FailedPaths, restoreRes.FilePath)
			result.Errors = append(result.Errors, restoreRes.Error)
		}
	}

	// Determine overall success
	result.Success = result.FailedFiles == 0
	result.Duration = time.Since(startTime)

	// Log final results
	logger.LogRollback("complete", rm.config.MonitorPath, result.Success)
	logger.Log.WithFields(map[string]interface{}{
		"total_files":    result.TotalFiles,
		"restored_files": result.RestoredFiles,
		"failed_files":   result.FailedFiles,
		"duration":       result.Duration,
		"success":        result.Success,
		"rollback_type":  "local",
	}).Info("Local rollback operation completed")

	return result, nil
}

// executeRemoteRollback performs rollback using remote backups via Rsync over SSH
func (rm *RollbackManager) executeRemoteRollback(ctx context.Context, result *RollbackResult) (*RollbackResult, error) {
	startTime := result.Timestamp
	remoteConfig := rm.config.Backup.RemoteBackup

	result.RemoteHost = remoteConfig.Host
	result.TempStaging = remoteConfig.RemoteRollback.TempDir

	logger.Log.WithFields(map[string]interface{}{
		"component":   "rollback",
		"event_type":  "remote_rollback_start",
		"remote_host": remoteConfig.Host,
		"remote_path": remoteConfig.RemotePath,
	}).Info("Starting remote rollback operation")

	// Step 1: Create temp staging directory
	if err := rm.createTempStagingDir(result.TempStaging); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to create temp staging directory: %v", err))
		result.Duration = time.Since(startTime)
		return result, err
	}
	defer rm.cleanupTempStaging(result.TempStaging)

	// Step 2: Backup current state if configured
	if remoteConfig.RemoteRollback.BackupBeforeRollback {
		if err := rm.backupCurrentState(ctx); err != nil {
			logger.Log.WithError(err).Warn("Failed to backup current state before remote rollback")
			// Continue with rollback
		}
	}

	// Step 3: Download remote backups to staging area
	if err := rm.downloadRemoteBackups(ctx, result); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to download remote backups: %v", err))
		result.Duration = time.Since(startTime)
		return result, err
	}

	// Step 4: Clean up malicious files
	if err := rm.cleanupMaliciousFiles(ctx); err != nil {
		logger.LogError("rollback", "cleanup_malicious", err)
		// Continue with rollback even if cleanup fails
	}

	// Step 5: Restore files from staging area to monitor path
	if err := rm.restoreFromStaging(ctx, result); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to restore from staging: %v", err))
		result.Duration = time.Since(startTime)
		return result, err
	}

	// Step 6: Verify integrity if configured
	if remoteConfig.RemoteRollback.VerifyIntegrity {
		if err := rm.verifyRemoteRollbackIntegrity(ctx, result); err != nil {
			logger.Log.WithError(err).Warn("Integrity verification failed after remote rollback")
			// Don't fail the rollback for verification issues
		}
	}

	// Determine overall success
	result.Success = result.FailedFiles == 0
	result.Duration = time.Since(startTime)

	// Log final results
	logger.LogRollback("complete", rm.config.MonitorPath, result.Success)
	logger.Log.WithFields(map[string]interface{}{
		"total_files":    result.TotalFiles,
		"restored_files": result.RestoredFiles,
		"failed_files":   result.FailedFiles,
		"duration":       result.Duration,
		"success":        result.Success,
		"rollback_type":  "remote",
		"remote_host":    result.RemoteHost,
	}).Info("Remote rollback operation completed")

	return result, nil
}

// executeHybridRollback performs rollback using both remote and local backups for redundancy
func (rm *RollbackManager) executeHybridRollback(ctx context.Context, result *RollbackResult) (*RollbackResult, error) {
	logger.Log.WithFields(map[string]interface{}{
		"component":  "rollback",
		"event_type": "hybrid_rollback_start",
	}).Info("Starting hybrid rollback operation")

	// Try remote rollback first
	remoteResult, remoteErr := rm.executeRemoteRollback(ctx, result)

	if remoteErr == nil && remoteResult.Success {
		// Remote rollback succeeded
		remoteResult.RollbackSource = "hybrid-remote"
		logger.Log.Info("Hybrid rollback completed successfully using remote source")
		return remoteResult, nil
	}

	// Remote rollback failed, try local rollback
	logger.Log.WithError(remoteErr).Warn("Remote rollback failed, attempting local rollback")

	// Reset result for local rollback attempt
	result.Errors = make([]string, 0)
	result.RestoredPaths = make([]string, 0)
	result.FailedPaths = make([]string, 0)
	result.RestoredFiles = 0
	result.FailedFiles = 0

	localResult, localErr := rm.executeLocalRollback(ctx, result)
	if localErr == nil {
		localResult.RollbackSource = "hybrid-local"
		logger.Log.Info("Hybrid rollback completed successfully using local source")
		return localResult, nil
	}

	// Both failed
	result.Success = false
	result.RollbackSource = "hybrid-failed"
	result.Errors = append(result.Errors, fmt.Sprintf("Remote rollback failed: %v", remoteErr))
	result.Errors = append(result.Errors, fmt.Sprintf("Local rollback failed: %v", localErr))
	result.Duration = time.Since(result.Timestamp)

	logger.Log.WithFields(map[string]interface{}{
		"remote_error": remoteErr.Error(),
		"local_error":  localErr.Error(),
		"component":    "rollback",
		"event_type":   "hybrid_rollback_failed",
	}).Error("Both remote and local rollback attempts failed")

	return result, fmt.Errorf("both remote and local rollback failed")
}

// createTempStagingDir creates a temporary directory for staging remote files
func (rm *RollbackManager) createTempStagingDir(tempDir string) error {
	if err := os.RemoveAll(tempDir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to clean existing temp directory: %w", err)
	}

	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp staging directory: %w", err)
	}

	logger.Log.WithFields(map[string]interface{}{
		"temp_dir":   tempDir,
		"component":  "rollback",
		"event_type": "temp_staging_created",
	}).Info("Created temporary staging directory")

	return nil
}

// cleanupTempStaging removes the temporary staging directory
func (rm *RollbackManager) cleanupTempStaging(tempDir string) {
	if err := os.RemoveAll(tempDir); err != nil {
		logger.Log.WithError(err).WithField("temp_dir", tempDir).Warn("Failed to cleanup temp staging directory")
	} else {
		logger.Log.WithField("temp_dir", tempDir).Info("Cleaned up temporary staging directory")
	}
}

// backupCurrentState creates a backup of the current state before remote rollback
func (rm *RollbackManager) backupCurrentState(ctx context.Context) error {
	backupDir := fmt.Sprintf("%s-pre-remote-rollback-%d", rm.config.BackupPath, time.Now().Unix())

	cmd := exec.Command("cp", "-r", rm.config.MonitorPath, backupDir)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to backup current state: %w", err)
	}

	logger.Log.WithFields(map[string]interface{}{
		"backup_dir": backupDir,
		"component":  "rollback",
		"event_type": "current_state_backed_up",
	}).Info("Backed up current state before remote rollback")

	return nil
}

// downloadRemoteBackups downloads backup files from remote server to staging area
func (rm *RollbackManager) downloadRemoteBackups(ctx context.Context, result *RollbackResult) error {
	remoteConfig := rm.config.Backup.RemoteBackup

	// Build rsync command to download from remote
	cmd, err := rm.buildRemoteDownloadCommand(result.TempStaging)
	if err != nil {
		return fmt.Errorf("failed to build download command: %w", err)
	}

	logger.Log.WithFields(map[string]interface{}{
		"component":   "rollback",
		"event_type":  "remote_download_start",
		"remote_host": remoteConfig.Host,
		"temp_dir":    result.TempStaging,
	}).Info("Starting download of remote backups")

	// Execute rsync download with retries
	var lastErr error
	for attempt := 1; attempt <= remoteConfig.MaxRetries; attempt++ {
		logger.Log.WithFields(map[string]interface{}{
			"attempt":   attempt,
			"max_tries": remoteConfig.MaxRetries,
			"component": "rollback",
		}).Info("Attempting remote backup download")

		output, err := cmd.CombinedOutput()
		if err == nil {
			logger.Log.WithFields(map[string]interface{}{
				"component":   "rollback",
				"event_type":  "remote_download_success",
				"attempt":     attempt,
				"output_size": len(output),
			}).Info("Remote backup download completed successfully")
			return nil
		}

		lastErr = err
		logger.Log.WithFields(map[string]interface{}{
			"attempt": attempt,
			"error":   err.Error(),
			"output":  string(output),
		}).Warn("Remote download attempt failed")

		// Wait before retry (except on last attempt)
		if attempt < remoteConfig.MaxRetries {
			retryDelay := time.Duration(remoteConfig.RetryDelay) * time.Second
			logger.Log.WithField("retry_delay", retryDelay.String()).Info("Waiting before retry")
			time.Sleep(retryDelay)
		}
	}

	return fmt.Errorf("remote download failed after %d attempts: %w", remoteConfig.MaxRetries, lastErr)
}

// buildRemoteDownloadCommand builds the rsync command for downloading from remote server
func (rm *RollbackManager) buildRemoteDownloadCommand(tempDir string) (*exec.Cmd, error) {
	remoteConfig := rm.config.Backup.RemoteBackup

	// Base rsync options for download
	args := make([]string, 0)
	args = append(args, remoteConfig.RemoteRollback.RollbackOptions...)

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

	// Add source (remote) and destination (local temp)
	source := fmt.Sprintf("%s@%s:%s/", remoteConfig.Username, remoteConfig.Host, remoteConfig.RemotePath)
	args = append(args, source, tempDir+"/")

	logger.Log.WithFields(map[string]interface{}{
		"command": "rsync",
		"args":    args,
		"source":  source,
		"dest":    tempDir,
	}).Debug("Built remote download command")

	return exec.Command("rsync", args...), nil
}

// restoreFromStaging restores files from staging area to monitor path
func (rm *RollbackManager) restoreFromStaging(ctx context.Context, result *RollbackResult) error {
	// Use rsync to efficiently copy from staging to monitor path
	cmd := exec.Command("rsync", "-av", "--delete", result.TempStaging+"/", rm.config.MonitorPath+"/")

	logger.Log.WithFields(map[string]interface{}{
		"component":  "rollback",
		"event_type": "staging_restore_start",
		"source":     result.TempStaging,
		"dest":       rm.config.MonitorPath,
	}).Info("Starting restore from staging area")

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Log.WithFields(map[string]interface{}{
			"error":  err.Error(),
			"output": string(output),
		}).Error("Failed to restore from staging area")
		return fmt.Errorf("failed to restore from staging: %w", err)
	}

	// Count restored files (simple estimation from rsync output)
	lines := strings.Split(string(output), "\n")
	fileCount := 0
	for _, line := range lines {
		if line != "" && !strings.HasSuffix(line, "/") && !strings.Contains(line, "total size") {
			fileCount++
		}
	}

	result.TotalFiles = fileCount
	result.RestoredFiles = fileCount
	result.FailedFiles = 0

	logger.Log.WithFields(map[string]interface{}{
		"component":      "rollback",
		"event_type":     "staging_restore_success",
		"restored_files": fileCount,
	}).Info("Successfully restored files from staging area")

	return nil
}

// verifyRemoteRollbackIntegrity verifies the integrity of files after remote rollback
func (rm *RollbackManager) verifyRemoteRollbackIntegrity(ctx context.Context, result *RollbackResult) error {
	logger.Log.Info("Starting integrity verification after remote rollback")

	// This is a simple implementation - in production you might want more sophisticated verification
	verifiedCount := 0
	failedCount := 0

	err := filepath.Walk(rm.config.MonitorPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			// Simple existence check - could be enhanced with checksum verification
			if _, err := os.Stat(path); err == nil {
				verifiedCount++
			} else {
				failedCount++
			}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("integrity verification failed: %w", err)
	}

	logger.Log.WithFields(map[string]interface{}{
		"component":      "rollback",
		"event_type":     "integrity_verification_complete",
		"verified_files": verifiedCount,
		"failed_files":   failedCount,
	}).Info("Completed integrity verification")

	if failedCount > 0 {
		return fmt.Errorf("integrity verification failed for %d files", failedCount)
	}

	return nil
}
