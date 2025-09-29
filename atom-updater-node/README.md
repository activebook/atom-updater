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

## Installation

```bash
npm install atom-updater
```

> **Note**: This package requires the `atom-updater` Go executable to be available on your system. You can download it from the [releases page](https://github.com/activebook/atom-updater/releases).

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

Perform an atomic update.

```typescript
const result = await updater.update({
  pid: 12345,
  currentPath: '/path/to/current',
  newPath: '/path/to/new',
  appName: 'optional-app-name' // Optional
});
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
  success: boolean;      // Whether the update was successful
  version?: string;      // Version of the updater used
  logPath?: string;      // Path to the log file
  error?: string;        // Error message if update failed
  launchedPid?: number;  // Process ID of the launched application
}
```

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
        console.log('Update successful!');

        // The updater will automatically launch the new version
        // and exit the current process
        app.quit();
      } else {
        console.error('Update failed:', result.error);
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
      console.log('Update completed successfully!');
      console.log(`Check the log file: ${result.logPath}`);
    } else {
      console.error('Update failed:', result.error);
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

- **Windows**: `amd64`, `386`
- **macOS**: `amd64`, `arm64` (Apple Silicon) - **optimized for .app bundles**
- **Linux**: `amd64`, `arm64`, `386`

## Requirements

- Node.js 14.0.0 or later
- The `atom-updater` Go executable must be available on your system

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

## License

MIT License - see LICENSE file for details.

## Contributing

Contributions are welcome! Please see the main [atom-updater repository](https://github.com/activebook/atom-updater) for contribution guidelines.

## Support

For issues and questions:

- [GitHub Issues](https://github.com/activebook/atom-updater/issues)
- [Discussions](https://github.com/activebook/atom-updater/discussions)