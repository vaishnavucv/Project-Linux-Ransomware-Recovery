package backup

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"ransomware-recovery/pkg/config"
	"ransomware-recovery/pkg/logger"
)

// BackupManager handles file backup operations
type BackupManager struct {
	config *config.Config
	mu     sync.RWMutex
}

// BackupInfo holds metadata about a backup
type BackupInfo struct {
	OriginalPath string    `json:"original_path"`
	BackupPath   string    `json:"backup_path"`
	Checksum     string    `json:"checksum"`
	Size         int64     `json:"size"`
	CreatedAt    time.Time `json:"created_at"`
}

// NewBackupManager creates a new backup manager
func NewBackupManager(cfg *config.Config) *BackupManager {
	return &BackupManager{
		config: cfg,
	}
}

// BackupFile creates a backup of the specified file
func (bm *BackupManager) BackupFile(ctx context.Context, filePath string) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	// Validate file path to prevent path traversal attacks
	if err := bm.validateFilePath(filePath); err != nil {
		logger.LogError("backup", "validate_path", err)
		return fmt.Errorf("invalid file path: %w", err)
	}

	// Check if file exists and is readable
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		logger.LogError("backup", "stat_file", err)
		return fmt.Errorf("failed to stat file: %w", err)
	}

	// Skip directories
	if fileInfo.IsDir() {
		return nil
	}

	// Create backup path
	backupPath, err := bm.createBackupPath(filePath)
	if err != nil {
		logger.LogError("backup", "create_backup_path", err)
		return fmt.Errorf("failed to create backup path: %w", err)
	}

	// Create backup directory if it doesn't exist
	backupDir := filepath.Dir(backupPath)
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		logger.LogError("backup", "create_backup_dir", err)
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Copy file with checksum verification
	checksum, err := bm.copyFileWithChecksum(ctx, filePath, backupPath)
	if err != nil {
		logger.LogError("backup", "copy_file", err)
		return fmt.Errorf("failed to copy file: %w", err)
	}

	// Create backup info
	backupInfo := BackupInfo{
		OriginalPath: filePath,
		BackupPath:   backupPath,
		Checksum:     checksum,
		Size:         fileInfo.Size(),
		CreatedAt:    time.Now(),
	}

	// Log successful backup
	logger.LogBackup("create", filePath, backupPath, true)

	// Save backup metadata (optional, for future enhancements)
	if err := bm.saveBackupMetadata(backupInfo); err != nil {
		logger.LogError("backup", "save_metadata", err)
		// Don't fail the backup operation for metadata errors
	}

	return nil
}

// RestoreFile restores a file from backup
func (bm *BackupManager) RestoreFile(ctx context.Context, originalPath string) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	backupPath, err := bm.createBackupPath(originalPath)
	if err != nil {
		logger.LogError("backup", "create_backup_path", err)
		return fmt.Errorf("failed to create backup path: %w", err)
	}

	// Check if backup exists
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return fmt.Errorf("backup not found for file: %s", originalPath)
	}

	// Verify backup integrity if checksums are enabled
	if bm.config.Backup.EnableChecksums {
		if err := bm.verifyBackupIntegrity(backupPath); err != nil {
			logger.LogError("backup", "verify_integrity", err)
			return fmt.Errorf("backup integrity check failed: %w", err)
		}
	}

	// Create directory for original file if it doesn't exist
	originalDir := filepath.Dir(originalPath)
	if err := os.MkdirAll(originalDir, 0755); err != nil {
		logger.LogError("backup", "create_original_dir", err)
		return fmt.Errorf("failed to create original directory: %w", err)
	}

	// Copy backup back to original location
	if err := bm.copyFile(ctx, backupPath, originalPath); err != nil {
		logger.LogError("backup", "restore_file", err)
		return fmt.Errorf("failed to restore file: %w", err)
	}

	logger.LogBackup("restore", backupPath, originalPath, true)
	return nil
}

// ListBackups returns a list of all backup files
func (bm *BackupManager) ListBackups() ([]string, error) {
	var backups []string

	err := filepath.Walk(bm.config.BackupPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			// Convert backup path back to original path
			relPath, err := filepath.Rel(bm.config.BackupPath, path)
			if err != nil {
				return err
			}
			originalPath := filepath.Join(bm.config.MonitorPath, relPath)
			backups = append(backups, originalPath)
		}

		return nil
	})

	return backups, err
}

// CleanupOldBackups removes backups older than the retention period
func (bm *BackupManager) CleanupOldBackups(ctx context.Context) error {
	cutoffTime := time.Now().AddDate(0, 0, -bm.config.Backup.RetentionDays)

	return filepath.Walk(bm.config.BackupPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && info.ModTime().Before(cutoffTime) {
			if err := os.Remove(path); err != nil {
				logger.LogError("backup", "cleanup_old", err)
				return err
			}
			logger.LogBackup("cleanup", path, "", true)
		}

		return nil
	})
}

// createBackupPath creates the backup path for a given file
func (bm *BackupManager) createBackupPath(filePath string) (string, error) {
	// Get relative path from monitor directory
	relPath, err := filepath.Rel(bm.config.MonitorPath, filePath)
	if err != nil {
		return "", err
	}

	// Create backup path
	backupPath := filepath.Join(bm.config.BackupPath, relPath)
	return backupPath, nil
}

// copyFileWithChecksum copies a file and returns its checksum
func (bm *BackupManager) copyFileWithChecksum(ctx context.Context, src, dst string) (string, error) {
	sourceFile, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return "", err
	}
	defer destFile.Close()

	// Create hash writer
	hash := sha256.New()
	multiWriter := io.MultiWriter(destFile, hash)

	// Copy with context cancellation support
	done := make(chan error, 1)
	go func() {
		_, err := io.Copy(multiWriter, sourceFile)
		done <- err
	}()

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case err := <-done:
		if err != nil {
			return "", err
		}
	}

	checksum := fmt.Sprintf("%x", hash.Sum(nil))
	return checksum, nil
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
		return err
	}
}

// validateFilePath validates that the file path is safe
func (bm *BackupManager) validateFilePath(filePath string) error {
	// Clean the path to resolve any .. or . elements
	cleanPath := filepath.Clean(filePath)
	
	// Check for path traversal attempts
	if strings.Contains(cleanPath, "..") {
		return fmt.Errorf("path traversal detected: %s", filePath)
	}

	// Ensure path is within the monitor directory
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
func (bm *BackupManager) verifyBackupIntegrity(backupPath string) error {
	// This is a placeholder for backup integrity verification
	// In a full implementation, you would compare against stored checksums
	file, err := os.Open(backupPath)
	if err != nil {
		return err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}

	// For now, just verify the file is readable and has content
	stat, err := file.Stat()
	if err != nil {
		return err
	}

	if stat.Size() == 0 {
		return fmt.Errorf("backup file is empty: %s", backupPath)
	}

	return nil
}

// saveBackupMetadata saves backup metadata for future reference
func (bm *BackupManager) saveBackupMetadata(info BackupInfo) error {
	// This is a placeholder for metadata storage
	// In a full implementation, you might store this in a database or index file
	logger.Log.WithFields(map[string]interface{}{
		"original_path": info.OriginalPath,
		"backup_path":   info.BackupPath,
		"checksum":      info.Checksum,
		"size":          info.Size,
		"created_at":    info.CreatedAt,
	}).Info("Backup metadata saved")

	return nil
} 