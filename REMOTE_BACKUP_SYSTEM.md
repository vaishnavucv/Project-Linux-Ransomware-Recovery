# Remote Backup System with Rsync over SSH

This document explains the comprehensive remote backup system that synchronizes local backups to an Ubuntu server using Rsync over SSH for off-site backup redundancy and disaster recovery.

## Overview

The remote backup system provides secure, reliable, and efficient synchronization of local backup files to a remote Ubuntu server. It uses Rsync over SSH to ensure data integrity, security, and bandwidth efficiency while providing comprehensive monitoring and error handling.

## Features

### 🔐 **Secure SSH Connection**
- **SSH Key Authentication**: Supports SSH key-based authentication for secure access
- **SSH Agent Support**: Uses SSH agent when no key file is specified
- **Connection Security**: Secure encrypted data transfer over SSH
- **Host Verification**: Configurable host key verification settings

### 📡 **Efficient Rsync Synchronization**
- **Incremental Sync**: Only transfers changed files to minimize bandwidth usage
- **Compression**: Optional data compression during transfer
- **Progress Monitoring**: Real-time transfer progress tracking
- **Bandwidth Control**: Configurable bandwidth limiting
- **Partial Transfer Recovery**: Resumes interrupted transfers

### 🔄 **Flexible Sync Modes**
- **Immediate Sync**: Sync after every local backup (sync_interval = 0)
- **Periodic Sync**: Sync at configurable intervals (e.g., every 5 minutes)
- **Manual Sync**: On-demand synchronization via API
- **Automatic Retry**: Configurable retry attempts with delay

### 📊 **Comprehensive Monitoring**
- **Sync Status Tracking**: Real-time status of remote synchronization
- **Performance Metrics**: Transfer duration, bandwidth usage, file counts
- **Error Logging**: Detailed error reporting and troubleshooting
- **Timeline Logging**: Complete audit trail of sync operations

## Configuration

### Remote Backup Settings

The remote backup system is configured in the `backup.remote_backup` section:

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
      "bandwidth_limit": 0
    }
  }
}
```

### Configuration Options

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `enabled` | boolean | `false` | Enable/disable remote backup |
| `host` | string | `""` | Remote server hostname or IP address |
| `username` | string | `""` | SSH username for remote server |
| `port` | integer | `22` | SSH port number |
| `remote_path` | string | `""` | Remote directory path to store backups |
| `ssh_key_path` | string | `""` | SSH private key file path (optional) |
| `rsync_options` | array | `["-avz", "--delete", "--partial", "--progress"]` | Rsync command options |
| `sync_interval` | integer | `5` | Sync interval in minutes (0 = immediate) |
| `connection_timeout` | integer | `30` | Connection timeout in seconds |
| `max_retries` | integer | `3` | Maximum retry attempts for failed syncs |
| `retry_delay` | integer | `10` | Delay between retries in seconds |
| `enable_compression` | boolean | `true` | Enable compression during transfer |
| `bandwidth_limit` | integer | `0` | Bandwidth limit in KB/s (0 = no limit) |

## Ubuntu Server Setup

### 1. **Server Preparation**

```bash
# Create backup user
sudo useradd -m -s /bin/bash backup-user
sudo passwd backup-user

# Create backup directory
sudo mkdir -p /home/backup-user/ransomware-backups
sudo chown backup-user:backup-user /home/backup-user/ransomware-backups
sudo chmod 755 /home/backup-user/ransomware-backups
```

### 2. **SSH Key Setup**

On the client system (ransomware recovery system):
```bash
# Generate SSH key pair
ssh-keygen -t rsa -b 4096 -f ~/.ssh/backup_key
```

On the Ubuntu server:
```bash
# Create .ssh directory
sudo -u backup-user mkdir -p /home/backup-user/.ssh
sudo -u backup-user chmod 700 /home/backup-user/.ssh

# Copy public key to server
sudo -u backup-user tee /home/backup-user/.ssh/authorized_keys << 'EOF'
ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAAC... your-public-key-here
EOF

# Set proper permissions
sudo -u backup-user chmod 600 /home/backup-user/.ssh/authorized_keys
```

### 3. **SSH Configuration**

On the Ubuntu server, edit `/etc/ssh/sshd_config`:
```bash
# Allow key-based authentication
PubkeyAuthentication yes
AuthorizedKeysFile .ssh/authorized_keys

# Optional: Disable password authentication for security
PasswordAuthentication no

# Restart SSH service
sudo systemctl restart sshd
```

### 4. **Firewall Configuration**

```bash
# Allow SSH access
sudo ufw allow ssh
sudo ufw enable
```

## System Architecture

### Sync Flow

```
1. 📁 Local Backup Creation
   ├── File is backed up locally
   ├── Backup database is updated
   └── Remote sync trigger is evaluated

2. 🔄 Sync Decision
   ├── Check if remote backup is enabled
   ├── Check sync interval requirements
   └── Determine if sync is needed

3. 📡 Remote Synchronization
   ├── Build rsync command with SSH options
   ├── Execute rsync with retry logic
   ├── Monitor transfer progress
   └── Log results and update status

4. 📊 Status Update
   ├── Update last sync timestamp
   ├── Log performance metrics
   ├── Update sync status
   └── Schedule next sync (if periodic)
```

### Integration Points

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│  Local Backup   │───▶│  Remote Sync     │───▶│  Ubuntu Server  │
│  (.backup/)     │    │  (Rsync/SSH)     │    │  (Remote Path)  │
└─────────────────┘    └──────────────────┘    └─────────────────┘
         │                       │                       │
         ▼                       ▼                       ▼
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│ Backup Database │    │  Sync Scheduler  │    │ Remote Storage  │
│ (metadata)      │    │  (periodic)      │    │ (redundancy)    │
└─────────────────┘    └──────────────────┘    └─────────────────┘
```

## Usage Examples

### 1. **Enable Remote Backup**

Update `config.json`:
```json
{
  "backup": {
    "remote_backup": {
      "enabled": true,
      "host": "192.168.1.100",
      "username": "backup-user",
      "port": 22,
      "remote_path": "/home/backup-user/ransomware-backups",
      "ssh_key_path": "/home/user/.ssh/backup_key",
      "sync_interval": 5
    }
  }
}
```

### 2. **Immediate Sync Mode**

For immediate sync after every backup:
```json
{
  "backup": {
    "remote_backup": {
      "enabled": true,
      "sync_interval": 0,
      "host": "backup-server.com",
      "username": "backup-user"
    }
  }
}
```

### 3. **Bandwidth Limited Sync**

For limited bandwidth environments:
```json
{
  "backup": {
    "remote_backup": {
      "enabled": true,
      "bandwidth_limit": 1024,
      "enable_compression": true,
      "rsync_options": ["-avz", "--delete", "--partial"]
    }
  }
}
```

## Timeline Logging Examples

### Sync Initiation
```json
{
  "level": "info",
  "msg": "Starting remote backup synchronization",
  "remote_host": "backup-server.com",
  "remote_username": "backup-user",
  "remote_path": "/home/backup-user/ransomware-backups",
  "component": "backup",
  "event_type": "remote_sync_start",
  "time": "2025-07-21 21:30:15"
}
```

### Sync Success
```json
{
  "level": "info",
  "msg": "Remote backup synchronization completed successfully",
  "duration": "2.345s",
  "duration_ms": 2345,
  "attempt": 1,
  "output_size": 1024,
  "component": "backup",
  "event_type": "remote_sync_success",
  "time": "2025-07-21 21:30:17"
}
```

### Sync Failure
```json
{
  "level": "error",
  "msg": "Remote backup synchronization failed after all retries",
  "duration": "45.123s",
  "attempts": 3,
  "final_error": "ssh: connect to host backup-server.com port 22: Connection refused",
  "component": "backup",
  "event_type": "remote_sync_failed",
  "time": "2025-07-21 21:31:00"
}
```

### Periodic Sync Status
```json
{
  "level": "info",
  "msg": "Started periodic remote backup sync",
  "sync_interval": "5m0s",
  "component": "backup",
  "time": "2025-07-21 21:25:00"
}
```

## Monitoring and Management

### Status Checking

The system provides comprehensive status information:

```bash
# Check system statistics (includes remote sync status)
curl -s http://localhost:8080/stats | jq '.backup.remote_sync'
```

Example status response:
```json
{
  "enabled": true,
  "host": "backup-server.com",
  "username": "backup-user",
  "remote_path": "/home/backup-user/ransomware-backups",
  "sync_interval": 5,
  "last_sync": "2025-07-21 21:30:17",
  "status": "up_to_date",
  "time_since_last_sync": "2m43s"
}
```

### Log Analysis Commands

```bash
# View remote sync logs
grep "remote_sync" logs/journal.log | tail -10

# Check sync success rate
grep "remote_sync_success" logs/journal.log | wc -l
grep "remote_sync_failed" logs/journal.log | wc -l

# Monitor sync performance
grep "remote_sync_success" logs/journal.log | jq '.duration_ms' | \
  awk '{sum+=$1; count++} END {print "Average:", sum/count, "ms"}'

# Check for connection issues
grep -E "(Connection refused|timeout|ssh.*error)" logs/journal.log
```

### Real-time Monitoring

```bash
# Monitor sync activity
tail -f logs/journal.log | grep -E "(remote_sync|rsync)"

# Watch sync status
watch -n 5 'grep "remote_sync" logs/journal.log | tail -3'
```

## Performance Characteristics

### Bandwidth Usage
- **Initial Sync**: Full backup directory transfer
- **Incremental Sync**: Only changed files (typically <1% of total)
- **Compression**: 20-70% bandwidth reduction (depending on file types)
- **Bandwidth Limiting**: Configurable rate limiting

### Typical Performance
- **Small Changes** (<10MB): 1-5 seconds sync time
- **Medium Changes** (10-100MB): 10-60 seconds sync time
- **Large Changes** (>100MB): 1-10 minutes sync time
- **Network Impact**: Minimal during incremental syncs

### Resource Usage
- **CPU**: Low (rsync is efficient)
- **Memory**: Minimal (streaming transfers)
- **Disk I/O**: Moderate (reading backup files)
- **Network**: Configurable bandwidth usage

## Security Considerations

### SSH Security
- **Key-Based Authentication**: More secure than passwords
- **Host Key Verification**: Prevents man-in-the-middle attacks
- **Encrypted Transfer**: All data encrypted in transit
- **Connection Timeouts**: Prevents hanging connections

### Access Control
- **Dedicated User**: Separate backup user on remote server
- **Limited Permissions**: Backup user has minimal system access
- **Path Restrictions**: Backups stored in dedicated directory
- **Firewall Rules**: Restrict SSH access to known IPs

### Data Protection
- **Encryption in Transit**: SSH provides encryption
- **File Integrity**: Rsync checksums verify data integrity
- **Backup Verification**: Local checksums match remote files
- **Access Logging**: SSH logs all connection attempts

## Troubleshooting

### Common Issues

#### 1. **Connection Refused**
**Symptoms**: `ssh: connect to host ... port 22: Connection refused`
**Solutions**:
- Check if SSH service is running on remote server
- Verify firewall allows SSH connections
- Confirm correct hostname/IP and port

#### 2. **Authentication Failed**
**Symptoms**: `Permission denied (publickey)`
**Solutions**:
- Verify SSH key is correctly installed
- Check SSH key file permissions (600)
- Ensure authorized_keys file permissions (600)
- Test SSH connection manually

#### 3. **Remote Path Issues**
**Symptoms**: `mkdir failed: Permission denied`
**Solutions**:
- Verify remote directory exists
- Check directory permissions
- Ensure backup user owns the directory

#### 4. **Bandwidth Issues**
**Symptoms**: Slow sync or timeouts
**Solutions**:
- Enable compression: `"enable_compression": true`
- Set bandwidth limit: `"bandwidth_limit": 1024`
- Adjust connection timeout: `"connection_timeout": 60`

### Diagnostic Commands

```bash
# Test SSH connection manually
ssh -i ~/.ssh/backup_key backup-user@backup-server.com

# Test rsync manually
rsync -avz --dry-run .backup/ backup-user@backup-server.com:/home/backup-user/ransomware-backups/

# Check remote directory
ssh backup-user@backup-server.com 'ls -la /home/backup-user/ransomware-backups/'

# Verify SSH key permissions
ls -la ~/.ssh/backup_key
```

### Configuration Validation

```bash
# Validate SSH key
ssh-keygen -y -f ~/.ssh/backup_key

# Test rsync options
rsync --help | grep -E "(avz|delete|partial|progress)"

# Check network connectivity
ping backup-server.com
telnet backup-server.com 22
```

## Best Practices

### 1. **Security**
- Use dedicated SSH keys for backup operations
- Disable password authentication on remote server
- Regularly rotate SSH keys
- Monitor SSH access logs

### 2. **Performance**
- Enable compression for slow connections
- Use bandwidth limiting during business hours
- Schedule intensive syncs during off-peak times
- Monitor sync duration and optimize intervals

### 3. **Reliability**
- Configure appropriate retry settings
- Monitor sync success rates
- Set up alerting for failed syncs
- Test restore procedures regularly

### 4. **Maintenance**
- Regularly check remote disk space
- Clean up old backups based on retention policy
- Monitor network connectivity
- Update SSH keys before expiration

## Integration Examples

### Manual Sync Trigger

```go
// Trigger immediate remote sync
err := backupManager.RemoteSyncBackups(context.Background())
if err != nil {
    log.Printf("Remote sync failed: %v", err)
}
```

### Status Monitoring

```go
// Get remote sync status
status := backupManager.GetRemoteSyncStatus()
fmt.Printf("Remote sync status: %s\n", status["status"])
fmt.Printf("Last sync: %s\n", status["last_sync"])
```

### Configuration Updates

```go
// Enable remote backup
config.Backup.RemoteBackup.Enabled = true
config.Backup.RemoteBackup.Host = "new-server.com"
config.SaveConfig("config.json")
```

The remote backup system provides enterprise-grade off-site backup capabilities with secure, efficient, and reliable synchronization to Ubuntu servers using industry-standard Rsync over SSH protocols. 