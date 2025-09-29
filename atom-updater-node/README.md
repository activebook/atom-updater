# atom-updater-node

A TypeScript Node.js wrapper for the [atom-updater](https://github.com/activebook/atom-updater) Go executable. This package provides atomic directory replacement with rollback capabilities for Node.js and Electron applications.

## Features

- 🔄 **Atomic Directory Updates**: All-or-nothing directory replacement with automatic rollback
- 🍎 **macOS .app Bundle Support**: Specialized handling for directories containing .app bundles
- 📊 **Comprehensive Logging**: Real-time console output + persistent log file
- 🛡️ **Robust Error Handling**: Graceful failure handling with safe rollback
- 🚀 **Smart Application Launching**: Auto-detects and launches the correct application
- 🌐 **Cross-Platform**: Works on Windows, macOS, and Linux
- 📦 **Easy Integration**: Simple TypeScript API for Node.js and Electron apps
- 🔗 **Self-Contained**: Bundled binaries eliminate external dependencies

## How It Works

This package uses a **bundled binary approach** where platform-specific `atom-updater` executables are included directly in the npm package. When you install the package, you get:

- The TypeScript wrapper code
- Pre-compiled `atom-updater` binaries for your platform
- Automatic binary detection and selection

The wrapper automatically finds and uses the correct binary for your platform and architecture, eliminating the need for separate downloads or installations.

## Installation

```bash
npm install atom-updater
```

The package includes platform-specific `atom-updater` binaries, so no additional downloads are required. The wrapper automatically detects and uses the bundled binary for your platform and architecture.

## Quick Start

```typescript
import { AtomUpdater } from 'atom-updater';

const updater = new AtomUpdater();

// Check version
const version = await updater.getVersion();
console.log(`Atom updater version: ${version}`);

// Perform update
const result = await updater.update({
  pid: process.pid,
  currentPath: '/path/to/current/app',
  newPath: '/path/to/new/app/version'
});

if (result.success) {
  console.log('Update completed successfully!');
  console.log(`Log file: ${result.logPath}`);
} else {
  console.error('Update failed:', result.error);
}
```

## API Reference

### AtomUpdater Class

#### Constructor

```typescript
constructor(options?: AtomUpdaterOptions)
```

**Options:**
- `executablePath?: string` - Custom path to the atom-updater executable
- `workingDirectory?: string` - Working directory for the update process
- `verbose?: boolean` - Enable verbose logging
- `logger?: (message: string) => void` - Custom logger function

#### Methods

##### `getVersion(): Promise<string>`

Get the version of the atom-updater executable.

```typescript
const version = await updater.getVersion();
```

##### `update(config: UpdateConfig): Promise<UpdateResult>`

Perform an atomic update by starting the atom-updater process as a detached background process. **This method returns immediately** - the actual update happens asynchronously after the calling process exits.

```typescript
const result = await updater.update({
  pid: 12345,
  currentPath: '/path/to/current',
  newPath: '/path/to/new',
  appName: 'optional-app-name' // Optional
});

// The method returns immediately with success: true
// The calling application should exit immediately after this call
// atom-updater will wait for the exit, perform the update, and launch the new version
```

##### `isAvailable(): Promise<boolean>`

Check if the atom-updater executable is available.

```typescript
const available = await updater.isAvailable();
```

##### `getExecutablePath(): string`

Get the path to the executable being used.

### Types

#### UpdateConfig

```typescript
interface UpdateConfig {
  pid: number;           // Process ID to wait for exit
  currentPath: string;   // Path to current application directory
  newPath: string;       // Path to new application directory
  appName?: string;      // Optional specific executable to launch
  timeout?: number;      // Optional timeout for the update process
}
```

#### UpdateResult

```typescript
interface UpdateResult {
  success: boolean;      // Whether the update process was started successfully
  logPath?: string;      // Path to the log file (atom-updater.log)
  launchedPid?: number;  // Process ID of the atom-updater process
  // Note: version and error fields are not used in the bundled binary approach
}
```

## Update Process Flow

With the bundled binary approach, the update process follows this sequence:

1. **Your app calls** `updater.update(config)` with paths and PID
2. **Wrapper starts** `atom-updater` as a detached background process
3. **Method returns immediately** with `success: true`
4. **Your app exits** (via `app.quit()` in Electron or `process.exit()` in Node.js)
5. **atom-updater waits** for your app process to fully exit
6. **atom-updater performs** the atomic directory replacement
7. **atom-updater launches** the new version of your application
8. **atom-updater exits**

This approach ensures safe, atomic updates without the chicken-and-egg problem of waiting for completion.

## Integration Examples

### Electron Application

```typescript
import { AtomUpdater } from 'atom-updater';
import { app } from 'electron';

class AppUpdater {
  private updater = new AtomUpdater({ verbose: true });

  async checkForUpdates() {
    try {
      const available = await this.updater.isAvailable();
      if (!available) {
        console.error('atom-updater executable not found');
        return;
      }

      const version = await this.updater.getVersion();
      console.log(`Using atom-updater version: ${version}`);
    } catch (error) {
      console.error('Failed to check updater:', error);
    }
  }

  async performUpdate(newVersionPath: string) {
    try {
      const result = await this.updater.update({
        pid: process.pid,
        currentPath: __dirname, // Electron app directory
        newPath: newVersionPath
      });

      if (result.success) {
        console.log('Update initiated successfully!');
        console.log(`Log file: ${result.logPath}`);

        // Exit immediately - atom-updater will handle the rest
        // It will wait for this process to exit, perform the update,
        // and launch the new version automatically
        app.quit();
      } else {
        console.error('Update failed to start:', result.error);
      }
    } catch (error) {
      console.error('Update error:', error);
    }
  }
}
```

### Node.js Application

```typescript
import { update, getVersion } from 'atom-updater';

async function performUpdate() {
  try {
    // Check if updater is available
    const version = await getVersion();
    console.log(`Using atom-updater ${version}`);

    // Perform update
    const result = await update({
      pid: process.pid,
      currentPath: process.cwd(),
      newPath: './updates/new-version'
    });

    if (result.success) {
      console.log('Update initiated successfully!');
      console.log(`Log file: ${result.logPath}`);

      // Exit immediately - atom-updater will handle the rest
      process.exit(0);
    } else {
      console.error('Update failed to start:', result.error);
    }
  } catch (error) {
    console.error('Update process failed:', error);
  }
}
```

### Custom Executable Path

```typescript
import { AtomUpdater } from 'atom-updater';

const updater = new AtomUpdater({
  executablePath: '/custom/path/to/atom-updater',
  verbose: true
});
```

## Platform Support

- **Windows**: `amd64`, `386` - bundled binaries included
- **macOS**: `amd64`, `arm64` (Apple Silicon) - bundled binaries included, **optimized for .app bundles**
- **Linux**: `amd64`, `arm64`, `386` - bundled binaries included

The package includes platform-specific binaries, so no additional downloads are required.

## Requirements

- Node.js 14.0.0 or later
- No additional dependencies - the `atom-updater` binary is bundled with the package

## Error Handling

The package provides specific error classes for different failure scenarios:

```typescript
import {
  AtomUpdaterError,
  ExecutableNotFoundError,
  UpdateFailedError
} from 'atom-updater';

try {
  await updater.update(config);
} catch (error) {
  if (error instanceof ExecutableNotFoundError) {
    console.error('atom-updater executable not found');
  } else if (error instanceof UpdateFailedError) {
    console.error('Update failed:', error.message);
  } else {
    console.error('Unexpected error:', error);
  }
}
```

## Logging

The atom-updater executable provides comprehensive logging:

- **Console Output**: Real-time progress during updates
- **File Logging**: Persistent log at `./atom-updater.log` (auto-cleared on startup)
- **Debug Information**: Timestamps and source file names for troubleshooting

The log file is created in the same directory as the `atom-updater` executable.
