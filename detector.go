package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"ransomware-recovery/pkg/config"
	"ransomware-recovery/pkg/logger"
)

// Detector handles suspicious pattern detection
type Detector struct {
	config           *config.Config
	suspiciousEvents []SuspiciousEvent
	eventChan        chan FileEvent
	detectionChan    chan DetectionResult
	ctx              context.Context
	cancel           context.CancelFunc
	mu               sync.RWMutex
	wg               sync.WaitGroup
}

// SuspiciousEvent represents a suspicious file system event
type SuspiciousEvent struct {
	FilePath    string    `json:"file_path"`
	EventType   string    `json:"event_type"`
	Pattern     string    `json:"pattern"`
	Severity    string    `json:"severity"`
	Timestamp   time.Time `json:"timestamp"`
	Description string    `json:"description"`
}

// DetectionResult represents the result of pattern detection
type DetectionResult struct {
	TriggerRollback bool              `json:"trigger_rollback"`
	EventCount      int               `json:"event_count"`
	Threshold       int               `json:"threshold"`
	TimeWindow      time.Duration     `json:"time_window"`
	Events          []SuspiciousEvent `json:"events"`
	Timestamp       time.Time         `json:"timestamp"`
}

// NewDetector creates a new suspicious pattern detector
func NewDetector(cfg *config.Config) *Detector {
	ctx, cancel := context.WithCancel(context.Background())

	return &Detector{
		config:           cfg,
		suspiciousEvents: make([]SuspiciousEvent, 0),
		eventChan:        make(chan FileEvent, cfg.System.BufferSize),
		detectionChan:    make(chan DetectionResult, 10),
		ctx:              ctx,
		cancel:           cancel,
	}
}

// Start begins the detection process
func (d *Detector) Start() error {
	logger.Log.Info("Starting suspicious pattern detector")

	// Start real-time detection if enabled
	if d.config.Detection.RealTimeDetection {
		d.wg.Add(1)
		go d.realTimeDetection()
	}

	// Start periodic log scanning
	d.wg.Add(1)
	go d.periodicLogScan()

	// Start event cleanup routine
	d.wg.Add(1)
	go d.eventCleanup()

	logger.Log.Info("Suspicious pattern detector started successfully")
	return nil
}

// Stop stops the detector
func (d *Detector) Stop() error {
	logger.Log.Info("Stopping suspicious pattern detector")

	// Cancel context to stop all goroutines
	d.cancel()

	// Close channels
	close(d.eventChan)
	close(d.detectionChan)

	// Wait for all goroutines to finish
	d.wg.Wait()

	logger.Log.Info("Suspicious pattern detector stopped")
	return nil
}

// GetEventChannel returns the event channel for receiving file events
func (d *Detector) GetEventChannel() chan<- FileEvent {
	return d.eventChan
}

// GetDetectionChannel returns the detection result channel
func (d *Detector) GetDetectionChannel() <-chan DetectionResult {
	return d.detectionChan
}

// realTimeDetection processes file events in real-time
func (d *Detector) realTimeDetection() {
	defer d.wg.Done()

	logger.Log.Info("Starting real-time detection")

	for {
		select {
		case <-d.ctx.Done():
			return

		case event, ok := <-d.eventChan:
			if !ok {
				return
			}

			d.analyzeEvent(event)
		}
	}
}

// periodicLogScan scans the journal.log file periodically
func (d *Detector) periodicLogScan() {
	defer d.wg.Done()

	// Scan every 30 seconds
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Initial scan
	d.scanJournalLog()

	for {
		select {
		case <-d.ctx.Done():
			return

		case <-ticker.C:
			d.scanJournalLog()
		}
	}
}

// scanJournalLog scans the journal.log file for suspicious patterns
func (d *Detector) scanJournalLog() {
	journalPath := filepath.Join(d.config.LogPath, "journal.log")

	file, err := os.Open(journalPath)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.LogError("detector", "open_journal", err)
		}
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0

	for scanner.Scan() {
		lineCount++
		line := scanner.Text()

		// Parse JSON log entry
		var logEntry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &logEntry); err != nil {
			continue // Skip non-JSON lines
		}

		// Analyze log entry for suspicious patterns
		d.analyzeLogEntry(logEntry)
	}

	if err := scanner.Err(); err != nil {
		logger.LogError("detector", "scan_journal", err)
	}
}

// analyzeEvent analyzes a file event for suspicious patterns
func (d *Detector) analyzeEvent(event FileEvent) {
	suspicious := d.isSuspiciousEvent(event)
	if len(suspicious) == 0 {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	// Add suspicious events
	for _, suspEvent := range suspicious {
		d.suspiciousEvents = append(d.suspiciousEvents, suspEvent)
		logger.LogDetection(suspEvent.Pattern, suspEvent.FilePath, suspEvent.Severity)
	}

	// Check if we should trigger rollback
	d.checkRollbackTrigger()
}

// analyzeLogEntry analyzes a log entry for suspicious patterns
func (d *Detector) analyzeLogEntry(logEntry map[string]interface{}) {
	// Extract relevant fields
	filePath, _ := logEntry["file_path"].(string)
	eventType, _ := logEntry["event_type"].(string)
	component, _ := logEntry["component"].(string)

	if filePath == "" || component != "agent" {
		return
	}

	// Create a synthetic FileEvent for analysis
	event := FileEvent{
		Path:      filePath,
		Operation: eventType,
		Timestamp: time.Now(),
		Size:      0,
	}

	// Analyze the event
	d.analyzeEvent(event)
}

// isSuspiciousEvent checks if a file event is suspicious
func (d *Detector) isSuspiciousEvent(event FileEvent) []SuspiciousEvent {
	var suspicious []SuspiciousEvent

	// Check file extension
	ext := filepath.Ext(event.Path)
	if d.config.IsSuspiciousExtension(ext) {
		suspicious = append(suspicious, SuspiciousEvent{
			FilePath:    event.Path,
			EventType:   event.Operation,
			Pattern:     fmt.Sprintf("suspicious_extension:%s", ext),
			Severity:    "high",
			Timestamp:   event.Timestamp,
			Description: fmt.Sprintf("File with suspicious extension %s detected", ext),
		})
	}

	// Check for rapid file creation/modification patterns
	if event.Operation == "CREATE" || event.Operation == "WRITE" {
		if d.isRapidFileActivity(event.Path) {
			suspicious = append(suspicious, SuspiciousEvent{
				FilePath:    event.Path,
				EventType:   event.Operation,
				Pattern:     "rapid_file_activity",
				Severity:    "medium",
				Timestamp:   event.Timestamp,
				Description: "Rapid file activity detected",
			})
		}
	}

	// Check file content if it's a text file
	if event.Operation == "CREATE" || event.Operation == "WRITE" {
		if d.hasSuspiciousContent(event.Path) {
			suspicious = append(suspicious, SuspiciousEvent{
				FilePath:    event.Path,
				EventType:   event.Operation,
				Pattern:     "suspicious_content",
				Severity:    "high",
				Timestamp:   event.Timestamp,
				Description: "Suspicious content patterns detected in file",
			})
		}
	}

	// Check for mass file operations
	if d.isMassFileOperation(event.Path) {
		suspicious = append(suspicious, SuspiciousEvent{
			FilePath:    event.Path,
			EventType:   event.Operation,
			Pattern:     "mass_file_operation",
			Severity:    "high",
			Timestamp:   event.Timestamp,
			Description: "Mass file operation detected",
		})
	}

	return suspicious
}

// isRapidFileActivity checks for rapid file activity patterns
func (d *Detector) isRapidFileActivity(filePath string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// Count recent events for the same file
	now := time.Now()
	count := 0
	
	for _, event := range d.suspiciousEvents {
		if event.FilePath == filePath && now.Sub(event.Timestamp) < 10*time.Second {
			count++
		}
	}

	return count > 5 // More than 5 events in 10 seconds
}

// hasSuspiciousContent checks if a file contains suspicious content
func (d *Detector) hasSuspiciousContent(filePath string) bool {
	// Only check small text files to avoid performance issues
	info, err := os.Stat(filePath)
	if err != nil || info.Size() > 10*1024 { // Skip files larger than 10KB
		return false
	}

	file, err := os.Open(filePath)
	if err != nil {
		return false
	}
	defer file.Close()

	// Read first 1KB of content
	buffer := make([]byte, 1024)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return false
	}

	content := string(buffer[:n])
	return d.config.HasSuspiciousContent(content)
}

// isMassFileOperation checks for mass file operations
func (d *Detector) isMassFileOperation(filePath string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// Count recent events in the same directory
	now := time.Now()
	dir := filepath.Dir(filePath)
	count := 0

	for _, event := range d.suspiciousEvents {
		eventDir := filepath.Dir(event.FilePath)
		if eventDir == dir && now.Sub(event.Timestamp) < 30*time.Second {
			count++
		}
	}

	return count > 10 // More than 10 events in same directory within 30 seconds
}

// checkRollbackTrigger checks if rollback should be triggered
func (d *Detector) checkRollbackTrigger() {
	now := time.Now()
	timeWindow := d.config.GetTimeWindow()
	
	// Count recent high-severity events
	recentEvents := make([]SuspiciousEvent, 0)
	
	for _, event := range d.suspiciousEvents {
		if now.Sub(event.Timestamp) <= timeWindow {
			recentEvents = append(recentEvents, event)
		}
	}

	// Check if threshold is exceeded
	shouldTrigger := len(recentEvents) >= d.config.Detection.Threshold

	// Create detection result
	result := DetectionResult{
		TriggerRollback: shouldTrigger,
		EventCount:      len(recentEvents),
		Threshold:       d.config.Detection.Threshold,
		TimeWindow:      timeWindow,
		Events:          recentEvents,
		Timestamp:       now,
	}

	// Send result to detection channel
	select {
	case d.detectionChan <- result:
	case <-d.ctx.Done():
		return
	default:
		logger.Log.Warn("Detection channel full, dropping result")
	}

	if shouldTrigger {
		logger.Log.WithFields(map[string]interface{}{
			"event_count": len(recentEvents),
			"threshold":   d.config.Detection.Threshold,
			"time_window": timeWindow,
		}).Warn("Rollback trigger threshold exceeded")
	}
}

// eventCleanup periodically cleans up old events
func (d *Detector) eventCleanup() {
	defer d.wg.Done()

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return

		case <-ticker.C:
			d.cleanupOldEvents()
		}
	}
}

// cleanupOldEvents removes events older than the time window
func (d *Detector) cleanupOldEvents() {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	timeWindow := d.config.GetTimeWindow()
	
	// Keep only recent events
	filtered := make([]SuspiciousEvent, 0)
	
	for _, event := range d.suspiciousEvents {
		if now.Sub(event.Timestamp) <= timeWindow*2 { // Keep events for 2x time window
			filtered = append(filtered, event)
		}
	}

	removedCount := len(d.suspiciousEvents) - len(filtered)
	d.suspiciousEvents = filtered

	if removedCount > 0 {
		logger.Log.WithField("removed_count", removedCount).Debug("Cleaned up old suspicious events")
	}
}

// GetSuspiciousEvents returns current suspicious events
func (d *Detector) GetSuspiciousEvents() []SuspiciousEvent {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// Return a copy to avoid race conditions
	events := make([]SuspiciousEvent, len(d.suspiciousEvents))
	copy(events, d.suspiciousEvents)
	
	return events
}

// GetStats returns detector statistics
func (d *Detector) GetStats() map[string]interface{} {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return map[string]interface{}{
		"total_suspicious_events": len(d.suspiciousEvents),
		"detection_threshold":     d.config.Detection.Threshold,
		"time_window_seconds":     d.config.Detection.TimeWindow,
		"real_time_detection":     d.config.Detection.RealTimeDetection,
		"suspicious_extensions":   d.config.Detection.SuspiciousExtensions,
		"content_patterns":        d.config.Detection.ContentPatterns,
	}
}

// ForceDetection manually triggers detection analysis
func (d *Detector) ForceDetection() DetectionResult {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	timeWindow := d.config.GetTimeWindow()
	
	// Count recent events
	recentEvents := make([]SuspiciousEvent, 0)
	
	for _, event := range d.suspiciousEvents {
		if now.Sub(event.Timestamp) <= timeWindow {
			recentEvents = append(recentEvents, event)
		}
	}

	// Create detection result
	result := DetectionResult{
		TriggerRollback: len(recentEvents) >= d.config.Detection.Threshold,
		EventCount:      len(recentEvents),
		Threshold:       d.config.Detection.Threshold,
		TimeWindow:      timeWindow,
		Events:          recentEvents,
		Timestamp:       now,
	}

	logger.Log.WithFields(map[string]interface{}{
		"event_count":      len(recentEvents),
		"threshold":        d.config.Detection.Threshold,
		"trigger_rollback": result.TriggerRollback,
	}).Info("Manual detection analysis completed")

	return result
} 