import { UpdateConfig, UpdateResult, AtomUpdaterOptions, ExecutableNotFoundError, UpdateFailedError } from './types.js';
import { PlatformUtils } from './PlatformUtils.js';
import process from 'node:process';
import { spawn } from 'node:child_process';
import { join } from 'node:path';

/**
 * Main class for atom-updater Node.js wrapper
 * Provides a simple interface to use the atom-updater Go executable
 */
export class AtomUpdater {
  private executablePath: string;
  private options: Required<AtomUpdaterOptions>;

  constructor(options: AtomUpdaterOptions = {}) {
    this.options = {
      executablePath: options.executablePath || '',
      workingDirectory: options.workingDirectory || process.cwd(),
      verbose: options.verbose || false,
      logger: options.logger || ((msg: string) => {})
    };

    this.executablePath = this.options.executablePath || PlatformUtils.findExecutable();
  }

  /**
   * Get the version of the atom-updater executable
   */
  async getVersion(): Promise<string> {
    return new Promise((resolve, reject) => {
      try {
        const child = spawn(this.executablePath, ['--version'], {
          cwd: this.options.workingDirectory,
          stdio: ['pipe', 'pipe', 'pipe']
        });

        let stdout = '';
        let stderr = '';

        child.stdout?.on('data', (data: Buffer) => {
          stdout += data.toString();
        });

        child.stderr?.on('data', (data: Buffer) => {
          stderr += data.toString();
        });

        child.on('close', (code: number) => {
          if (code === 0) {
            resolve(stdout.trim());
          } else {
            reject(new Error(`Failed to get version: ${stderr || stdout}`));
          }
        });

        child.on('error', (error: Error) => {
          reject(new ExecutableNotFoundError(this.executablePath));
        });
      } catch (error) {
        reject(error);
      }
    });
  }

  /**
   * Perform an atomic update using the atom-updater executable
   * This method starts the updater as a detached process and exits immediately
   */
  async update(config: UpdateConfig): Promise<UpdateResult> {
    // Validate inputs
    PlatformUtils.validateDirectory(config.currentPath, 'Current path');
    PlatformUtils.validateDirectory(config.newPath, 'New path');
    PlatformUtils.validateNotAppBundle(config.currentPath, 'Current path');
    PlatformUtils.validateNotAppBundle(config.newPath, 'New path');

    if (this.options.verbose) {
      this.options.logger(`Starting update with atom-updater:`);
      this.options.logger(`  PID: ${config.pid}`);
      this.options.logger(`  Current: ${config.currentPath}`);
      this.options.logger(`  New: ${config.newPath}`);
      if (config.appName) {
        this.options.logger(`  App Name: ${config.appName}`);
      }
    }

    try {
      // Build command arguments
      const args = [
        config.pid.toString(),
        config.currentPath,
        config.newPath
      ];

      if (config.appName) {
        args.push('--app-name', config.appName);
      }

      if (this.options.verbose) {
        this.options.logger(`Executing: ${this.executablePath} ${args.join(' ')}`);
      }

      // Start atom-updater as detached process
      // atom-updater will wait for this process to exit, then perform the update
      const child = spawn(this.executablePath, args, {
        cwd: this.options.workingDirectory,
        // stdio: 'inherit', // Show progress to user, this would cause error, because the user app already quit!
        stdio: 'ignore',   // No output
        detached: true    // Run independently
      });

      // Unref the child process so it can run independently
      child.unref();

      if (this.options.verbose) {
        this.options.logger(`Started atom-updater with PID: ${child.pid}`);
        this.options.logger(`Log file will be available at: ${this.getLogPath()}`);
      }

      // Return immediately - atom-updater will handle the rest
      const result: UpdateResult = {
        success: true,
        logPath: this.getLogPath(),
        launchedPid: child.pid
      };

      return result;

    } catch (error) {
      throw new UpdateFailedError(`Failed to start update process: ${error}`, error as Error);
    }
  }

  /**
   * Get the expected log file path
   */
  private getLogPath(): string {
    return join(this.options.workingDirectory, 'atom-updater.log');
  }

  /**
   * Check if the atom-updater executable is available
   */
  async isAvailable(): Promise<boolean> {
    try {
      await this.getVersion();
      return true;
    } catch {
      return false;
    }
  }

  /**
   * Get the path to the executable being used
   */
  getExecutablePath(): string {
    return this.executablePath;
  }

  /**
   * Update the executable path
   */
  setExecutablePath(path: string): void {
    this.executablePath = path;
  }
}