/**
 * TypeScript interfaces and types for atom-updater Node.js wrapper
 */

export interface UpdateConfig {
  /** Process ID to wait for exit */
  pid: number;
  /** Path to current application directory (must be directory) */
  currentAppDir: string;
  /** Path to new application directory (must be directory) */
  newAppDir: string;
  /** Optional specific executable to launch (for directories) */
  appName?: string;
  /** Optional external directory to copy the atom-updater binary (for self-update scenarios) */
  binDir?: string;
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

export type Platform = 'darwin' | 'linux' | 'win32';
export type Architecture = 'x64' | 'arm64' | 'ia32';

export interface PlatformInfo {
  platform: Platform;
  arch: Architecture;
  executableName: string;
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
