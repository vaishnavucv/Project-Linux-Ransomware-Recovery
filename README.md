# Linux Ransomware Recovery PoC

A comprehensive Proof of Concept (PoC) for Linux ransomware recovery using Go, featuring real-time file monitoring, automatic backup, suspicious pattern detection, and automated rollback capabilities.

## Features

### Core Components

- **Agent** (`agent.go`): Real-time file system monitoring using `fsnotify`
- **Detector** (`detector.go`): Suspicious pattern detection with customizable triggers
- **Rollback** (`rollback.go`): Automated file restoration from backups
- **Backup System**: Automatic file backup with checksums and integrity verification
- **Logging**: Structured JSON logging with multiple severity levels
- **Configuration**: Flexible JSON-based configuration system

### Key Capabilities

※ **Real-time Monitoring**: Watches `./demo-folder/` for all file system events  
※ **Automatic Backup**: Creates backups in `.backup/` on file creation/modification  
※ **Pattern Detection**: Detects suspicious extensions (`.locked`, `.encrypted`, etc.)  
※ **Content Analysis**: Scans file content for ransomware-related keywords  
※ **Threshold-based Triggers**: Configurable detection thresholds and time windows  
※ **Automatic Rollback**: Restores files when suspicious activity is detected  
※ **File Quarantine**: Isolates infected files before restoration  
※ **Parallel Processing**: Multi-threaded operations for performance  
※ **Graceful Shutdown**: Proper cleanup on SIGINT/SIGTERM  
※ **Security**: Path traversal protection and input validation  

## Architecture

```
┌─────────────┐    ┌──────────────┐    ┌─────────────┐
│   Agent     │───▶│   Detector   │───▶│  Rollback   │
│ (fsnotify)  │    │ (patterns)   │    │ (restore)   │
└─────────────┘    └──────────────┘    └─────────────┘
       │                    │                   │
       ▼                    ▼                   ▼
┌─────────────┐    ┌──────────────┐    ┌─────────────┐
│   Backup    │    │   Logger     │    │   Config    │
│ (checksums) │    │ (journal)    │    │ (settings)  │
└─────────────┘    └──────────────┘    └─────────────┘
```

## Quick Start

### Prerequisites

- Go 1.21 or later
- Linux environment (tested on Ubuntu 22.04)
- Write permissions for backup and log directories

### Installation

1. Clone the repository:
```bash
git clone https://github.com/vaishnavucv/Project-Linux-Ransomware-Recovery
cd Project-Linux-Ransomware-Recovery
```

2. Install dependencies:
```bash
go mod tidy
```

3. Build the project:
```bash
go build -o ransomware-recovery .
```

**Note**: The `ransomware-recovery` executable is excluded from git tracking via `.gitignore`. Users need to build it locally after cloning.

### Running the System

1. **Basic Usage**:
```bash
./ransomware-recovery
```

2. **With Demo Script**:
```bash
./demo.sh
```

The system will:
- Monitor `./demo-folder/` for file changes
- Create backups in `./.backup/`
- Log activities to `./logs/journal.log`
- Automatically trigger rollback when suspicious activity is detected

## Configuration

The system uses a JSON configuration file (`config.json`) with the following structure:

```json
{
  "monitor_path": "./demo-folder",
  "backup_path": "./.backup",
  "log_path": "./logs",
  "detection": {
    "suspicious_extensions": [".locked", ".encrypted", ".crypto", ".crypt", ".enc"],
    "threshold": 3,
    "time_window": 60,
    "real_time_detection": true,
    "content_patterns": ["RANSOMWARE", "DECRYPT", "BITCOIN", "PAYMENT"]
  },
  "backup": {
    "auto_backup": true,
    "retention_days": 30,
    "enable_checksums": true,
    "compression": false
  },
  "system": {
    "worker_count": 4,
    "buffer_size": 100,
    "operation_timeout": 30
  }
}
```

### Configuration Options

#### Detection Settings
- `suspicious_extensions`: File extensions that trigger alerts
- `threshold`: Number of suspicious events to trigger rollback
- `time_window`: Time window (seconds) for counting events
- `real_time_detection`: Enable real-time pattern detection
- `content_patterns`: Keywords to detect in file content

#### Backup Settings
- `auto_backup`: Enable automatic backup on file changes
- `retention_days`: How long to keep backup files
- `enable_checksums`: Enable SHA-256 checksums for integrity
- `compression`: Enable backup compression (future feature)

#### System Settings
- `worker_count`: Number of parallel processing workers
- `buffer_size`: Event channel buffer size
- `operation_timeout`: Timeout for individual operations

## Usage Examples

### Manual Operations

The system provides APIs for manual operations:

```go
// Manual rollback
result, err := system.ManualRollback()

// Force detection analysis
detection := system.ForceDetection()

// Get system statistics
stats := system.GetSystemStats()
```

### Testing Ransomware Simulation

1. Create test files in `demo-folder/`
2. Run the system: `./ransomware-recovery`
3. Simulate ransomware by creating `.locked` files:
```bash
echo "RANSOMWARE" > demo-folder/test.txt.locked
echo "DECRYPT" > demo-folder/document.pdf.locked
echo "BITCOIN" > demo-folder/image.jpg.locked
```
4. Watch the system automatically detect and rollback

## Logging

The system uses structured JSON logging with the following levels:

- **INFO**: Normal operations and status updates
- **WARN**: Suspicious activity and rollback triggers
- **ERROR**: System errors and failures
- **DEBUG**: Detailed debugging information

Log entries include:
- Timestamp
- Component (agent, detector, rollback, backup)
- Event details
- File paths
- Operation results

Example log entry:
```json
{
  "component": "detector",
  "event_type": "CREATE",
  "file_path": "./demo-folder/test.txt.locked",
  "level": "warning",
  "msg": "Suspicious pattern detected",
  "pattern": "suspicious_extension:.locked",
  "severity": "high",
  "time": "2024-01-15 10:30:45"
}
```

## Security Features

### Path Traversal Protection
- Validates all file paths to prevent directory traversal attacks
- Ensures operations stay within designated directories

### Input Validation
- Sanitizes file paths and content
- Validates configuration parameters
- Prevents injection attacks

### Access Control
- Restricts operations to monitored directories
- Validates file permissions before operations
- Secure backup and quarantine handling

## Performance Considerations

### Concurrent Processing
- Multi-threaded file operations
- Parallel backup and restore operations
- Non-blocking event processing

### Resource Management
- Configurable worker pools
- Memory-efficient file streaming
- Automatic cleanup of old events and backups

### Scalability
- Handles large directory structures
- Efficient file system monitoring
- Configurable buffer sizes and timeouts

## Limitations

- **File Size**: Large files may impact performance
- **Network Drives**: Not tested with network-mounted filesystems
- **Encryption**: Cannot decrypt already encrypted files
- **Real-time**: Small delay between detection and rollback
- **Storage**: Requires adequate disk space for backups

## Development

### Project Structure
```
├── main.go                 # Main application entry point
├── agent.go               # File system monitoring
├── detector.go            # Pattern detection
├── rollback.go            # File restoration
├── pkg/
│   ├── backup/           # Backup management
│   ├── config/           # Configuration handling
│   └── logger/           # Logging system
├── demo.sh               # Demonstration script
├── go.mod                # Go module definition
├── .gitignore            # Git ignore rules
└── README.md             # This file
```

### Git Ignore Rules

The `.gitignore` file excludes the following from version control:

- **Built executable**: `ransomware-recovery` (users build this locally)
- **Runtime directories**: `logs/`, `.backup/`, `demo-folder/`
- **Configuration files**: `config.json` (generated at runtime)
- **Temporary files**: `*.log`, `*.tmp`, `*.pid`
- **IDE files**: `.vscode/`, `.idea/`, `*.swp`
- **OS files**: `.DS_Store`, `Thumbs.db`
- **Build artifacts**: `*.test`, `*.out`, `*.prof`

### Adding New Detection Patterns

1. Update `config.json` with new patterns:
```json
{
  "detection": {
    "suspicious_extensions": [".locked", ".encrypted", ".your_extension"],
    "content_patterns": ["RANSOMWARE", "YOUR_PATTERN"]
  }
}
```

2. Implement custom detection logic in `detector.go`:
```go
func (d *Detector) customDetection(event FileEvent) []SuspiciousEvent {
    // Your custom detection logic here
}
```

### Testing

Run the demo script to test all functionality:
```bash
./demo.sh
```

The script will:
1. Build the project
2. Create test files
3. Start the system
4. Simulate ransomware activity
5. Verify automatic detection and rollback
6. Clean up resources

## Contributing

1. Fork the repository
2. Create a feature branch
3. Implement your changes
4. Add tests for new functionality
5. Update documentation
6. Submit a pull request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Disclaimer

This is a Proof of Concept for educational and research purposes. It should not be used in production environments without thorough security review and testing. The system provides basic ransomware detection and recovery capabilities but may not protect against all types of ransomware attacks.

## Support

For issues, questions, or contributions, please:
1. Check the existing issues
2. Create a new issue with detailed description
3. Provide system information and logs
4. Follow the contribution guidelines

---

**Note**: This PoC demonstrates core concepts of ransomware detection and recovery. In a production environment, additional security measures, comprehensive testing, and integration with enterprise security systems would be required.
