# Remote Rollback System with Rsync over SSH

This document explains the comprehensive remote rollback system that can restore files and directory structures from an Ubuntu backup server using Rsync over SSH when using the `--rollback` command.

## Overview

The remote rollback system provides complete disaster recovery capabilities by downloading and restoring backup files from a remote Ubuntu server. It supports intelligent rollback source selection, hybrid redundancy, and comprehensive file structure restoration with detailed logging and status tracking.

## Features

### 🚀 **Intelligent Rollback Source Selection**
- **Local Rollback**: Uses local backup files when available and remote is disabled
- **Remote Rollback**: Downloads backups from remote server via Rsync over SSH
- **Hybrid Rollback**: Attempts remote first, falls back to local if remote fails
- **Automatic Detection**: Intelligently chooses the best available source

### 🔄 **Complete Disaster Recovery**
- **File Structure Restoration**: Recreates complete directory hierarchy from remote backups
- **Staging Area**: Downloads remote files to temporary staging before restoration
- **Integrity Verification**: Verifies file integrity after remote rollback
- **Current State Backup**: Optionally backs up current state before rollback

### 📡 **Secure Remote Operations**
- **SSH Key Authentication**: Uses SSH keys for secure remote access
- **Encrypted Transfer**: All data transfers encrypted via SSH tunnel
- **Connection Testing**: Verifies remote server availability before rollback
- **Retry Logic**: Configurable retry attempts with exponential backoff

### 📊 **Comprehensive Logging & Monitoring**
- **Timeline Logging**: Detailed audit trail of all rollback operations
- **Performance Metrics**: Transfer speeds, file counts, success rates
- **Error Tracking**: Detailed error reporting and troubleshooting
- **Status Updates**: Real-time progress and status information

## Configuration

### Remote Rollback Settings

The remote rollback system is configured in the `backup.remote_backup.remote_rollback` section:

```json
{
  "backup": {
    "remote_backup": {
      "enabled": false,
      "host": "your-backup-server.com",
      "username": "backup-user",
      "port": 22,
      "remote_path": "/home/backup-user/ransomware-backups",
      "ssh_key_path": "",
      "rsync_options": ["-avz", "--delete", "--partial", "--progress"],
      "sync_interval": 5,
      "connection_timeout": 30,
      "max_retries": 3,
      "retry_delay": 10,
      "enable_compression": true,
      "bandwidth_limit": 0,
      "remote_rollback": {
        "enabled": false,
        "prefer_remote": true,
        "backup_before_rollback": true,
        "verify_integrity": true,
        "rollback_options": ["-avz", "--delete", "--partial", "--progress"],
        "temp_dir": "./.temp-rollback"
      }
    }
  }
}
```

### Configuration Options

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `enabled` | boolean | `false` | Enable/disable remote rollback functionality |
| `prefer_remote` | boolean | `true` | Prefer remote rollback over local when both available |
| `backup_before_rollback` | boolean | `true` | Create backup of current state before rollback |
| `verify_integrity` | boolean | `true` | Verify file integrity after remote rollback |
| `rollback_options` | array | `["-avz", "--delete", "--partial", "--progress"]` | Rsync options for rollback |
| `temp_dir` | string | `"./.temp-rollback"` | Temporary staging directory for remote files |

## Rollback Source Selection

The system intelligently determines the best rollback source:

### Decision Matrix

```
1. Remote Rollback Configuration Check
   ├── Remote backup disabled → LOCAL
   ├── Remote rollback disabled → LOCAL
   └── Remote enabled → Continue to availability check

2. Remote Server Availability Check
   ├── Remote server unavailable → LOCAL
   └── Remote server available → Continue to preference check

3. Preference and Backup Availability
   ├── prefer_remote = true → REMOTE
   ├── No local backups available → REMOTE
   ├── Local backups available → HYBRID
   └── Default → LOCAL
```

### Rollback Source Types

1. **Local Rollback** (`"local"`)
   - Uses local backup files only
   - Fastest option when local backups are available
   - No network dependency

2. **Remote Rollback** (`"remote"`)
   - Downloads files from remote server via Rsync over SSH
   - Complete disaster recovery capability
   - Useful when local backups are corrupted or unavailable

3. **Hybrid Rollback** (`"hybrid"`)
   - Attempts remote rollback first
   - Falls back to local rollback if remote fails
   - Provides maximum reliability and redundancy

## Remote Rollback Process

### Step-by-Step Process

```
1. 📋 Source Determination
   ├── Check remote rollback configuration
   ├── Test remote server connectivity
   ├── Evaluate local backup availability
   └── Select optimal rollback source

2. 🗂️ Staging Setup
   ├── Create temporary staging directory
   ├── Clean any existing staging data
   └── Set proper permissions

3. 💾 Current State Backup (Optional)
   ├── Check backup_before_rollback setting
   ├── Create snapshot of current state
   └── Store in dated backup directory

4. 📡 Remote Download
   ├── Build rsync download command
   ├── Execute download with retry logic
   ├── Monitor transfer progress
   └── Verify download completion

5. 🧹 Malicious File Cleanup
   ├── Scan for suspicious files
   ├── Remove ransomware-encrypted files
   └── Clean up potential threats

6. 📁 File Restoration
   ├── Use rsync to restore from staging
   ├── Preserve file permissions and timestamps
   ├── Recreate directory structure
   └── Count restored files

7. ✅ Integrity Verification (Optional)
   ├── Check verify_integrity setting
   ├── Verify file existence and accessibility
   ├── Report verification results
   └── Log any integrity issues

8. 🧼 Cleanup
   ├── Remove temporary staging directory
   ├── Clean up temporary files
   └── Report final results
```

## Usage Examples

### 1. **Enable Remote Rollback**

To enable remote rollback functionality, update `config.json`:

```json
{
  "backup": {
    "remote_backup": {
      "enabled": true,
      "host": "192.168.1.100",
      "username": "backup-user",
      "ssh_key_path": "/home/user/.ssh/backup_key",
      "remote_rollback": {
        "enabled": true,
        "prefer_remote": true
      }
    }
  }
}
```

### 2. **Hybrid Rollback Configuration**

For maximum reliability with local backup as fallback:

```json
{
  "backup": {
    "remote_backup": {
      "enabled": true,
      "host": "backup-server.com",
      "username": "backup-user",
      "remote_rollback": {
        "enabled": true,
        "prefer_remote": false,
        "backup_before_rollback": true,
        "verify_integrity": true
      }
    }
  }
}
```

### 3. **Performance Optimized Configuration**

For faster rollback with compression and bandwidth control:

```json
{
  "backup": {
    "remote_backup": {
      "bandwidth_limit": 2048,
      "enable_compression": true,
      "max_retries": 5,
      "retry_delay": 5,
      "remote_rollback": {
        "enabled": true,
        "rollback_options": ["-avz", "--compress-level=6", "--partial", "--progress"],
        "verify_integrity": false
      }
    }
  }
}
```

## Command Usage

### Basic Remote Rollback

```bash
# Perform rollback (automatically determines source)
./ransomware-recovery --rollback
```

### With Custom Configuration

```bash
# Use custom config with remote rollback enabled
./ransomware-recovery --rollback --config /path/to/remote-config.json
```

### Environment Variables

```bash
# Set SSH key path via environment
export SSH_KEY_PATH="/home/user/.ssh/backup_key"
./ransomware-recovery --rollback
```

## Timeline Logging Examples

### Rollback Source Determination
```json
{
  "level": "info",
  "msg": "Determined rollback source",
  "rollback_source": "remote",
  "component": "rollback",
  "event_type": "rollback_source_determined",
  "time": "2025-07-21 22:15:30"
}
```

### Remote Server Availability Check
```json
{
  "level": "debug",
  "msg": "Remote server availability check",
  "remote_host": "backup-server.com",
  "available": true,
  "component": "rollback",
  "time": "2025-07-21 22:15:31"
}
```

### Remote Rollback Initiation
```json
{
  "level": "info",
  "msg": "Starting remote rollback operation",
  "component": "rollback",
  "event_type": "remote_rollback_start",
  "remote_host": "backup-server.com",
  "remote_path": "/home/backup-user/ransomware-backups",
  "time": "2025-07-21 22:15:32"
}
```

### Staging Directory Creation
```json
{
  "level": "info",
  "msg": "Created temporary staging directory",
  "temp_dir": "./.temp-rollback",
  "component": "rollback",
  "event_type": "temp_staging_created",
  "time": "2025-07-21 22:15:33"
}
```

### Current State Backup
```json
{
  "level": "info",
  "msg": "Backed up current state before remote rollback",
  "backup_dir": "./.backup-pre-remote-rollback-1642795533",
  "component": "rollback",
  "event_type": "current_state_backed_up",
  "time": "2025-07-21 22:15:34"
}
```

### Remote Download Progress
```json
{
  "level": "info",
  "msg": "Attempting remote backup download",
  "attempt": 1,
  "max_tries": 3,
  "component": "rollback",
  "time": "2025-07-21 22:15:35"
}
```

### Download Success
```json
{
  "level": "info",
  "msg": "Remote backup download completed successfully",
  "component": "rollback",
  "event_type": "remote_download_success",
  "attempt": 1,
  "output_size": 2048576,
  "time": "2025-07-21 22:15:45"
}
```

### File Restoration
```json
{
  "level": "info",
  "msg": "Starting restore from staging area",
  "component": "rollback",
  "event_type": "staging_restore_start",
  "source": "./.temp-rollback",
  "dest": "./demo-folder",
  "time": "2025-07-21 22:15:46"
}
```

### Restoration Success
```json
{
  "level": "info",
  "msg": "Successfully restored files from staging area",
  "component": "rollback",
  "event_type": "staging_restore_success",
  "restored_files": 25,
  "time": "2025-07-21 22:15:48"
}
```

### Integrity Verification
```json
{
  "level": "info",
  "msg": "Completed integrity verification",
  "component": "rollback",
  "event_type": "integrity_verification_complete",
  "verified_files": 25,
  "failed_files": 0,
  "time": "2025-07-21 22:15:50"
}
```

### Rollback Completion
```json
{
  "level": "info",
  "msg": "Remote rollback operation completed",
  "total_files": 25,
  "restored_files": 25,
  "failed_files": 0,
  "duration": "18.456s",
  "success": true,
  "rollback_type": "remote",
  "remote_host": "backup-server.com",
  "time": "2025-07-21 22:15:50"
}
```

### Hybrid Rollback Fallback
```json
{
  "level": "warn",
  "msg": "Remote rollback failed, attempting local rollback",
  "remote_error": "ssh: connect to host backup-server.com port 22: Connection refused",
  "time": "2025-07-21 22:15:51"
}
```

## Monitoring and Management

### Log Analysis Commands

```bash
# View remote rollback logs
grep "remote_rollback" logs/journal.log | tail -20

# Check rollback source decisions
grep "rollback_source_determined" logs/journal.log

# Monitor remote download progress
grep -E "(remote_download|staging_restore)" logs/journal.log

# Check integrity verification results
grep "integrity_verification" logs/journal.log

# View hybrid rollback operations
grep "hybrid_rollback" logs/journal.log
```

### Performance Monitoring

```bash
# Calculate average download times
grep "remote_download_success" logs/journal.log | \
  jq -r '.duration' | \
  awk '{sum+=$1; count++} END {print "Average download time:", sum/count, "seconds"}'

# Check rollback success rates by source
grep "rollback_type" logs/journal.log | \
  jq -r '"\(.rollback_type): \(.success)"' | \
  sort | uniq -c

# Monitor file restoration counts
grep "staging_restore_success" logs/journal.log | \
  jq '.restored_files' | \
  awk '{sum+=$1; count++} END {print "Total files restored:", sum, "Average per rollback:", sum/count}'
```

### Status Checking

```bash
# Check remote server connectivity
ssh -o ConnectTimeout=5 backup-user@backup-server.com 'echo "Connection test successful"'

# Test rsync command manually
rsync -avz --dry-run backup-user@backup-server.com:/home/backup-user/ransomware-backups/ ./.temp-test/

# Verify staging directory permissions
ls -la ./.temp-rollback/
```

## Security Considerations

### Remote Access Security
- **SSH Key Management**: Use dedicated SSH keys for backup operations
- **Host Key Verification**: Configure known_hosts for server verification
- **Connection Timeouts**: Prevent hanging connections with reasonable timeouts
- **Access Logging**: Monitor SSH access logs on remote server

### Data Protection
- **Encryption in Transit**: All transfers encrypted via SSH
- **Staging Security**: Temporary files stored securely with proper permissions
- **Cleanup Procedures**: Automatic cleanup of temporary staging data
- **Integrity Verification**: Optional verification ensures data integrity

### Access Control
- **Dedicated User**: Use separate backup user on remote server
- **Path Restrictions**: Limit access to backup directories only
- **Permission Management**: Proper file and directory permissions
- **Audit Trail**: Complete logging of all rollback operations

## Troubleshooting

### Common Issues

#### 1. **Remote Server Connection Failed**
**Symptoms**: `ssh: connect to host ... port 22: Connection refused`
**Solutions**:
- Verify remote server is running and accessible
- Check firewall settings on remote server
- Confirm SSH service is running: `sudo systemctl status sshd`
- Test manual SSH connection: `ssh backup-user@backup-server.com`

#### 2. **SSH Authentication Failed**
**Symptoms**: `Permission denied (publickey)`
**Solutions**:
- Verify SSH key path in configuration
- Check SSH key permissions: `chmod 600 ~/.ssh/backup_key`
- Ensure public key is in remote authorized_keys
- Test SSH key: `ssh -i ~/.ssh/backup_key backup-user@backup-server.com`

#### 3. **Download Fails with rsync Errors**
**Symptoms**: Various rsync error messages
**Solutions**:
- Check remote directory permissions
- Verify rsync is installed on both systems
- Test rsync manually with same options
- Check available disk space for staging

#### 4. **Staging Directory Issues**
**Symptoms**: `Failed to create temp staging directory`
**Solutions**:
- Check disk space: `df -h`
- Verify write permissions in parent directory
- Clean up any existing staging directories
- Check temp_dir configuration path

#### 5. **Integrity Verification Failures**
**Symptoms**: `Integrity verification failed for X files`
**Solutions**:
- Check file permissions after restoration
- Verify rsync completed successfully
- Review rsync output for errors
- Test manual file access in restored directory

### Diagnostic Commands

```bash
# Test remote connectivity
telnet backup-server.com 22

# Check SSH configuration
ssh -vvv -i ~/.ssh/backup_key backup-user@backup-server.com

# Test rsync with verbose output
rsync -avz --dry-run --verbose backup-user@backup-server.com:/path/to/backups/ ./test-staging/

# Check staging directory
ls -la ./.temp-rollback/
du -sh ./.temp-rollback/

# Verify restored files
find ./demo-folder -type f -exec ls -la {} \;
```

## Performance Optimization

### Network Optimization
- **Compression**: Enable compression for slower connections
- **Bandwidth Limiting**: Use bandwidth_limit for shared connections
- **Parallel Transfers**: Rsync automatically optimizes transfer efficiency
- **Incremental Transfers**: Only downloads changed files

### Disk Optimization
- **SSD Storage**: Use SSD for staging directory when possible
- **Separate Disk**: Use different disk for staging to avoid I/O contention
- **Cleanup Strategy**: Regular cleanup of old staging directories
- **Space Monitoring**: Monitor available disk space

### Configuration Tuning
- **Retry Settings**: Adjust max_retries and retry_delay for network conditions
- **Timeout Values**: Optimize connection_timeout for network latency
- **Rsync Options**: Fine-tune rsync options for specific use cases
- **Verification**: Disable integrity verification for faster rollbacks

## Best Practices

### 1. **Pre-Deployment Testing**
- Test SSH connectivity and authentication
- Verify rsync functionality with test data
- Test rollback procedures in non-production environment
- Document and train staff on rollback procedures

### 2. **Operational Procedures**
- Regular testing of remote rollback functionality
- Monitor remote server health and capacity
- Maintain current SSH keys and access credentials
- Document rollback decision criteria

### 3. **Security Practices**
- Regular SSH key rotation
- Monitor access logs on remote server
- Use dedicated network for backup traffic when possible
- Implement proper backup retention policies

### 4. **Monitoring and Alerting**
- Set up alerts for rollback failures
- Monitor rollback duration trends
- Track success rates by rollback source
- Alert on remote server connectivity issues

The remote rollback system provides enterprise-grade disaster recovery capabilities with intelligent source selection, comprehensive logging, and robust error handling, ensuring your data can be recovered from remote backup servers even in the most severe ransomware attack scenarios. 