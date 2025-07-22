# File Deletion Alert System

This document explains the comprehensive file deletion alert system that provides high-priority logging and monitoring for file deletions, removals, and purges to detect potential ransomware attacks and data loss scenarios.

## Overview

The file deletion alert system monitors all file deletion events in the demo-folder and generates high-priority alerts based on configurable criteria. It provides immediate detection of suspicious deletion patterns that could indicate ransomware attacks or accidental data loss.

## Features

### 🚨 **High-Priority Alert Levels**
- **HIGH ALERT**: Individual file deletions with backup information
- **CRITICAL ALERT**: Suspicious deletions and mass deletion events
- **Always Logged**: Deletion alerts bypass long-term logging mode restrictions

### 🔍 **Detection Capabilities**
- **Individual File Tracking**: Every file deletion is logged with full details
- **Mass Deletion Detection**: Alerts when multiple files are deleted quickly
- **Suspicious Pattern Detection**: Identifies critical file deletions
- **Backup Status Tracking**: Shows whether deleted files have backups available
- **File Type Analysis**: Special alerts for valuable file types (documents, images, etc.)

### 📊 **Comprehensive Logging**
- **Detailed Metadata**: File paths, sizes, timestamps, backup information
- **Timeline Tracking**: Complete audit trail of all deletion events
- **Statistics Integration**: Deletion counts included in system statistics
- **Alert Categorization**: Different alert types for different scenarios

## Alert Types

### 1. **Individual File Deletion (HIGH ALERT)**
Triggered for every file deletion with the following information:
- File path and size
- Deletion timestamp
- Backup availability and location
- Alert level: `HIGH`

**Example Log:**
```json
{
  "alert_level": "HIGH",
  "event_type": "FILE_DELETION",
  "file_path": "demo-folder/document.pdf",
  "deletion_type": "removed",
  "details": "Original file deleted, backup available at: .backup/document_20250721_201514.pdf",
  "has_backup": true,
  "component": "agent",
  "timestamp": "2025-07-21 20:15:17.457",
  "level": "warning",
  "msg": "🚨 HIGH ALERT: File deletion detected"
}
```

### 2. **Mass File Deletion (CRITICAL ALERT)**
Triggered when multiple files (≥5 by default) are deleted within a short time window (2 minutes by default):
- Count of deleted files
- List of all deleted file paths
- Number of files with/without backups
- Time window for the deletions
- Alert level: `CRITICAL`

**Example Log:**
```json
{
  "alert_level": "CRITICAL",
  "event_type": "MASS_FILE_DELETION",
  "deleted_files_count": 6,
  "files_with_backup": 6,
  "files_without_backup": 0,
  "time_window": "2m0s",
  "deleted_files": [
    "demo-folder/test_file_1.txt",
    "demo-folder/test_file_2.txt",
    "demo-folder/test_file_3.txt",
    "demo-folder/test_file_4.txt",
    "demo-folder/test_file_5.txt",
    "demo-folder/test_file_6.txt"
  ],
  "component": "agent",
  "timestamp": "2025-07-21 20:16:04.903",
  "level": "error",
  "msg": "🚨🚨 CRITICAL ALERT: Mass file deletion detected - Possible ransomware attack!"
}
```

### 3. **Suspicious File Deletion (CRITICAL ALERT)**
Triggered when deleted files match suspicious patterns:
- Critical files (containing "important", "critical", "backup")
- Valuable file types (.pdf, .doc, .jpg, .mp4, etc.)
- Large files (>1MB by default)
- Files without backups
- Alert level: `CRITICAL`

**Example Log:**
```json
{
  "alert_level": "CRITICAL",
  "event_type": "SUSPICIOUS_DELETION",
  "file_path": "demo-folder/important_document.pdf",
  "reason": "critical_file_deleted, valuable_file_type_deleted",
  "backup_info": {
    "has_backup": true,
    "backup_path": ".backup/important_document_20250721_201514.pdf",
    "file_size": 19,
    "deleted_at": "2025-07-21 20:15:17"
  },
  "component": "detector",
  "timestamp": "2025-07-21 20:15:17.457",
  "level": "error",
  "msg": "🚨🚨 CRITICAL ALERT: Suspicious file deletion detected"
}
```

## Configuration

### Deletion Alert Settings

The deletion alert system is configured in the `detection.deletion_alert` section:

```json
{
  "detection": {
    "deletion_alert": {
      "enabled": true,
      "mass_threshold": 5,
      "time_window": 2,
      "alert_on_valuable_files": true,
      "alert_on_no_backup": true,
      "large_file_threshold": 1
    }
  }
}
```

### Configuration Options

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `enabled` | boolean | `true` | Enable/disable deletion alerts |
| `mass_threshold` | integer | `5` | Number of deletions to trigger mass deletion alert |
| `time_window` | integer | `2` | Time window in minutes for mass deletion detection |
| `alert_on_valuable_files` | boolean | `true` | Alert on deletion of valuable file types |
| `alert_on_no_backup` | boolean | `true` | Alert when files without backups are deleted |
| `large_file_threshold` | integer | `1` | File size threshold in MB for large file alerts |

## Suspicious Deletion Patterns

### Critical File Patterns
Files containing these keywords in their path trigger alerts:
- `important`
- `critical` 
- `backup`

### Valuable File Types
The following file extensions are considered valuable:
- **Documents**: `.doc`, `.docx`, `.pdf`, `.xlsx`, `.pptx`
- **Images**: `.jpg`, `.png`
- **Videos**: `.mp4`

### Large File Detection
Files larger than the configured threshold (default 1MB) trigger alerts as they represent potential data loss.

### No Backup Alert
Files deleted without available backups trigger critical alerts as they cannot be recovered.

## Integration with Backup System

### Backup Database Tracking
The system maintains deletion status in the backup database:
- `is_deleted`: Boolean flag indicating if original file was deleted
- `deleted_at`: Timestamp when the file was deleted
- Backup files remain available for recovery even after deletion

### Statistics Integration
Deletion statistics are included in system reports:
- Total tracked files
- Active files (not deleted)
- Deleted files count
- Files with/without backups

## Monitoring and Alerting

### Log Analysis Commands

```bash
# View all deletion alerts
grep -E "(HIGH ALERT|CRITICAL ALERT)" logs/journal.log

# View mass deletion events
grep "MASS_FILE_DELETION" logs/journal.log

# View suspicious deletions
grep "SUSPICIOUS_DELETION" logs/journal.log

# Count deletion events by type
grep "FILE_DELETION" logs/journal.log | wc -l
grep "MASS_FILE_DELETION" logs/journal.log | wc -l
```

### Real-time Monitoring

```bash
# Monitor deletion alerts in real-time
tail -f logs/journal.log | grep -E "(🚨|ALERT)"

# Monitor specific alert types
tail -f logs/journal.log | grep "CRITICAL ALERT"
```

### Statistics Queries

```bash
# Check backup database for deletion statistics
cat .backup/backup_database.json | jq '{
  total_files: (.backup_db | length),
  deleted_files: [.backup_db[] | select(.is_deleted == true)] | length,
  active_files: [.backup_db[] | select(.is_deleted != true)] | length
}'
```

## Performance Impact

### Minimal Overhead
- Deletion tracking adds minimal performance overhead
- Mass deletion detection uses efficient time-window sliding
- Alert generation is asynchronous and non-blocking

### Memory Usage
- Deletion tracker maintains recent deletion events in memory
- Automatic cleanup removes old events outside time window
- Typical memory usage: <1MB for deletion tracking

## Use Cases

### 1. **Ransomware Detection**
**Scenario**: Ransomware encrypts and deletes original files
**Detection**: Mass deletion alert + suspicious file pattern alerts
**Response**: Immediate notification allows for quick rollback

### 2. **Accidental Data Loss**
**Scenario**: User accidentally deletes important files
**Detection**: Individual deletion alerts + backup status
**Response**: Easy identification of deleted files for recovery

### 3. **System Compromise**
**Scenario**: Attacker deletes files to cover tracks
**Detection**: Suspicious deletion patterns + no backup alerts
**Response**: Forensic timeline reconstruction from logs

### 4. **Storage Cleanup Gone Wrong**
**Scenario**: Automated cleanup script deletes wrong files
**Detection**: Mass deletion alert with backup status
**Response**: Bulk recovery using backup information

## Alert Response Procedures

### For HIGH ALERTS (Individual Deletions)
1. **Verify Intent**: Check if deletion was intentional
2. **Assess Impact**: Review file importance and backup status
3. **Document**: Log the incident if necessary
4. **Recovery**: Use backup if deletion was accidental

### For CRITICAL ALERTS (Mass/Suspicious Deletions)
1. **Immediate Response**: Stop any running processes that might be causing deletions
2. **Assessment**: Determine if this is a ransomware attack or system compromise
3. **Isolation**: Consider isolating the system if attack is suspected
4. **Recovery Planning**: Prepare for manual rollback if needed
5. **Investigation**: Analyze logs for attack patterns and entry points

## Integration with Existing Systems

### SIEM Integration
Deletion alerts can be forwarded to SIEM systems:
- JSON structured logs are SIEM-friendly
- Alert levels map to SIEM severity levels
- Timestamps enable correlation with other events

### Monitoring Tools
Integration with monitoring systems:
- Log parsing for alert extraction
- Metrics generation from deletion statistics
- Dashboard creation for deletion trends

### Notification Systems
Alert forwarding options:
- Email notifications for critical alerts
- Slack/Teams integration for team notifications
- SMS alerts for high-priority incidents

## Troubleshooting

### Common Issues

#### 1. **Too Many False Positives**
**Symptoms**: Frequent alerts for normal file operations
**Solutions**: 
- Adjust `mass_threshold` to higher value
- Disable `alert_on_valuable_files` if too sensitive
- Increase `large_file_threshold`

#### 2. **Missing Alerts**
**Symptoms**: Expected deletions not generating alerts
**Solutions**:
- Verify `enabled: true` in configuration
- Check file paths are within monitor directory
- Ensure system is running during deletion events

#### 3. **Performance Issues**
**Symptoms**: System slowdown during deletion tracking
**Solutions**:
- Reduce `time_window` to limit memory usage
- Increase cleanup frequency for deletion tracker
- Monitor system resources during high deletion periods

### Diagnostic Commands

```bash
# Check deletion alert configuration
cat config.json | jq '.detection.deletion_alert'

# Verify system is tracking deletions
grep "trackDeletion" logs/journal.log | tail -5

# Check deletion tracker statistics
./ransomware-recovery --help  # (Future: add stats command)
```

## Security Considerations

### Alert Integrity
- Alerts are logged immediately and cannot be suppressed
- Multiple log destinations prevent single point of failure
- Structured logging enables automated analysis

### Attack Resistance
- Deletion tracking continues even if main process is compromised
- Backup database provides independent deletion record
- Log rotation prevents log tampering through size manipulation

The file deletion alert system provides comprehensive protection against data loss scenarios while maintaining high performance and configurability for different operational environments. 