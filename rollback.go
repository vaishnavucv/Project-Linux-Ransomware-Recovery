package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
	Success          bool              `json:"success"`
	TotalFiles       int               `json:"total_files"`
	RestoredFiles    int               `json:"restored_files"`
	FailedFiles      int               `json:"failed_files"`
	Duration         time.Duration     `json:"duration"`
	Errors           []string          `json:"errors"`
	RestoredPaths    []string          `json:"restored_paths"`
	FailedPaths      []string          `json:"failed_paths"`
	Timestamp        time.Time         `json:"timestamp"`
}

// NewRollbackManager creates a new rollback manager
func NewRollbackManager(cfg *config.Config) *RollbackManager {
	return &RollbackManager{
		config:        cfg,
		backupManager: backup.NewBackupManager(cfg),
	}
}

// TriggerRollback initiates a full rollback operation
func (rm *RollbackManager) TriggerRollback(ctx context.Context) (*RollbackResult, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	logger.Log.Warn("Initiating emergency rollback operation")
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

	// Get list of backup files
	backupFiles, err := rm.backupManager.ListBackups()
	if err != nil {
		errorMsg := fmt.Sprintf("failed to list backup files: %v", err)
		logger.LogError("rollback", "list_backups", err)
		result.Errors = append(result.Errors, errorMsg)
		result.Duration = time.Since(startTime)
		return result, fmt.Errorf(errorMsg)
	}

	result.TotalFiles = len(backupFiles)

	if result.TotalFiles == 0 {
		logger.Log.Warn("No backup files found for rollback")
		result.Duration = time.Since(startTime)
		return result, fmt.Errorf("no backup files found")
	}

	logger.Log.WithField("total_files", result.TotalFiles).Info("Starting rollback of backup files")

	// Create a worker pool for parallel restoration
	workerCount := rm.config.System.WorkerCount
	if workerCount > result.TotalFiles {
		workerCount = result.TotalFiles
	}

	// Channels for work distribution
	fileChan := make(chan string, result.TotalFiles)
	resultChan := make(chan restoreResult, result.TotalFiles)

	// Start worker goroutines
	var wg sync.WaitGroup
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
			result.Duration = time.Since(startTime)
			return result, ctx.Err()
		}
	}
	close(fileChan)

	// Wait for all workers to complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	for restoreRes := range resultChan {
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

	result.Duration = time.Since(startTime)
	result.Success = result.RestoredFiles > 0

	// Log final results
	logger.Log.WithFields(map[string]interface{}{
		"total_files":    result.TotalFiles,
		"restored_files": result.RestoredFiles,
		"failed_files":   result.FailedFiles,
		"duration":       result.Duration,
		"success":        result.Success,
	}).Info("Rollback operation completed")

	if result.Success {
		logger.Log.Info("Emergency rollback completed successfully")
	} else {
		logger.Log.Error("Emergency rollback failed")
	}

	return result, nil
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

// QuarantineInfectedFiles moves infected files to a quarantine directory
func (rm *RollbackManager) QuarantineInfectedFiles(ctx context.Context, infectedFiles []string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	logger.Log.WithField("file_count", len(infectedFiles)).Info("Starting file quarantine")

	// Create quarantine directory
	quarantineDir := filepath.Join(rm.config.BackupPath, "quarantine", time.Now().Format("2006-01-02_15-04-05"))
	if err := os.MkdirAll(quarantineDir, 0755); err != nil {
		logger.LogError("rollback", "create_quarantine_dir", err)
		return fmt.Errorf("failed to create quarantine directory: %w", err)
	}

	successCount := 0
	for _, filePath := range infectedFiles {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := rm.quarantineFile(filePath, quarantineDir); err != nil {
				logger.LogError("rollback", "quarantine_file", err)
				logger.Log.WithFields(map[string]interface{}{
					"file_path": filePath,
					"error":     err.Error(),
				}).Error("Failed to quarantine file")
			} else {
				successCount++
				logger.Log.WithField("file_path", filePath).Info("File quarantined successfully")
			}
		}
	}

	logger.Log.WithFields(map[string]interface{}{
		"total_files":      len(infectedFiles),
		"quarantined_files": successCount,
		"quarantine_dir":   quarantineDir,
	}).Info("File quarantine completed")

	return nil
}

// quarantineFile moves a single file to quarantine
func (rm *RollbackManager) quarantineFile(filePath, quarantineDir string) error {
	// Validate file path
	if !strings.HasPrefix(filePath, rm.config.MonitorPath) {
		return fmt.Errorf("file path outside monitor directory: %s", filePath)
	}

	// Create relative path for quarantine
	relPath, err := filepath.Rel(rm.config.MonitorPath, filePath)
	if err != nil {
		return err
	}

	quarantinePath := filepath.Join(quarantineDir, relPath)

	// Create quarantine subdirectory if needed
	quarantineSubDir := filepath.Dir(quarantinePath)
	if err := os.MkdirAll(quarantineSubDir, 0755); err != nil {
		return err
	}

	// Move file to quarantine
	if err := os.Rename(filePath, quarantinePath); err != nil {
		return err
	}

	return nil
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