# Periodic File Scanning System

This document explains the comprehensive periodic file scanning system that performs random interval file change detection and analysis to catch changes that might be missed by real-time monitoring.

## Overview

The periodic file scanning system complements the real-time file monitoring by performing scheduled scans at random intervals (60 seconds, 5 minutes, 10 minutes) to detect file changes, analyze content, and take appropriate backup and security actions.

## Features

### 🕐 **Random Interval Scanning**
- **Random Timing**: Chooses randomly between 60s, 5min, and 10min intervals
- **Unpredictable Schedule**: Makes it harder for malware to predict scan times
- **Configurable Intervals**: Fully customizable interval options
- **Intelligent Scheduling**: Logs next scan time for transparency

### 🔍 **Comprehensive Analysis**
- **Hash-Based Change Detection**: Uses SHA256 hashes to detect file modifications
- **Deep Content Analysis**: Optional deep scanning for suspicious content patterns
- **Batch Processing**: Configurable batch sizes to manage system load
- **Timeout Management**: Per-file and per-scan timeout controls

### 📊 **Detailed Logging**
- **Timeline Tracking**: Complete audit trail of all scan activities
- **Performance Metrics**: Scan duration, file counts, and success rates
- **Change Detection**: Logs all detected file changes with reasons
- **Suspicious Activity**: Alerts for suspicious files found during scans

### ⚡ **Performance Optimized**
- **Concurrent Processing**: Parallel file analysis within timeout constraints
- **Resource Management**: Configurable limits to prevent system overload
- **Context Cancellation**: Proper timeout and cancellation handling
- **Efficient Scanning**: Skips directories and backup paths automatically

## Configuration

### Periodic Scan Settings

The periodic scanning system is configured in the `system.periodic_scan` section:

```json
{
  "system": {
    "periodic_scan": {
      "enabled": true,
      "random_intervals": [60, 300, 600],
      "deep_analysis": true,
      "max_files_per_batch": 100,
      "scan_timeout_ms": 5000
    }
  }
}
```

### Configuration Options

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `enabled` | boolean | `true` | Enable/disable periodic file scanning |
| `random_intervals` | array | `[60, 300, 600]` | Random intervals in seconds (1min, 5min, 10min) |
| `deep_analysis` | boolean | `true` | Enable deep content analysis during scans |
| `max_files_per_batch` | integer | `100` | Maximum files to scan per batch |
| `scan_timeout_ms` | integer | `5000` | Timeout per file in milliseconds |

## System Architecture

### Scanning Flow

```
1. 🎲 Random Interval Selection
   ├── Choose from configured intervals [60s, 5min, 10min]
   ├── Log next scan schedule
   └── Wait for selected interval

2. 📁 File Discovery
   ├── Walk through monitor directory
   ├── Skip directories and backup paths
   ├── Apply batch size limits
   └── Create file processing list

3. 🔍 File Analysis
   ├── Calculate current file hash
   ├── Compare with stored hash
   ├── Detect content changes
   └── Perform deep analysis (if enabled)

4. 🛡️ Security Checks
   ├── Check suspicious file extensions
   ├── Analyze file content patterns
   ├── Log suspicious activities
   └── Trigger alerts if needed

5. 💾 Backup Actions
   ├── Backup changed files immediately
   ├── Update backup database
   ├── Log backup operations
   └── Verify backup integrity

6. 📊 Results Reporting
   ├── Generate scan statistics
   ├── Log performance metrics
   ├── Report suspicious findings
   └── Schedule next scan
```

## Scanning Process

### Random Interval Selection

The system randomly selects from configured intervals:

```go
intervals := []int{60, 300, 600} // 1min, 5min, 10min
randomInterval := intervals[rand.Intn(len(intervals))]
nextScanTime := time.Duration(randomInterval) * time.Second
```

### File Change Detection

Each file is analyzed for changes using hash comparison:

```go
// Calculate current file hash
currentHash := calculateSHA256(filePath)

// Compare with stored hash
if storedHash != currentHash {
    // File has changed - backup immediately
    backupManager.BackupFileImmediate(filePath)
}
```

### Deep Content Analysis

When enabled, performs content analysis on small files:

```go
// Only analyze files under 10KB
if fileSize <= 10*1024 {
    content := readFileContent(filePath, 1024) // Read first 1KB
    if containsSuspiciousPatterns(content) {
        logSuspiciousContent(filePath, content)
    }
}
```

## Timeline Logging Examples

### Scan Scheduling
```json
{
  "level": "info",
  "msg": "Scheduled next periodic file scan",
  "next_scan_in_seconds": 300,
  "next_scan_in": "5m0s",
  "component": "agent",
  "event_type": "periodic_scan_scheduled",
  "time": "2025-07-21 20:35:15"
}
```

### Scan Initiation
```json
{
  "level": "info",
  "msg": "Starting periodic file scan",
  "component": "agent",
  "event_type": "periodic_scan_start",
  "scan_time": "2025-07-21 20:40:15",
  "deep_analysis": true,
  "time": "2025-07-21 20:40:15"
}
```

### File Analysis
```json
{
  "level": "debug",
  "msg": "Periodic file analysis completed",
  "file_path": "demo-folder/document.txt",
  "change_type": "modified",
  "backup_needed": true,
  "file_size": 2048,
  "mod_time": "2025-07-21 20:39:30",
  "component": "agent",
  "event_type": "periodic_file_analysis",
  "scan_type": "periodic",
  "time": "2025-07-21 20:40:16"
}
```

### Backup Success
```json
{
  "level": "info",
  "msg": "File backed up during periodic scan",
  "file_path": "demo-folder/document.txt",
  "change_type": "modified",
  "backup_reason": "periodic_scan_detected_change",
  "component": "agent",
  "event_type": "periodic_backup_success",
  "time": "2025-07-21 20:40:16"
}
```

### Suspicious File Detection
```json
{
  "level": "warning",
  "msg": "Suspicious file detected during periodic scan",
  "file_path": "demo-folder/suspicious.locked",
  "suspicious_ext": ".locked",
  "component": "agent",
  "event_type": "periodic_suspicious_file",
  "detection_method": "extension_check",
  "time": "2025-07-21 20:40:17"
}
```

### Scan Completion
```json
{
  "level": "info",
  "msg": "Periodic file scan completed",
  "component": "agent",
  "event_type": "periodic_scan_complete",
  "scanned_files": 25,
  "changed_files": 3,
  "suspicious_files": 1,
  "errors": 0,
  "duration": "1.234s",
  "duration_ms": 1234,
  "scan_success": true,
  "time": "2025-07-21 20:40:16"
}
```

## Benefits

### 1. **Comprehensive Coverage**
- ✅ **Gap Detection**: Catches changes missed by real-time monitoring
- ✅ **System Reliability**: Provides backup detection mechanism
- ✅ **Consistency Verification**: Ensures all files are properly tracked
- ✅ **Historical Analysis**: Detects gradual changes over time

### 2. **Security Enhancement**
- ✅ **Stealth Detection**: Random intervals make it hard for malware to evade
- ✅ **Content Analysis**: Deep scanning detects sophisticated threats
- ✅ **Pattern Recognition**: Identifies suspicious file modifications
- ✅ **Timeline Reconstruction**: Provides detailed audit trail

### 3. **Performance Optimization**
- ✅ **Batch Processing**: Manages system load effectively
- ✅ **Timeout Controls**: Prevents system lockups
- ✅ **Resource Management**: Configurable limits prevent overload
- ✅ **Efficient Algorithms**: Hash-based change detection is fast

## Use Cases

### 1. **Delayed Ransomware Detection**
**Scenario**: Ransomware that operates slowly to avoid detection
**Detection**: Periodic scans catch gradual file modifications
**Response**: Immediate backup and alert generation

### 2. **System Monitoring Gaps**
**Scenario**: Real-time monitoring misses some file changes
**Detection**: Periodic scans provide comprehensive coverage
**Response**: Backup missing changes and update tracking

### 3. **Forensic Analysis**
**Scenario**: Need to understand file modification timeline
**Detection**: Periodic scan logs provide detailed timeline
**Response**: Reconstruct attack progression and impact

### 4. **Compliance Verification**
**Scenario**: Ensure all file changes are properly backed up
**Detection**: Periodic scans verify backup completeness
**Response**: Backup any missed files and generate reports

## Performance Characteristics

### Typical Performance Metrics
- **Small Files** (<1KB): ~2-5ms per file
- **Medium Files** (1-100KB): ~10-50ms per file
- **Large Files** (>100KB): Hash calculation only (~100-500ms)
- **Scan Frequency**: Every 60s-10min (random)
- **Batch Size**: Up to 100 files per scan (configurable)

### Resource Usage
- **CPU Impact**: Low (hash calculation is efficient)
- **Memory Usage**: Minimal (streaming file processing)
- **Disk I/O**: Moderate (reading files for hash calculation)
- **Network**: None (local file operations only)

## Monitoring and Analysis

### Real-time Monitoring Commands

```bash
# Monitor periodic scan logs
tail -f logs/journal.log | grep "periodic_scan"

# View scan scheduling
grep "periodic_scan_scheduled" logs/journal.log | tail -5

# Check scan performance
grep "periodic_scan_complete" logs/journal.log | jq '.duration_ms'

# Monitor suspicious detections
grep "periodic_suspicious" logs/journal.log
```

### Performance Analysis

```bash
# Analyze scan frequency
grep "periodic_scan_start" logs/journal.log | wc -l

# Calculate average scan duration
grep "periodic_scan_complete" logs/journal.log | \
  jq -r '.duration_ms' | \
  awk '{sum+=$1; count++} END {print "Average:", sum/count, "ms"}'

# Check file change detection rate
grep "periodic_backup_success" logs/journal.log | wc -l
```

### Statistics Queries

```bash
# Check periodic scan configuration
cat config.json | jq '.system.periodic_scan'

# View recent scan results
grep "periodic_scan_complete" logs/journal.log | tail -3 | jq .

# Monitor suspicious activity
grep -E "(periodic_suspicious|CRITICAL ALERT)" logs/journal.log | tail -5
```

## Integration with Existing Systems

### Real-time Monitoring
- **Complementary Operation**: Works alongside fsnotify-based monitoring
- **Gap Coverage**: Catches changes missed by real-time system
- **Unified Logging**: Uses same logging infrastructure
- **Shared Resources**: Uses same backup and detection systems

### Backup System
- **Immediate Backup**: Changed files are backed up instantly
- **Hash Integration**: Uses existing hash-based change detection
- **Database Updates**: Updates backup database with new changes
- **Version Control**: Creates versioned backups with timestamps

### Detection System
- **Pattern Matching**: Uses same suspicious pattern detection
- **Alert Generation**: Triggers same alert mechanisms
- **Timeline Integration**: Contributes to detection timeline
- **Threshold Tracking**: Counts towards detection thresholds

## Troubleshooting

### Common Issues

#### 1. **Scans Taking Too Long**
**Symptoms**: Scan duration exceeds expected time
**Solutions**:
- Reduce `max_files_per_batch` setting
- Increase `scan_timeout_ms` for large files
- Disable `deep_analysis` for performance
- Check system disk performance

#### 2. **High System Load**
**Symptoms**: System becomes slow during scans
**Solutions**:
- Reduce scan frequency (increase intervals)
- Lower `max_files_per_batch` setting
- Disable `deep_analysis` temporarily
- Monitor system resources during scans

#### 3. **Missing File Changes**
**Symptoms**: File changes not detected by periodic scans
**Solutions**:
- Verify file is within monitor path
- Check if file size exceeds analysis limits
- Ensure sufficient scan timeout values
- Review hash calculation errors in logs

#### 4. **Too Many False Positives**
**Symptoms**: Frequent suspicious file alerts
**Solutions**:
- Adjust suspicious extension patterns
- Tune content analysis patterns
- Review deep analysis configuration
- Filter out known safe file types

### Diagnostic Commands

```bash
# Check periodic scan status
grep "Starting periodic file scanner" logs/journal.log | tail -1

# Verify scan scheduling
grep "Scheduled next periodic file scan" logs/journal.log | tail -5

# Monitor scan performance
grep "periodic_scan_complete" logs/journal.log | \
  jq '{files: .scanned_files, duration: .duration, success: .scan_success}'

# Check for scan errors
grep -E "(periodic.*error|periodic.*failed)" logs/journal.log
```

## Security Considerations

### Attack Resistance
- **Random Intervals**: Unpredictable timing makes evasion difficult
- **Deep Analysis**: Content scanning detects sophisticated threats
- **Multiple Detection**: Combined with real-time monitoring for redundancy
- **Immediate Response**: Changed files are backed up instantly

### Privacy and Performance
- **Local Processing**: All analysis performed locally
- **Efficient Algorithms**: Minimal system impact
- **Configurable Limits**: Prevents resource exhaustion
- **Timeout Controls**: Prevents system lockups

The periodic file scanning system provides comprehensive file change detection and analysis, ensuring that no file modifications go unnoticed while maintaining optimal system performance and security. 