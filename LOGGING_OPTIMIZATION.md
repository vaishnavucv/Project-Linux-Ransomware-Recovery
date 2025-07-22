# Logging Optimization for 24/7 Operation

This document explains the logging optimizations implemented to prevent log files from becoming too large during long-term operation (24/7 or extended periods).

## Overview

The ransomware recovery system has been optimized for long-term operation with minimal logging to prevent log files from growing excessively large while maintaining essential monitoring capabilities.

## Key Features

### 1. **Long-Term Mode**
- **Reduced Log Level**: Only warnings and errors are logged (no INFO logs)
- **Selective Logging**: File events are only logged if they involve suspicious extensions
- **Essential Only**: Backup operations only logged if they fail
- **Critical Events**: Detection and rollback events are always logged

### 2. **Automatic Log Rotation**
- **Size-Based Rotation**: Logs are rotated when they exceed 50MB (configurable)
- **File Retention**: Keeps only the last 3 log files by default (configurable)
- **Automatic Cleanup**: Old log files are automatically removed

### 3. **Periodic Summary Logging**
- **Activity Summaries**: System logs periodic summaries instead of individual events
- **Statistics Tracking**: Tracks file events, backup operations, detections, and errors
- **Performance Metrics**: Includes events per minute and system uptime
- **Configurable Interval**: Summary interval is configurable (default: 10 minutes)

## Configuration

Add the following to your `config.json`:

```json
{
  "logging": {
    "long_term_mode": true,
    "summary_interval": 10,
    "max_log_size": 50,
    "max_log_files": 3
  }
}
```

### Configuration Options

- **`long_term_mode`**: Enable optimized logging for 24/7 operation
- **`summary_interval`**: Minutes between periodic summary logs
- **`max_log_size`**: Maximum log file size in MB before rotation
- **`max_log_files`**: Maximum number of rotated log files to keep

## Log Reduction Examples

### Standard Mode (Verbose)
```json
{"level":"info","msg":"File system event","event_type":"WRITE","file_path":"./demo-folder/file.txt","time":"2024-01-15 10:30:45"}
{"level":"info","msg":"Backup operation","operation":"create","source_path":"./demo-folder/file.txt","success":true,"time":"2024-01-15 10:30:45"}
{"level":"info","msg":"File system event","event_type":"WRITE","file_path":"./demo-folder/file2.txt","time":"2024-01-15 10:30:46"}
```

### Long-Term Mode (Optimized)
```json
{"level":"info","msg":"System activity summary","uptime":"2h30m15s","period":"10m0s","file_events":1247,"backup_operations":1247,"detection_events":0,"rollback_events":0,"errors":0,"events_per_minute":124.7,"time":"2024-01-15 10:40:45"}
{"level":"warn","msg":"Suspicious file event detected","event_type":"CREATE","file_path":"./demo-folder/document.txt.locked","time":"2024-01-15 10:41:22"}
```

## Benefits

1. **Reduced Log Size**: Up to 95% reduction in log volume for normal operations
2. **Better Performance**: Less I/O overhead from reduced logging
3. **Easier Monitoring**: Focus on important events and summaries
4. **Automatic Maintenance**: Log rotation prevents disk space issues
5. **Long-Term Stability**: Suitable for continuous 24/7 operation

## What's Still Logged

Even in long-term mode, the following are always logged:

- ✅ **Suspicious file events** (files with ransomware extensions)
- ✅ **Detection alerts** (pattern matches, threshold violations)
- ✅ **Rollback operations** (all restore activities)
- ✅ **System errors** (failures and exceptions)
- ✅ **Periodic summaries** (activity statistics every 10 minutes)
- ✅ **System startup/shutdown** (lifecycle events)

## What's Reduced/Removed

In long-term mode, the following are minimized:

- ❌ **Individual file events** (unless suspicious)
- ❌ **Successful backup operations** (only failures logged)
- ❌ **Debug information** (metadata, verbose details)
- ❌ **INFO level messages** (routine operational logs)
- ❌ **Quarantine operations** (functionality removed entirely)

## Monitoring in Long-Term Mode

To monitor system health in long-term mode:

1. **Check periodic summaries** for activity levels
2. **Watch for warning/error messages** for issues
3. **Monitor events per minute** for unusual activity spikes
4. **Review detection events** for security threats

## Dynamic Mode Switching

You can switch logging modes during runtime (future enhancement):

```go
// Enable long-term mode
system.EnableLongTermLogging()

// Return to standard mode
system.DisableLongTermLogging()
```

## Log File Management

The system automatically manages log files:

- **Current log**: `logs/journal.log`
- **Rotated logs**: `logs/journal.log.1`, `logs/journal.log.2`, etc.
- **Automatic cleanup**: Old logs beyond the retention limit are deleted

## Recommendations

For **24/7 production environments**:
- Enable `long_term_mode: true`
- Set `summary_interval: 10` (10 minutes)
- Use `max_log_size: 50` (50MB per file)
- Keep `max_log_files: 3` (3 rotated files)

For **development/testing**:
- Use `long_term_mode: false`
- Set `summary_interval: 5` (5 minutes)
- Increase `max_log_size: 100` (100MB per file)
- Keep more files `max_log_files: 5`

This optimization ensures your ransomware recovery system can run continuously without log files consuming excessive disk space while maintaining essential monitoring capabilities. 