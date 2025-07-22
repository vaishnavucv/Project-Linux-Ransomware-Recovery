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
	MonitorPath string `json:"monitor_path"`
	BackupPath  string `json:"backup_path"`
	LogPath     string `json:"log_path"`

	// Detection settings
	Detection DetectionConfig `json:"detection"`

	// Backup settings
	Backup BackupConfig `json:"backup"`

	// System settings
	System SystemConfig `json:"system"`

	// Logging settings
	Logging LoggingConfig `json:"logging"`
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

	// Deletion alert settings
	DeletionAlert DeletionAlertConfig `json:"deletion_alert"`
}

// DeletionAlertConfig holds deletion alert configuration
type DeletionAlertConfig struct {
	// Enable high-alert logging for file deletions
	Enabled bool `json:"enabled"`

	// Mass deletion threshold (number of files)
	MassThreshold int `json:"mass_threshold"`

	// Time window for mass deletion detection (in minutes)
	TimeWindow int `json:"time_window"`

	// Alert on deletions of valuable file types
	AlertOnValueableFiles bool `json:"alert_on_valuable_files"`

	// Alert on deletions without backups
	AlertOnNoBackup bool `json:"alert_on_no_backup"`

	// Alert on large file deletions (size in MB)
	LargeFileThreshold int `json:"large_file_threshold"`
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

	// Remote backup settings
	RemoteBackup RemoteBackupConfig `json:"remote_backup"`
}

// RemoteBackupConfig holds remote backup configuration
type RemoteBackupConfig struct {
	// Enable remote backup using Rsync over SSH
	Enabled bool `json:"enabled"`

	// Remote server hostname or IP address
	Host string `json:"host"`

	// SSH username for remote server
	Username string `json:"username"`

	// SSH port (default: 22)
	Port int `json:"port"`

	// Remote directory path to store backups
	RemotePath string `json:"remote_path"`

	// SSH private key file path (optional, uses SSH agent if not specified)
	SSHKeyPath string `json:"ssh_key_path"`

	// Rsync options (default: "-avz --delete")
	RsyncOptions []string `json:"rsync_options"`

	// Sync interval in minutes (0 = sync after every backup)
	SyncInterval int `json:"sync_interval"`

	// Connection timeout in seconds
	ConnectionTimeout int `json:"connection_timeout"`

	// Maximum retry attempts for failed syncs
	MaxRetries int `json:"max_retries"`

	// Retry delay in seconds
	RetryDelay int `json:"retry_delay"`

	// Enable compression during transfer
	EnableCompression bool `json:"enable_compression"`

	// Bandwidth limit in KB/s (0 = no limit)
	BandwidthLimit int `json:"bandwidth_limit"`

	// Remote rollback settings
	RemoteRollback RemoteRollbackConfig `json:"remote_rollback"`
}

// RemoteRollbackConfig holds remote rollback configuration
type RemoteRollbackConfig struct {
	// Enable remote rollback from backup server
	Enabled bool `json:"enabled"`

	// Prefer remote rollback over local when both are available
	PreferRemote bool `json:"prefer_remote"`

	// Create local backup before remote rollback
	BackupBeforeRollback bool `json:"backup_before_rollback"`

	// Verify file integrity after remote rollback
	VerifyIntegrity bool `json:"verify_integrity"`

	// Rollback options for rsync
	RollbackOptions []string `json:"rollback_options"`

	// Temp directory for staging remote files during rollback
	TempDir string `json:"temp_dir"`
}

// SystemConfig holds system-related configuration
type SystemConfig struct {
	// Number of worker goroutines for file processing
	WorkerCount int `json:"worker_count"`

	// Buffer size for file event channels
	BufferSize int `json:"buffer_size"`

	// Timeout for operations (in seconds)
	OperationTimeout int `json:"operation_timeout"`

	// Periodic scanning settings
	PeriodicScan PeriodicScanConfig `json:"periodic_scan"`
}

// PeriodicScanConfig holds periodic file scanning configuration
type PeriodicScanConfig struct {
	// Enable periodic file change scanning
	Enabled bool `json:"enabled"`

	// Random interval scanning (chooses randomly between min and max)
	RandomIntervals []int `json:"random_intervals"` // Intervals in seconds [60, 300, 600]

	// Enable deep content analysis during periodic scans
	DeepAnalysis bool `json:"deep_analysis"`

	// Maximum files to scan per batch
	MaxFilesPerBatch int `json:"max_files_per_batch"`

	// Scan timeout per file in milliseconds
	ScanTimeoutMs int `json:"scan_timeout_ms"`
}

// LoggingConfig holds logging-related configuration
type LoggingConfig struct {
	// Enable long-term mode for 24/7 operation (minimal logging)
	LongTermMode bool `json:"long_term_mode"`

	// Summary interval for periodic logging (in minutes)
	SummaryInterval int `json:"summary_interval"`

	// Maximum log file size in MB before rotation
	MaxLogSize int `json:"max_log_size"`

	// Maximum number of rotated log files to keep
	MaxLogFiles int `json:"max_log_files"`
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		MonitorPath: "./demo-folder",
		BackupPath:  "./.backup",
		LogPath:     "./logs",
		Detection: DetectionConfig{
			SuspiciousExtensions: []string{".locked", ".encrypted", ".crypto", ".crypt", ".enc"},
			Threshold:            3,
			TimeWindow:           60,
			RealTimeDetection:    true,
			ContentPatterns:      []string{"RANSOMWARE", "DECRYPT", "BITCOIN", "PAYMENT"},
			DeletionAlert: DeletionAlertConfig{
				Enabled:               true,
				MassThreshold:         5,
				TimeWindow:            2,
				AlertOnValueableFiles: true,
				AlertOnNoBackup:       true,
				LargeFileThreshold:    1,
			},
		},
		Backup: BackupConfig{
			AutoBackup:      true,
			RetentionDays:   30,
			EnableChecksums: true,
			Compression:     false,
			RemoteBackup: RemoteBackupConfig{
				Enabled:           false, // Disabled by default, user needs to configure
				Host:              "backup-server.example.com",
				Username:          "backup-user",
				Port:              22,
				RemotePath:        "/home/backup-user/ransomware-backups",
				SSHKeyPath:        "", // Uses SSH agent by default
				RsyncOptions:      []string{"-avz", "--delete", "--partial", "--progress"},
				SyncInterval:      5,  // Sync every 5 minutes
				ConnectionTimeout: 30, // 30 seconds timeout
				MaxRetries:        3,  // Retry 3 times on failure
				RetryDelay:        10, // Wait 10 seconds between retries
				EnableCompression: true,
				BandwidthLimit:    0, // No bandwidth limit
				RemoteRollback: RemoteRollbackConfig{
					Enabled:              false, // Disabled by default
					PreferRemote:         true,  // Prefer remote when available
					BackupBeforeRollback: true,  // Create local backup before rollback
					VerifyIntegrity:      true,  // Verify files after rollback
					RollbackOptions:      []string{"-avz", "--delete", "--partial", "--progress"},
					TempDir:              "./.temp-rollback", // Temp staging directory
				},
			},
		},
		System: SystemConfig{
			WorkerCount:      4,
			BufferSize:       100,
			OperationTimeout: 30,
			PeriodicScan: PeriodicScanConfig{
				Enabled:          true,
				RandomIntervals:  []int{60, 300, 600}, // 1min, 5min, 10min
				DeepAnalysis:     true,
				MaxFilesPerBatch: 100,
				ScanTimeoutMs:    5000, // 5 seconds per file
			},
		},
		Logging: LoggingConfig{
			LongTermMode:    false,
			SummaryInterval: 5,
			MaxLogSize:      50,
			MaxLogFiles:     5,
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
