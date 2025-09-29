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

    return new Promise((resolve, reject) => {
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

        // Spawn the updater process
        const child = spawn(this.executablePath, args, {
          cwd: this.options.workingDirectory,
          stdio: 'inherit', // Inherit parent's stdio to show progress
          detached: false
        });

        let logPath = '';
        let launchedPid: number | undefined;

        child.on('close', (code: number) => {
          if (code === 0) {
            const result: UpdateResult = {
              success: true,
              version: undefined, // We could get this from getVersion() if needed
              logPath: this.getLogPath(),
              launchedPid
            };
            resolve(result);
          } else {
            reject(new UpdateFailedError(`Update process exited with code ${code}`));
          }
        });

        child.on('error', (error: Error) => {
          reject(new UpdateFailedError(`Failed to start update process: ${error.message}`, error));
        });

      } catch (error) {
        reject(new UpdateFailedError(`Failed to execute update: ${error}`, error as Error));
      }
    });
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