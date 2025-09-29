/**
 * TypeScript interfaces and types for atom-updater Node.js wrapper
 */

export interface UpdateConfig {
  /** Process ID to wait for exit */
  pid: number;
  /** Path to current application directory (must be directory) */
  currentPath: string;
  /** Path to new application directory (must be directory) */
  newPath: string;
  /** Optional specific executable to launch (for directories) */
  appName?: string;
  /** Optional timeout for the update process */
  timeout?: number;
}

export interface UpdateResult {
  /** Whether the update was successful */
  success: boolean;
  /** Version of the updater used */
  version?: string | undefined;
  /** Path to the log file */
  logPath?: string;
  /** Error message if update failed */
  error?: string;
  /** Process ID of the launched application */
  launchedPid?: number | undefined;
}

export interface DownloadOptions {
  /** URL to download the update from */
  url: string;
  /** Directory to download the update to */
  downloadPath: string;
  /** Optional filename for the downloaded file */
  filename?: string;
  /** Optional headers for the download request */
  headers?: Record<string, string>;
  /** Whether to overwrite existing files */
  overwrite?: boolean;
}

export interface AtomUpdaterOptions {
  /** Custom path to the atom-updater executable */
  executablePath?: string;
  /** Custom working directory */
  workingDirectory?: string;
  /** Enable verbose logging */
  verbose?: boolean;
  /** Custom logger function */
  logger?: (message: string) => void;
}

export type Platform = 'darwin' | 'linux' | 'windows';
export type Architecture = 'x64' | 'arm64' | 'ia32';

export interface PlatformInfo {
  platform: Platform;
  arch: Architecture;
  executableName: string;
  archiveExt: string;
}

/**
 * Custom error class for atom-updater related errors
 */
export class AtomUpdaterError extends Error {
  constructor(
    message: string,
    public code?: string,
    public cause?: Error
  ) {
    super(message);
    this.name = 'AtomUpdaterError';

    if (cause) {
      this.stack += '\nCaused by: ' + cause.stack;
    }
  }
}

/**
 * Error thrown when the atom-updater executable is not found
 */
export class ExecutableNotFoundError extends AtomUpdaterError {
  constructor(executablePath?: string) {
    super(
      `atom-updater executable not found${executablePath ? ` at path: ${executablePath}` : ''}`,
      'EXECUTABLE_NOT_FOUND'
    );
    this.name = 'ExecutableNotFoundError';
  }
}

/**
 * Error thrown when the update process fails
 */
export class UpdateFailedError extends AtomUpdaterError {
  constructor(message: string, cause?: Error) {
    super(message, 'UPDATE_FAILED', cause);
    this.name = 'UpdateFailedError';
  }
}

/**
 * Error thrown when download fails
 */
export class DownloadFailedError extends AtomUpdaterError {
  constructor(message: string, cause?: Error) {
    super(message, 'DOWNLOAD_FAILED', cause);
    this.name = 'DownloadFailedError';
  }
}