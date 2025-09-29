/**
 * atom-updater Node.js wrapper
 *
 * A TypeScript wrapper for the atom-updater Go executable that provides
 * atomic directory replacement with rollback capabilities for Node.js and Electron applications.
 */

// Main classes and functions
export { AtomUpdater } from './AtomUpdater';
export { PlatformUtils } from './PlatformUtils';

// Types and interfaces
export type {
  UpdateConfig,
  UpdateResult,
  AtomUpdaterOptions,
  Platform,
  Architecture,
  PlatformInfo,
  DownloadOptions
} from './types';

// Error classes
export {
  AtomUpdaterError,
  ExecutableNotFoundError,
  UpdateFailedError,
  DownloadFailedError
} from './types';

// Convenience function for quick updates
import { AtomUpdater } from './AtomUpdater';
import { UpdateConfig, UpdateResult } from './types';

/**
 * Convenience function to perform an update with default options
 */
export async function update(config: UpdateConfig): Promise<UpdateResult> {
  const updater = new AtomUpdater();
  return await updater.update(config);
}

/**
 * Convenience function to check the atom-updater version
 */
export async function getVersion(): Promise<string> {
  const updater = new AtomUpdater();
  return await updater.getVersion();
}

/**
 * Check if atom-updater is available on the system
 */
export async function isAvailable(): Promise<boolean> {
  const updater = new AtomUpdater();
  return await updater.isAvailable();
}

// Default export
export default AtomUpdater;