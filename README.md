# Atom-Updater

A robust, cross-platform Go application designed to safely update application directories that cannot update themselves while running. It provides atomic directory replacement with advanced `.app` bundle support and comprehensive rollback capabilities.

## Features

- **🔄 Atomic Directory Replacement**: All-or-nothing directory replacement with automatic rollback
- **🍎 macOS `.app` Bundle Support**: Specialized handling for directories containing `.app` bundles
- **📊 Dual Logging**: Real-time console output + persistent log file (`atom-updater.log`)
- **🛡️ Robust Error Handling**: Comprehensive logging and graceful failure handling with safe rollback
- **🚀 Smart Application Launching**: Auto-detects and launches the correct application from directories
- **🌐 Cross-Platform**: Works on Windows, macOS, and Linux
- **⚡ Simple Integration**: Clean CLI interface with easy-to-parse version output
- **🔒 Safe Operations**: Process monitoring and file validation with resilient PID handling

## Installation

### Download Pre-built Binaries

Download the latest release for your platform from the [releases page](https://github.com/activebook/atom-updater/releases).

## Usage

### Version Information

```bash
./atom-updater --version
# Output: v2.0.0
```

### Update Application

```bash
./atom-updater <pid> <current_dir> <new_dir> [--app-name <name>]
```

**Parameters:**

- `<pid>`: Process ID to wait for exit
- `<current_dir>`: Path to current application directory (must be directory)
- `<new_dir>`: Path to new application directory (must be directory)
- `--app-name <name>`: Optional specific executable to launch (for directories)

**⚠️ Restrictions:**
- Both `<current_dir>` and `<new_dir>` **MUST** be directories
- Single files (like `.exe`) are **NOT** allowed
- `.app` bundles are **NOT** allowed as direct arguments

**Examples:**

```bash
# macOS directory containing .app bundles
./atom-updater 12345 ./test/myapp ./test/updates/macapp

# Windows directory with specific exe
./atom-updater 12345 ./MyApp/ ./updates/MyApp/ --app-name app.exe

# Generic application directory
./atom-updater 6789 /opt/myapp /tmp/new/myapp
```

### Help

```bash
./atom-updater --help
```

## How It Works

### Directory-Based Update Process

1. **Wait**: Monitors the target process PID until it exits (graceful handling if PID not found)
2. **Backup**: Creates backup directory and moves current files to it
3. **Replace**: Copies new directory contents with full fidelity
4. **Atomic Move**: For `.app` bundles: `.app` → `.app.new` → `.app` (prevents permission issues)
5. **Smart Launch**: Auto-detects and launches the correct application:
   - **macOS**: Finds first `.app` bundle in directory
   - **Windows**: Finds first `.exe` file
   - **Linux**: Finds first executable
6. **Cleanup**: Removes backup directory after successful launch
7. **Logging**: Writes to both console and `atom-updater.log` file

### Special `.app` Bundle Handling

For directories containing `.app` bundles, the updater uses Apple's recommended approach:
- **Atomic replacement**: `.app` → `.app.new` → `.app` pattern
- **macOS-optimized**: Preserves all metadata and code signatures
- **Permission-safe**: Avoids modifying existing `.app` bundle contents
- **Rollback-capable**: Can restore previous version if update fails

If any step fails, the updater automatically rolls back to the previous version.

## Integration Example

### Target Application (Directory-Based Update)

```javascript
const { spawn } = require('child_process');
const path = require('path');
const fs = require('fs');

// Check updater version
function checkUpdaterVersion() {
  return new Promise((resolve, reject) => {
    const updaterPath = path.join(process.cwd(), 'atom-updater');
    const child = spawn(updaterPath, ['--version'], { stdio: 'pipe' });

    let version = '';
    child.stdout.on('data', (data) => {
      version += data.toString();
    });

    child.on('close', (code) => {
      if (code === 0) {
        resolve(version.trim());
      } else {
        reject(new Error(`Updater check failed with code ${code}`));
      }
    });
  });
}

// Update application directory
async function updateApplication(newVersionDir) {
  const currentPid = process.pid;
  const currentDir = path.dirname(process.execPath);
  const updaterPath = path.join(process.cwd(), 'atom-updater');

  // Validate that new version directory exists
  if (!fs.existsSync(newVersionDir)) {
    throw new Error(`New version directory does not exist: ${newVersionDir}`);
  }

  // Check log file for debugging
  const logPath = path.join(process.cwd(), 'atom-updater.log');
  console.log(`Update log will be available at: ${logPath}`);

  // Exit current app and let updater handle the rest
  spawn(updaterPath, [currentPid.toString(), currentDir, newVersionDir], {
    detached: true,
    stdio: 'inherit'
  }).unref();

  app.quit();
}

// Example usage
async function performUpdate() {
  try {
    const version = await checkUpdaterVersion();
    console.log(`Updater version: ${version}`);

    const newVersionDir = './updates/v2.0.0';
    await updateApplication(newVersionDir);
  } catch (error) {
    console.error('Update failed:', error);
  }
}
```

### Supported Application Types

- **macOS directories containing .app bundles** ✨ (primary feature)
- **macOS directories with executables**
- **Windows directories with executables**
- **Linux directories with executables**

### Logging

The updater provides comprehensive logging:

- **Console Output**: Real-time progress during updates
- **File Logging**: Persistent log at `./atom-updater.log` (auto-cleared on startup)
- **Debug Information**: Timestamps and source file names for troubleshooting

**Log file location**: Same directory as the `atom-updater` executable

### Supported Platforms

- **Windows**: `amd64`, `386`
- **macOS**: `amd64`, `arm64` (Apple Silicon) - **optimized for .app bundles**
- **Linux**: `amd64`, `arm64`, `386`

## Development

### Prerequisites

- Go 1.21 or later
- Git

### Recent Changes (v2.0.0)

- **Directory-only updates**: Now exclusively handles application directories
- **`.app` bundle optimization**: Perfect macOS support with metadata preservation
- **Dual logging system**: Console + file logging with automatic log rotation
- **Smart application launching**: Auto-detects correct executable to launch
- **Resilient process handling**: Graceful handling of missing PIDs

## Configuration

The application requires no configuration files. All parameters are passed via command-line arguments.
