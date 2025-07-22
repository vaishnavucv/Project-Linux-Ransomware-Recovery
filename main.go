package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"ransomware-recovery/pkg/config"
	"ransomware-recovery/pkg/logger"
)

// Command-line flags
var (
	rollbackFlag = flag.Bool("rollback", false, "Perform manual rollback of all backed up files")
	configFlag   = flag.String("config", "./config.json", "Path to configuration file")
	helpFlag     = flag.Bool("help", false, "Show help information")
	versionFlag  = flag.Bool("version", false, "Show version information")
)

const (
	Version = "1.0.0"
	AppName = "Linux Ransomware Recovery System"
)

// RansomwareRecoverySystem orchestrates all components
type RansomwareRecoverySystem struct {
	config          *config.Config
	agent           *Agent
	detector        *Detector
	rollbackManager *RollbackManager
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
}

// NewRansomwareRecoverySystem creates a new system instance
func NewRansomwareRecoverySystem(configPath string) (*RansomwareRecoverySystem, error) {
	// Load configuration
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	// Initialize logger with configuration
	logConfig := logger.LogConfig{
		LongTermMode:    cfg.Logging.LongTermMode,
		SummaryInterval: time.Duration(cfg.Logging.SummaryInterval) * time.Minute,
		MaxLogSize:      int64(cfg.Logging.MaxLogSize) * 1024 * 1024, // Convert MB to bytes
		MaxLogFiles:     cfg.Logging.MaxLogFiles,
	}
	if err := logger.InitLoggerWithConfig(cfg.LogPath, logConfig); err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	// Create context
	ctx, cancel := context.WithCancel(context.Background())

	// Create components
	agent, err := NewAgent(cfg)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	detector := NewDetector(cfg)
	rollbackManager := NewRollbackManager(cfg)

	system := &RansomwareRecoverySystem{
		config:          cfg,
		agent:           agent,
		detector:        detector,
		rollbackManager: rollbackManager,
		ctx:             ctx,
		cancel:          cancel,
	}

	return system, nil
}

// Start initializes and starts all system components
func (rrs *RansomwareRecoverySystem) Start() error {
	logger.Log.Info("Starting Ransomware Recovery System")

	// Start detector
	if err := rrs.detector.Start(); err != nil {
		return fmt.Errorf("failed to start detector: %w", err)
	}

	// Start agent
	if err := rrs.agent.Start(); err != nil {
		return fmt.Errorf("failed to start agent: %w", err)
	}

	// Start event bridging (connect agent events to detector)
	rrs.wg.Add(1)
	go rrs.bridgeEvents()

	// Start detection monitoring
	rrs.wg.Add(1)
	go rrs.monitorDetections()

	// Start periodic status reporting
	rrs.wg.Add(1)
	go rrs.statusReporter()

	logger.Log.Info("Ransomware Recovery System started successfully")
	return nil
}

// Stop gracefully shuts down all system components
func (rrs *RansomwareRecoverySystem) Stop() error {
	logger.Log.Info("Stopping Ransomware Recovery System")

	// Cancel context to stop all goroutines
	rrs.cancel()

	// Stop components
	if err := rrs.agent.Stop(); err != nil {
		logger.LogError("main", "stop_agent", err)
	}

	if err := rrs.detector.Stop(); err != nil {
		logger.LogError("main", "stop_detector", err)
	}

	// Wait for all goroutines to finish
	rrs.wg.Wait()

	logger.Log.Info("Ransomware Recovery System stopped")
	return nil
}

// bridgeEvents connects agent file events to detector
func (rrs *RansomwareRecoverySystem) bridgeEvents() {
	defer rrs.wg.Done()

	agentEvents := rrs.agent.GetEventChannel()
	detectorEvents := rrs.detector.GetEventChannel()

	for {
		select {
		case <-rrs.ctx.Done():
			return

		case event, ok := <-agentEvents:
			if !ok {
				return
			}

			// Forward event to detector
			select {
			case detectorEvents <- event:
			case <-rrs.ctx.Done():
				return
			default:
				logger.Log.Warn("Detector event channel full, dropping event")
			}
		}
	}
}

// monitorDetections monitors detection results and triggers rollback if needed
func (rrs *RansomwareRecoverySystem) monitorDetections() {
	defer rrs.wg.Done()

	detectionChan := rrs.detector.GetDetectionChannel()

	for {
		select {
		case <-rrs.ctx.Done():
			return

		case detection, ok := <-detectionChan:
			if !ok {
				return
			}

			rrs.handleDetectionResult(detection)
		}
	}
}

// handleDetectionResult processes detection results and triggers rollback if necessary
func (rrs *RansomwareRecoverySystem) handleDetectionResult(detection DetectionResult) {
	logger.Log.WithFields(map[string]interface{}{
		"event_count":      detection.EventCount,
		"threshold":        detection.Threshold,
		"trigger_rollback": detection.TriggerRollback,
		"time_window":      detection.TimeWindow,
	}).Info("Detection result received")

	if detection.TriggerRollback {
		logger.Log.Warn("Suspicious activity threshold exceeded - triggering emergency rollback")

		// Trigger full rollback directly without quarantine
		rollbackCtx, cancel := context.WithTimeout(rrs.ctx, 5*time.Minute)
		defer cancel()

		result, err := rrs.rollbackManager.TriggerRollback(rollbackCtx)
		if err != nil {
			logger.LogError("main", "trigger_rollback", err)
		} else {
			logger.Log.WithFields(map[string]interface{}{
				"total_files":    result.TotalFiles,
				"restored_files": result.RestoredFiles,
				"failed_files":   result.FailedFiles,
				"duration":       result.Duration,
				"success":        result.Success,
			}).Info("Emergency rollback completed")
		}
	}
}

// statusReporter periodically reports system status
func (rrs *RansomwareRecoverySystem) statusReporter() {
	defer rrs.wg.Done()

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-rrs.ctx.Done():
			return

		case <-ticker.C:
			rrs.reportStatus()
		}
	}
}

// reportStatus logs current system status
func (rrs *RansomwareRecoverySystem) reportStatus() {
	// Get agent stats
	agentStats := rrs.agent.GetStats()

	// Get detector stats
	detectorStats := rrs.detector.GetStats()

	// Get rollback stats
	rollbackStats, err := rrs.rollbackManager.GetRollbackStats()
	if err != nil {
		logger.LogError("main", "get_rollback_stats", err)
		rollbackStats = map[string]interface{}{"error": err.Error()}
	}

	// Get suspicious events
	suspiciousEvents := rrs.detector.GetSuspiciousEvents()

	logger.Log.WithFields(map[string]interface{}{
		"agent":                agentStats,
		"detector":             detectorStats,
		"rollback":             rollbackStats,
		"suspicious_events":    len(suspiciousEvents),
		"rollback_in_progress": rrs.rollbackManager.IsRollbackInProgress(),
	}).Info("System status report")
}

// GetSystemStats returns comprehensive system statistics
func (rrs *RansomwareRecoverySystem) GetSystemStats() map[string]interface{} {
	agentStats := rrs.agent.GetStats()
	detectorStats := rrs.detector.GetStats()
	rollbackStats, _ := rrs.rollbackManager.GetRollbackStats()

	return map[string]interface{}{
		"agent":    agentStats,
		"detector": detectorStats,
		"rollback": rollbackStats,
		"config": map[string]interface{}{
			"monitor_path":        rrs.config.MonitorPath,
			"backup_path":         rrs.config.BackupPath,
			"log_path":            rrs.config.LogPath,
			"auto_backup":         rrs.config.Backup.AutoBackup,
			"real_time_detection": rrs.config.Detection.RealTimeDetection,
			"detection_threshold": rrs.config.Detection.Threshold,
		},
	}
}

// ManualRollback allows manual triggering of rollback
func (rrs *RansomwareRecoverySystem) ManualRollback() (*RollbackResult, error) {
	logger.Log.Info("Manual rollback requested")

	ctx, cancel := context.WithTimeout(rrs.ctx, 10*time.Minute)
	defer cancel()

	return rrs.rollbackManager.TriggerRollback(ctx)
}

// ForceDetection manually triggers detection analysis
func (rrs *RansomwareRecoverySystem) ForceDetection() DetectionResult {
	logger.Log.Info("Manual detection analysis requested")
	return rrs.detector.ForceDetection()
}

// EnableLongTermLogging switches to optimized logging for 24/7 operation
func (rrs *RansomwareRecoverySystem) EnableLongTermLogging() {
	logger.EnableLongTermMode()
	logger.Log.Warn("System switched to long-term logging mode for 24/7 operation")
}

// DisableLongTermLogging switches back to standard logging
func (rrs *RansomwareRecoverySystem) DisableLongTermLogging() {
	logger.DisableLongTermMode()
	logger.Log.Info("System switched back to standard logging mode")
}

// showHelp displays help information
func showHelp() {
	fmt.Printf("%s v%s\n\n", AppName, Version)
	fmt.Println("USAGE:")
	fmt.Printf("  %s [OPTIONS]\n\n", os.Args[0])
	fmt.Println("OPTIONS:")
	fmt.Println("  --rollback           Perform manual rollback of all backed up files")
	fmt.Println("  --config PATH        Path to configuration file (default: ./config.json)")
	fmt.Println("  --help               Show this help information")
	fmt.Println("  --version            Show version information")
	fmt.Println()
	fmt.Println("EXAMPLES:")
	fmt.Printf("  %s                   # Start monitoring system\n", os.Args[0])
	fmt.Printf("  %s --rollback        # Perform manual rollback\n", os.Args[0])
	fmt.Printf("  %s --config /path/to/config.json  # Use custom config\n", os.Args[0])
	fmt.Println()
	fmt.Println("DESCRIPTION:")
	fmt.Println("  Linux Ransomware Recovery System provides real-time file monitoring,")
	fmt.Println("  automatic backup, suspicious pattern detection, and automated rollback")
	fmt.Println("  capabilities to protect against ransomware attacks.")
	fmt.Println()
	fmt.Println("  The system monitors the demo-folder directory for file changes and")
	fmt.Println("  automatically creates backups. When suspicious activity is detected,")
	fmt.Println("  it can automatically restore files from the most recent backups.")
	fmt.Println()
	fmt.Println("  Use the --rollback flag to manually restore all files from backups")
	fmt.Println("  when you suspect a ransomware attack has occurred.")
}

// showVersion displays version information
func showVersion() {
	fmt.Printf("%s v%s\n", AppName, Version)
	fmt.Println("Built with Go 1.21+")
	fmt.Println("Copyright (c) 2025 - Linux Ransomware Recovery Project")
}

// performManualRollback executes a manual rollback operation
func performManualRollback(configPath string) error {
	fmt.Printf("=== %s - Manual Rollback ===\n", AppName)
	fmt.Println()

	// Create system instance
	system, err := NewRansomwareRecoverySystem(configPath)
	if err != nil {
		return fmt.Errorf("failed to create system: %w", err)
	}

	// Log rollback initiation
	logger.Log.WithFields(map[string]interface{}{
		"operation":    "manual_rollback",
		"initiated_by": "command_line",
		"config_path":  configPath,
		"timestamp":    time.Now().Format("2006-01-02 15:04:05"),
	}).Warn("Manual rollback initiated via command line")

	fmt.Println("🔄 Initiating manual rollback operation...")
	fmt.Printf("📁 Monitor Path: %s\n", system.config.MonitorPath)
	fmt.Printf("💾 Backup Path: %s\n", system.config.BackupPath)
	fmt.Println()

	// Perform rollback
	startTime := time.Now()
	result, err := system.ManualRollback()
	if err != nil {
		fmt.Printf("❌ Rollback failed: %v\n", err)
		logger.Log.WithFields(map[string]interface{}{
			"operation": "manual_rollback",
			"success":   false,
			"error":     err.Error(),
			"duration":  time.Since(startTime).String(),
		}).Error("Manual rollback failed")
		return err
	}

	// Display results
	fmt.Println("📊 Rollback Results:")
	fmt.Printf("   Total Files: %d\n", result.TotalFiles)
	fmt.Printf("   Restored Files: %d\n", result.RestoredFiles)
	fmt.Printf("   Failed Files: %d\n", result.FailedFiles)
	fmt.Printf("   Duration: %s\n", result.Duration.String())
	fmt.Printf("   Success: %t\n", result.Success)
	fmt.Println()

	if len(result.RestoredPaths) > 0 {
		fmt.Println("✅ Successfully Restored Files:")
		for _, path := range result.RestoredPaths {
			fmt.Printf("   - %s\n", path)
		}
		fmt.Println()
	}

	if len(result.FailedPaths) > 0 {
		fmt.Println("❌ Failed to Restore Files:")
		for i, path := range result.FailedPaths {
			fmt.Printf("   - %s", path)
			if i < len(result.Errors) {
				fmt.Printf(" (Error: %s)", result.Errors[i])
			}
			fmt.Println()
		}
		fmt.Println()
	}

	// Log completion
	logger.Log.WithFields(map[string]interface{}{
		"operation":      "manual_rollback",
		"success":        result.Success,
		"total_files":    result.TotalFiles,
		"restored_files": result.RestoredFiles,
		"failed_files":   result.FailedFiles,
		"duration":       result.Duration.String(),
		"duration_ms":    result.Duration.Milliseconds(),
		"initiated_by":   "command_line",
		"timestamp":      time.Now().Format("2006-01-02 15:04:05"),
	}).Info("Manual rollback completed")

	if result.Success {
		fmt.Println("🎉 Manual rollback completed successfully!")
		fmt.Printf("📝 Check logs at: %s/journal.log\n", system.config.LogPath)
	} else {
		fmt.Println("⚠️  Manual rollback completed with errors.")
		fmt.Printf("📝 Check logs at: %s/journal.log for details\n", system.config.LogPath)
	}

	return nil
}

func main() {
	// Parse command-line flags
	flag.Parse()

	// Handle help flag
	if *helpFlag {
		showHelp()
		return
	}

	// Handle version flag
	if *versionFlag {
		showVersion()
		return
	}

	// Handle rollback flag
	if *rollbackFlag {
		if err := performManualRollback(*configFlag); err != nil {
			fmt.Printf("Manual rollback failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Default behavior: start monitoring system
	fmt.Printf("Starting %s v%s\n", AppName, Version)

	// Create system
	system, err := NewRansomwareRecoverySystem(*configFlag)
	if err != nil {
		fmt.Printf("Failed to create system: %v\n", err)
		os.Exit(1)
	}

	// Start system
	if err := system.Start(); err != nil {
		fmt.Printf("Failed to start system: %v\n", err)
		os.Exit(1)
	}

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for shutdown signal
	logger.Log.Info("Ransomware Recovery System is running. Press Ctrl+C to stop.")
	<-sigChan

	// Graceful shutdown
	logger.Log.Info("Shutdown signal received, stopping system...")
	if err := system.Stop(); err != nil {
		logger.LogError("main", "stop_system", err)
		os.Exit(1)
	}

	logger.Log.Info("System shutdown completed")
}
