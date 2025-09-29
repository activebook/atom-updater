import { Platform, Architecture, PlatformInfo, ExecutableNotFoundError } from './types.js';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import process from 'node:process';
import { fileURLToPath } from 'node:url';

/**
 * Utility class for platform-specific operations
 */
export class PlatformUtils {
  /**
   * Get current platform information
   */
  static getCurrentPlatform(): Platform {
    const currentPlatform = os.platform();
    if (currentPlatform === 'darwin') return 'darwin';
    if (currentPlatform === 'linux') return 'linux';
    if (currentPlatform === 'win32') return 'windows';
    throw new Error(`Unsupported platform: ${currentPlatform}`);
  }

  /**
   * Get current architecture information
   */
  static getCurrentArchitecture(): Architecture {
    const currentArch = process.arch;
    if (currentArch === 'x64') return 'x64';
    if (currentArch === 'arm64') return 'arm64';
    if (currentArch === 'ia32') return 'ia32';
    throw new Error(`Unsupported architecture: ${currentArch}`);
  }

  /**
   * Get platform-specific information
   */
  static getPlatformInfo(): PlatformInfo {
    const platform = this.getCurrentPlatform();
    const arch = this.getCurrentArchitecture();

    let executableName: string;
    let archiveExt: string;

    switch (platform) {
      case 'windows':
        executableName = 'atom-updater.exe';
        archiveExt = 'zip';
        break;
      case 'darwin':
        executableName = 'atom-updater';
        archiveExt = 'tar.gz';
        break;
      case 'linux':
        executableName = 'atom-updater';
        archiveExt = 'tar.gz';
        break;
      default:
        throw new Error(`Unsupported platform: ${platform}`);
    }

    return {
      platform,
      arch,
      executableName,
      archiveExt
    };
  }

  /**
   * Find the atom-updater executable in various locations
   */
  static findExecutable(customPath?: string): string {
    // If custom path is provided, use it
    if (customPath) {
      if (!fs.existsSync(customPath)) {
        throw new ExecutableNotFoundError(customPath);
      }
      return customPath;
    }

    // 1. Try bundled binary first (self-contained)
    const bundledPath = this.getBundledExecutablePath();
    if (bundledPath) {
      return bundledPath;
    }

    // 2. Fall back to system-installed (legacy support)
    const platformInfo = this.getPlatformInfo();
    const executableName = platformInfo.executableName;

    // Get current module directory for ES modules
    const currentDir = path.dirname(fileURLToPath(import.meta.url));

    // Search locations in order of preference
    const searchPaths = [
      // Current working directory
      path.join(process.cwd(), executableName),
      // Relative to the Node.js module
      path.join(currentDir, '..', '..', executableName),
      // Common installation paths
      path.join(process.cwd(), 'bin', executableName),
      path.join(process.cwd(), 'dist', executableName)
    ];

    for (const searchPath of searchPaths) {
      if (fs.existsSync(searchPath)) {
        return searchPath;
      }
    }

    throw new ExecutableNotFoundError();
  }

  /**
   * Get the path to the bundled executable for the current platform
   */
  private static getBundledExecutablePath(): string | null {
    const platformInfo = this.getPlatformInfo();
    const platform = platformInfo.platform;
    const arch = platformInfo.arch;
    const executableName = platformInfo.executableName;

    // Get current module directory for ES modules
    const currentDir = path.dirname(fileURLToPath(import.meta.url));

    // Path to bundled binary: bin/{platform}/{arch}/{executableName}
    const bundledPath = path.join(currentDir, '..', 'bin', platform, arch, executableName);

    if (fs.existsSync(bundledPath)) {
      return bundledPath;
    }

    return null; // Bundled binary not available
  }

  /**
   * Check if the current platform supports the given feature
   */
  static platformSupports(feature: 'app-bundle' | 'windows-exe' | 'linux-executable'): boolean {
    const platform = this.getCurrentPlatform();

    switch (feature) {
      case 'app-bundle':
        return platform === 'darwin';
      case 'windows-exe':
        return platform === 'windows';
      case 'linux-executable':
        return platform === 'linux';
      default:
        return false;
    }
  }

  /**
   * Get platform-specific file extensions
   */
  static getExecutableExtensions(): string[] {
    const platform = this.getCurrentPlatform();

    switch (platform) {
      case 'windows':
        return ['.exe', '.com', '.bat', '.cmd'];
      case 'darwin':
      case 'linux':
        return ['']; // No extension for Unix executables
      default:
        return [];
    }
  }

  /**
   * Validate that a path is a directory (required by atom-updater)
   */
  static validateDirectory(filePath: string, name: string): void {
    if (!fs.existsSync(filePath)) {
      throw new Error(`${name} does not exist: ${filePath}`);
    }

    const stats = fs.statSync(filePath);
    if (!stats.isDirectory()) {
      throw new Error(`${name} must be a directory, not a file: ${filePath}`);
    }
  }

  /**
   * Validate that a path is not a .app bundle (not supported as direct argument)
   */
  static validateNotAppBundle(filePath: string, name: string): void {
    if (filePath.endsWith('.app')) {
      throw new Error(`${name} cannot be a .app bundle, must be a directory: ${filePath}`);
    }
  }
}