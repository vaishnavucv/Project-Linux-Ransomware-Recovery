package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Config holds all configuration settings for the ransomware recovery system
type Config struct {
	// Monitoring settings
	MonitorPath   string `json:"monitor_path"`
	BackupPath    string `json:"backup_path"`
	LogPath       string `json:"log_path"`
	
	// Detection settings
	Detection DetectionConfig `json:"detection"`
	
	// Backup settings
	Backup BackupConfig `json:"backup"`
	
	// System settings
	System SystemConfig `json:"system"`
}

// DetectionConfig holds detection-related configuration
type DetectionConfig struct {
	// Suspicious file extensions to detect
	SuspiciousExtensions []string `json:"suspicious_extensions"`
	
	// Threshold for triggering rollback (number of suspicious files)
	Threshold int `json:"threshold"`
	
	// Time window for counting suspicious events (in seconds)
	TimeWindow int `json:"time_window"`
	
	// Enable real-time detection
	RealTimeDetection bool `json:"real_time_detection"`
	
	// Patterns to detect in file content
	ContentPatterns []string `json:"content_patterns"`
}

// BackupConfig holds backup-related configuration
type BackupConfig struct {
	// Enable automatic backup on file creation/modification
	AutoBackup bool `json:"auto_backup"`
	
	// Backup retention period (in days)
	RetentionDays int `json:"retention_days"`
	
	// Enable checksums for backup verification
	EnableChecksums bool `json:"enable_checksums"`
	
	// Compress backups
	Compression bool `json:"compression"`
}

// SystemConfig holds system-related configuration
type SystemConfig struct {
	// Number of worker goroutines for file processing
	WorkerCount int `json:"worker_count"`
	
	// Buffer size for file event channels
	BufferSize int `json:"buffer_size"`
	
	// Timeout for operations (in seconds)
	OperationTimeout int `json:"operation_timeout"`
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		MonitorPath: "./demo-folder",
		BackupPath:  "./.backup",
		LogPath:     "./logs",
		Detection: DetectionConfig{
			SuspiciousExtensions: []string{".locked", ".encrypted", ".crypto", ".crypt", ".enc"},
			Threshold:           3,
			TimeWindow:          60,
			RealTimeDetection:   true,
			ContentPatterns:     []string{"RANSOMWARE", "DECRYPT", "BITCOIN", "PAYMENT"},
		},
		Backup: BackupConfig{
			AutoBackup:      true,
			RetentionDays:   30,
			EnableChecksums: true,
			Compression:     false,
		},
		System: SystemConfig{
			WorkerCount:      4,
			BufferSize:       100,
			OperationTimeout: 30,
		},
	}
}

// LoadConfig loads configuration from file or returns default if file doesn't exist
func LoadConfig(configPath string) (*Config, error) {
	// If config file doesn't exist, create it with default values
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		config := DefaultConfig()
		if err := config.SaveConfig(configPath); err != nil {
			return nil, err
		}
		return config, nil
	}

	// Read existing config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// SaveConfig saves the configuration to file
func (c *Config) SaveConfig(configPath string) error {
	// Create directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

// GetOperationTimeout returns the operation timeout as a time.Duration
func (c *Config) GetOperationTimeout() time.Duration {
	return time.Duration(c.System.OperationTimeout) * time.Second
}

// GetTimeWindow returns the detection time window as a time.Duration
func (c *Config) GetTimeWindow() time.Duration {
	return time.Duration(c.Detection.TimeWindow) * time.Second
}

// IsSuspiciousExtension checks if a file extension is considered suspicious
func (c *Config) IsSuspiciousExtension(ext string) bool {
	for _, suspiciousExt := range c.Detection.SuspiciousExtensions {
		if ext == suspiciousExt {
			return true
		}
	}
	return false
}

// HasSuspiciousContent checks if content contains suspicious patterns
func (c *Config) HasSuspiciousContent(content string) bool {
	for _, pattern := range c.Detection.ContentPatterns {
		if len(pattern) > 0 && len(content) > 0 {
			// Simple case-insensitive substring search
			// In production, you might want to use regex or more sophisticated matching
			if containsIgnoreCase(content, pattern) {
				return true
			}
		}
	}
	return false
}

// containsIgnoreCase performs case-insensitive substring search
func containsIgnoreCase(s, substr string) bool {
	s = strings.ToLower(s)
	substr = strings.ToLower(substr)
	return strings.Contains(s, substr)
} 