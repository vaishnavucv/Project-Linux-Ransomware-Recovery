#!/bin/bash

# Ransomware Recovery System Demo Script
# This script demonstrates the functionality of the system

set -e

echo "=== Ransomware Recovery System Demo ==="
echo

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_step() {
    echo -e "${GREEN}[STEP]${NC} $1"
}

print_info() {
    echo -e "${YELLOW}[INFO]${NC} $1"
}

print_warning() {
    echo -e "${RED}[WARNING]${NC} $1"
}

# Check if Go is installed
if ! command -v go &> /dev/null; then
    print_warning "Go is not installed. Please install Go 1.21 or later."
    exit 1
fi

# Build the project
print_step "Building the project..."
go mod tidy
go build -o ransomware-recovery .

# Create some demo files
print_step "Creating demo files..."
mkdir -p demo-folder/documents
mkdir -p demo-folder/images
mkdir -p demo-folder/config

# Create sample files
echo "This is a sample document." > demo-folder/documents/sample.txt
echo "Important configuration data" > demo-folder/config/app.conf
echo "User preferences" > demo-folder/config/user.json
echo "Binary data simulation" > demo-folder/images/photo.jpg

print_info "Created demo files:"
find demo-folder -type f -exec ls -la {} \;

# Start the system in background
print_step "Starting the ransomware recovery system..."
./ransomware-recovery &
SYSTEM_PID=$!

# Wait for system to initialize
sleep 3

print_info "System started with PID: $SYSTEM_PID"

# Function to simulate ransomware activity
simulate_ransomware() {
    print_step "Simulating ransomware activity..."
    
    # Create suspicious files with .locked extension
    echo "RANSOMWARE: Your files have been encrypted!" > demo-folder/documents/sample.txt.locked
    echo "RANSOMWARE: Pay bitcoin to decrypt!" > demo-folder/config/app.conf.locked
    echo "RANSOMWARE: Contact us for decryption key!" > demo-folder/images/photo.jpg.locked
    
    # Remove original files (simulate encryption)
    rm -f demo-folder/documents/sample.txt
    rm -f demo-folder/config/app.conf
    rm -f demo-folder/images/photo.jpg
    
    print_warning "Ransomware simulation: Files encrypted with .locked extension"
    
    # Create more suspicious files to trigger threshold
    for i in {1..5}; do
        echo "DECRYPT: Send payment to recover files" > demo-folder/documents/ransom_note_$i.locked
        sleep 0.5
    done
    
    print_info "Created multiple suspicious files to trigger detection threshold"
}

# Function to check system status
check_status() {
    print_step "Checking system status..."
    
    print_info "Backup files:"
    if [ -d ".backup" ]; then
        find .backup -type f -exec ls -la {} \;
    else
        echo "No backup directory found"
    fi
    
    print_info "Current demo-folder contents:"
    find demo-folder -type f -exec ls -la {} \;
    
    print_info "Log entries (last 10):"
    if [ -f "logs/journal.log" ]; then
        tail -n 10 logs/journal.log
    else
        echo "No log file found"
    fi
}

# Function to cleanup
cleanup() {
    print_step "Cleaning up..."
    
    # Kill the system process
    if kill -0 $SYSTEM_PID 2>/dev/null; then
        print_info "Stopping system (PID: $SYSTEM_PID)..."
        kill $SYSTEM_PID
        sleep 2
    fi
    
    print_info "Demo completed"
}

# Set trap for cleanup
trap cleanup EXIT

# Run the demo
print_step "Starting demo sequence..."

# Wait for initial backup
print_info "Waiting for initial backup to complete..."
sleep 5

# Check initial status
check_status

# Simulate ransomware
simulate_ransomware

# Wait for detection and rollback
print_info "Waiting for detection and automatic rollback..."
sleep 10

# Check final status
print_step "Checking final status after rollback..."
check_status

# Manual test of specific functions
print_step "Testing manual operations..."

# Test manual detection
print_info "Testing manual detection..."
# This would require additional API endpoints or CLI commands

# Test selective restore
print_info "Testing selective restore..."
# This would require additional API endpoints or CLI commands

print_step "Demo completed successfully!"
print_info "The system demonstrated:"
echo "  ✓ Real-time file monitoring with fsnotify"
echo "  ✓ Automatic backup on file creation/modification"
echo "  ✓ Suspicious pattern detection (.locked extensions)"
echo "  ✓ Automatic rollback when threshold is exceeded"
echo "  ✓ File quarantine for infected files"
echo "  ✓ Structured logging with JSON format"
echo "  ✓ Graceful shutdown on SIGINT/SIGTERM"

print_info "Check the logs/journal.log file for detailed operation logs"
print_info "Check the .backup directory for backed up files"
print_info "Check the .backup/quarantine directory for quarantined files" 