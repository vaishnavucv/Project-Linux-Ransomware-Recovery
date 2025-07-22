# Manual Rollback System

This document explains the comprehensive manual rollback system that allows users to manually restore all backed up files with complete file structure preservation.

## Overview

The system provides both **automatic rollback** (triggered by suspicious activity detection) and **manual rollback** (triggered by command-line flag) capabilities. This document focuses on the manual rollback system that can be invoked using the `--rollback` flag.

## Features

### 🔄 **Complete File Structure Restoration**
- **Directory Recreation**: Automatically recreates the complete directory structure from backups
- **File Content Restoration**: Restores all files from their most recent backups
- **Malicious File Cleanup**: Removes suspicious files before restoration
- **Integrity Verification**: Verifies backup integrity using checksums
- **Parallel Processing**: Uses multiple workers for fast restoration

### 🛡️ **Security Features**
- **Malicious File Detection**: Identifies and removes files with suspicious extensions
- **Content Analysis**: Detects and removes files with ransomware-related content
- **Path Validation**: Prevents path traversal attacks during restoration
- **Backup Verification**: Ensures backup integrity before restoration

### 📊 **Comprehensive Reporting**
- **Detailed Results**: Shows total, restored, and failed file counts
- **File Lists**: Lists all successfully restored and failed files
- **Error Details**: Provides specific error messages for failed restorations
- **Performance Metrics**: Reports operation duration and statistics

## Command Usage

### Basic Manual Rollback
```bash
./ransomware-recovery --rollback
```

### Manual Rollback with Custom Config
```bash
./ransomware-recovery --rollback --config /path/to/config.json
```

### Help and Version
```bash
./ransomware-recovery --help     # Show help information
./ransomware-recovery --version  # Show version information
```

## System Architecture

### Auto Rollback vs Manual Rollback

```
Auto Rollback Flow:
File Events → Detector → Threshold Check → Auto Rollback → Restoration

Manual Rollback Flow:
Command Line → Direct Rollback → Complete Restoration
```

### Rollback Process Steps

```
1. 🧹 Malicious File Cleanup
   ├── Scan for suspicious extensions (.locked, .encrypted, etc.)
   ├── Check for ransomware content patterns
   └── Remove identified malicious files

2. 📁 Directory Structure Recreation  
   ├── Analyze backup database for directory structure
   ├── Create all necessary parent directories
   └── Set appropriate permissions

3. 📄 File Restoration
   ├── Get list of all backed up files
   ├── Restore files using parallel workers
   ├── Verify backup integrity with checksums
   └── Update file hashes after restoration

4. 📊 Result Reporting
   ├── Generate comprehensive statistics
   ├── List successful and failed restorations
   └── Log detailed operation results
```

## Configuration

The manual rollback system uses the same configuration as the main system:

```json
{
  "monitor_path": "./demo-folder",
  "backup_path": "./.backup",
  "log_path": "./logs",
  "backup": {
    "auto_backup": true,
    "enable_checksums": true,
    "retention_days": 30
  },
  "system": {
    "worker_count": 4,
    "operation_timeout": 30
  },
  "logging": {
    "long_term_mode": true,
    "summary_interval": 10,
    "max_log_size": 50,
    "max_log_files": 3
  }
}
```

## Example Output

### Successful Manual Rollback
```
=== Linux Ransomware Recovery System - Manual Rollback ===

🔄 Initiating manual rollback operation...
📁 Monitor Path: ./demo-folder
💾 Backup Path: ./.backup

📊 Rollback Results:
   Total Files: 15
   Restored Files: 15
   Failed Files: 0
   Duration: 45.123ms
   Success: true

✅ Successfully Restored Files:
   - demo-folder/documents/report.pdf
   - demo-folder/documents/presentation.pptx
   - demo-folder/config/settings.json
   - demo-folder/images/photo1.jpg
   - demo-folder/images/photo2.png
   ... (and 10 more files)

🎉 Manual rollback completed successfully!
📝 Check logs at: ./logs/journal.log
```

### Rollback with Errors
```
📊 Rollback Results:
   Total Files: 10
   Restored Files: 8
   Failed Files: 2
   Duration: 123.456ms
   Success: true

✅ Successfully Restored Files:
   - demo-folder/file1.txt
   - demo-folder/file2.txt
   ... (and 6 more files)

❌ Failed to Restore Files:
   - demo-folder/corrupted.txt (Error: backup integrity check failed)
   - demo-folder/missing.txt (Error: backup file not found)

⚠️  Manual rollback completed with errors.
📝 Check logs at: ./logs/journal.log for details
```

## Logging and Monitoring

### Timeline Logging
The system provides detailed timeline logging for all rollback operations:

```json
{
  "level": "warning",
  "msg": "Manual rollback initiated via command line",
  "operation": "manual_rollback",
  "initiated_by": "command_line",
  "config_path": "./config.json",
  "timestamp": "2025-07-21 20:05:49",
  "time": "2025-07-21 20:05:49"
}
```

### Completion Logging
```json
{
  "level": "info",
  "msg": "Manual rollback completed",
  "operation": "manual_rollback",
  "success": true,
  "total_files": 15,
  "restored_files": 15,
  "failed_files": 0,
  "duration": "45.123ms",
  "duration_ms": 45,
  "initiated_by": "command_line",
  "timestamp": "2025-07-21 20:05:49",
  "time": "2025-07-21 20:05:49"
}
```

### Malicious File Cleanup Logging
```json
{
  "level": "info",
  "msg": "Removed malicious file",
  "file_path": "demo-folder/document.txt.locked",
  "reason": "suspicious_file",
  "suspicious_ext": true,
  "suspicious_content": false,
  "time": "2025-07-21 20:05:49"
}
```

## Security Considerations

### Malicious File Detection
The system identifies and removes files with:
- **Suspicious Extensions**: `.locked`, `.encrypted`, `.crypto`, `.crypt`, `.enc`
- **Ransomware Content**: Files containing "RANSOMWARE", "DECRYPT", "BITCOIN", "PAYMENT"
- **Size Limits**: Only scans small files (<10KB) for content to avoid performance issues

### Path Security
- **Path Validation**: Ensures all operations stay within the monitor directory
- **Traversal Protection**: Prevents directory traversal attacks
- **Permission Handling**: Maintains appropriate file permissions during restoration

## Performance Characteristics

### Parallel Processing
- **Worker Pools**: Uses configurable number of workers for parallel file restoration
- **Concurrent Operations**: Multiple files restored simultaneously
- **Timeout Management**: Individual file operations have timeout protection

### Typical Performance
- **Small Files** (<1MB): ~5-10ms per file
- **Medium Files** (1-10MB): ~50-100ms per file
- **Large Files** (>10MB): ~500ms+ per file
- **Directory Creation**: ~1-2ms per directory
- **Malicious File Cleanup**: ~10-50ms depending on file count

## Use Cases

### 1. **Post-Attack Recovery**
When you suspect a ransomware attack has occurred:
```bash
# Stop the monitoring system if running
pkill -f ransomware-recovery

# Perform manual rollback
./ransomware-recovery --rollback

# Restart monitoring if needed
./ransomware-recovery
```

### 2. **Scheduled Maintenance**
For regular system maintenance or testing:
```bash
# Create a backup of current state
cp -r demo-folder demo-folder-backup

# Perform rollback to clean state
./ransomware-recovery --rollback

# Verify system integrity
ls -la demo-folder/
```

### 3. **Disaster Recovery**
For complete system restoration:
```bash
# Ensure backup database exists
ls -la .backup/backup_database.json

# Perform complete rollback
./ransomware-recovery --rollback

# Verify all files restored
find demo-folder -type f | wc -l
```

## Troubleshooting

### Common Issues

#### 1. **No Backup Files Found**
```
Error: no backup files found
```
**Solution**: Ensure the system has been running and creating backups, or check backup path configuration.

#### 2. **Backup Integrity Check Failed**
```
Error: backup integrity check failed: hash mismatch
```
**Solution**: Backup file may be corrupted. Check backup directory and consider restoring from older backup.

#### 3. **Permission Denied**
```
Error: failed to create directory: permission denied
```
**Solution**: Ensure the user has write permissions to the monitor and backup directories.

#### 4. **Disk Space Issues**
```
Error: no space left on device
```
**Solution**: Free up disk space or move backups to a different location.

### Verification Commands

```bash
# Check backup database
cat .backup/backup_database.json | jq '.file_hashes | length'

# Verify file integrity
find demo-folder -type f -exec sha256sum {} \; > current_hashes.txt

# Compare with backup database
cat .backup/backup_database.json | jq -r '.file_hashes | to_entries[] | "\(.value)  \(.key)"' > backup_hashes.txt

# Check logs for errors
grep -i error logs/journal.log | tail -10
```

## Integration with Auto Rollback

The manual rollback system complements the automatic rollback:

### Auto Rollback Triggers
- **Threshold Detection**: 3+ suspicious files within 60 seconds
- **Pattern Matching**: Files with ransomware extensions
- **Content Analysis**: Files with ransomware keywords

### Manual Rollback Advantages
- **Complete Control**: User decides when to rollback
- **No Thresholds**: Doesn't require suspicious activity detection
- **Comprehensive**: Always performs complete system restoration
- **Detailed Reporting**: Provides extensive feedback and logging

## Best Practices

### 1. **Regular Testing**
- Test manual rollback periodically to ensure it works
- Verify backup integrity regularly
- Monitor disk space usage

### 2. **Backup Verification**
- Enable checksums in configuration
- Monitor backup database size and health
- Verify critical files are being backed up

### 3. **Emergency Procedures**
- Document rollback procedures for your team
- Test rollback in non-production environment first
- Keep offline backups for critical data

### 4. **Monitoring**
- Monitor log files for rollback operations
- Set up alerts for failed rollback operations
- Track rollback performance metrics

The manual rollback system provides a robust, secure, and comprehensive solution for recovering from ransomware attacks or other data corruption scenarios. Combined with the automatic rollback capabilities, it offers complete protection for your critical files and directory structures. 