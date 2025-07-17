package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"ransomware-recovery/pkg/config"
	"ransomware-recovery/pkg/logger"
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

	// Initialize logger
	if err := logger.InitLogger(cfg.LogPath); err != nil {
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

		// Extract infected file paths from detection events
		infectedFiles := make([]string, 0)
		for _, event := range detection.Events {
			infectedFiles = append(infectedFiles, event.FilePath)
		}

		// Quarantine infected files first
		if len(infectedFiles) > 0 {
			quarantineCtx, cancel := context.WithTimeout(rrs.ctx, 30*time.Second)
			defer cancel()

			if err := rrs.rollbackManager.QuarantineInfectedFiles(quarantineCtx, infectedFiles); err != nil {
				logger.LogError("main", "quarantine_files", err)
			}
		}

		// Trigger full rollback
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
		"agent":             agentStats,
		"detector":          detectorStats,
		"rollback":          rollbackStats,
		"suspicious_events": len(suspiciousEvents),
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

func main() {
	// Configuration file path
	configPath := "./config.json"

	// Create system
	system, err := NewRansomwareRecoverySystem(configPath)
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