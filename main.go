package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Version is the current version of atom-updater
const Version = "v2.0.0"

// Application types
type ApplicationType int

const (
	SingleFile ApplicationType = iota
	MacAppBundle
	MacAppBundleDirectory // Directory containing .app bundles
	MacDirectory
	WindowsAppDirectory
	LinuxAppDirectory
	GenericDirectory
)

// UpdateConfig holds configuration for the update process
type UpdateConfig struct {
	PID            int    `json:"pid"`
	CurrentPath    string `json:"current_path"`
	NewPath        string `json:"new_path"`
	AppName        string `json:"app_name,omitempty"`
	Timeout        int    `json:"timeout,omitempty"`
	VerifyChecksum bool   `json:"verify_checksum"`
	HealthCheckURL string `json:"health_check_url,omitempty"`
}

// Progress tracks the progress of directory operations
type Progress struct {
	CurrentFile string
	TotalFiles  int
	Processed   int
}

// Windows creation flags (numeric constants to avoid extra deps).
// https://learn.microsoft.com/en-us/windows/win32/procthread/process-creation-flags
// const (
// 	wCreateNoWindow   = 0x08000000 // CREATE_NO_WINDOW
// 	wDetachedProcess  = 0x00000008 // DETACHED_PROCESS
// 	wCreateNewProcGrp = 0x00000200 // CREATE_NEW_PROCESS_GROUP
// )

// generateTempFilename creates a unique temporary filename
func generateTempFilename(originalPath, suffix string) string {
	timestamp := strconv.FormatInt(time.Now().UnixNano(), 16)
	return fmt.Sprintf("%s.%s.%s", originalPath, suffix, timestamp[:8])
}

// typeToString converts ApplicationType to human-readable string
func typeToString(appType ApplicationType) string {
	switch appType {
	case SingleFile:
		return "single file (not supported)"
	case MacAppBundle:
		return "macOS app bundle (not supported)"
	case MacAppBundleDirectory:
		return "macOS app bundle directory"
	case MacDirectory:
		return "macOS directory"
	case WindowsAppDirectory:
		return "Windows directory"
	case LinuxAppDirectory:
		return "Linux directory"
	case GenericDirectory:
		return "generic directory"
	default:
		return "unknown"
	}
}

// areTypesCompatible checks if two application types can be updated from one to another
func areTypesCompatible(currentType, newType ApplicationType) bool {
	// Single file to single file is always compatible
	if currentType == SingleFile && newType == SingleFile {
		return true
	}

	// Any directory type to any other directory type is compatible
	// This allows updating between different platform-specific directory types
	if currentType != SingleFile && newType != SingleFile {
		return true
	}

	// Single file to directory or vice versa is not compatible
	return false
}

// detectApplicationType determines the type of application based on file system analysis
func detectApplicationType(appPath string) (ApplicationType, error) {
	info, err := os.Stat(appPath)
	if err != nil {
		return SingleFile, fmt.Errorf("failed to stat path %s: %w", appPath, err)
	}

	// Check if it's a single file
	if !info.IsDir() {
		return SingleFile, nil
	}

	// On macOS, treat .app bundles as single files, not directories
	if runtime.GOOS == "darwin" && strings.HasSuffix(appPath, ".app") {
		return MacAppBundle, nil
	}

	// It's a regular directory, analyze its contents
	switch runtime.GOOS {
	case "darwin":
		return detectMacDirectory(appPath)
	case "windows":
		return detectWindowsApp(appPath)
	default: // linux and others
		return detectLinuxApp(appPath)
	}
}

// containsAppBundles checks if a directory contains .app bundles
func containsAppBundles(dirPath string) (bool, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return false, err
	}

	for _, entry := range entries {
		if entry.IsDir() && strings.HasSuffix(entry.Name(), ".app") {
			return true, nil
		}
	}

	return false, nil
}

// detectMacDirectory detects macOS directory applications (non-bundle)
func detectMacDirectory(appPath string) (ApplicationType, error) {
	// First check if this directory contains .app bundles
	hasAppBundles, err := containsAppBundles(appPath)
	if err == nil && hasAppBundles {
		return MacAppBundleDirectory, nil
	}

	// Check if it's a regular directory with executables
	// On macOS, just search the directory itself
	executables, err := findExecutablesInDirectory(appPath, "")
	if err == nil && len(executables) > 0 {
		return MacDirectory, nil
	}

	return GenericDirectory, nil
}

// detectWindowsApp detects Windows application types
func detectWindowsApp(appPath string) (ApplicationType, error) {
	// Look for .exe files in the directory
	exeFiles, err := findExecutablesInDirectory(appPath, ".exe")
	if err != nil {
		return GenericDirectory, err
	}

	if len(exeFiles) > 0 {
		return WindowsAppDirectory, nil
	}

	return GenericDirectory, nil
}

// detectLinuxApp detects Linux application types
func detectLinuxApp(appPath string) (ApplicationType, error) {
	// Look for executable files in common locations
	locations := []string{
		filepath.Join(appPath, "bin"),
		filepath.Join(appPath, "usr", "bin"),
		appPath,
	}

	for _, location := range locations {
		if _, err := os.Stat(location); err == nil {
			executables, err := findExecutablesInDirectory(location, "")
			if err == nil && len(executables) > 0 {
				return LinuxAppDirectory, nil
			}
		}
	}

	return GenericDirectory, nil
}

// findExecutablesInDirectory finds executable files in a directory
func findExecutablesInDirectory(dir, extension string) ([]string, error) {
	var executables []string

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip files with permission errors
		}

		if d.IsDir() {
			// On macOS, treat .app directories as executable
			if runtime.GOOS == "darwin" && strings.HasSuffix(path, ".app") {
				relPath, _ := filepath.Rel(dir, path)
				executables = append(executables, relPath)
				return nil
			}
			return nil
		}

		// Check if file has executable extension or no extension (Linux)
		if extension != "" && !strings.HasSuffix(strings.ToLower(path), extension) {
			return nil
		}

		// Check if file is executable
		info, err := d.Info()
		if err != nil {
			return nil
		}

		if isExecutable(info) {
			relPath, _ := filepath.Rel(dir, path)
			executables = append(executables, relPath)
		}

		return nil
	})

	return executables, err
}

// isExecutable checks if a file is executable
func isExecutable(info fs.FileInfo) bool {
	// Check Unix executable permissions
	if runtime.GOOS != "windows" {
		return info.Mode().Perm()&0111 != 0
	}

	// On Windows, check file extensions
	ext := strings.ToLower(filepath.Ext(info.Name()))
	executableExts := []string{".exe", ".com", ".bat", ".cmd"}
	for _, exeExt := range executableExts {
		if ext == exeExt {
			return true
		}
	}

	return false
}

// findExecutableInDirectory finds the best executable to launch
func findExecutableInDirectory(appPath, preferredName string) (string, error) {
	appType, err := detectApplicationType(appPath)
	if err != nil {
		return "", err
	}

	var searchDirs []string
	var extension string

	switch appType {
	case MacDirectory:
		// On macOS, just search the directory itself
		searchDirs = []string{appPath}
		extension = ""
	case WindowsAppDirectory:
		searchDirs = []string{appPath}
		extension = ".exe"
	case LinuxAppDirectory:
		searchDirs = []string{appPath}
		extension = ""
	default:
		return "", fmt.Errorf("unsupported app type for executable detection: %v", appType)
	}

	// Search through all possible directories
	for _, searchDir := range searchDirs {
		if _, err := os.Stat(searchDir); err != nil {
			continue // Directory doesn't exist, try next one
		}

		executables, err := findExecutablesInDirectory(searchDir, extension)
		if err != nil || len(executables) == 0 {
			continue // No executables found, try next directory
		}

		// If preferred name is specified, look for it first
		if preferredName != "" {
			for _, exe := range executables {
				exeName := strings.ToLower(filepath.Base(exe))
				preferredLower := strings.ToLower(preferredName)

				// Remove extension from comparison if present
				if extension != "" {
					exeName = strings.TrimSuffix(exeName, extension)
				}

				if exeName == preferredLower {
					return filepath.Join(searchDir, exe), nil
				}
			}
		}

		// Return the first executable as fallback
		return filepath.Join(searchDir, executables[0]), nil
	}

	return "", fmt.Errorf("no executables found in any search directories")
}

// waitForProcessExit waits for the specified PID to exit
func waitForProcessExit(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		log.Printf("Process %d not found, assuming it already exited: %v", pid, err)
		return nil // Process doesn't exist, which is fine
	}

	// Wait for process to exit
	state, err := process.Wait()
	if err != nil {
		log.Printf("Process %d already exited or cannot wait: %v", pid, err)
		return nil // Process already exited, which is fine
	}

	log.Printf("Process %d exited with state: %v", pid, state)
	return nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %v", src, err)
	}
	defer sourceFile.Close()

	// Create destination directory if it doesn't exist
	destDir := filepath.Dir(dst)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory %s: %v", destDir, err)
	}

	destinationFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %v", dst, err)
	}
	defer destinationFile.Close()

	_, err = io.Copy(destinationFile, sourceFile)
	if err != nil {
		return fmt.Errorf("failed to copy file content: %v", err)
	}

	// Sync to ensure all data is written
	err = destinationFile.Sync()
	if err != nil {
		return fmt.Errorf("failed to sync destination file: %v", err)
	}

	return nil
}

// atomicReplace performs atomic file replacement with rollback capability
func atomicReplace(currentPath, newPath string) error {
	log.Printf("Starting atomic replacement: %s -> %s", newPath, currentPath)

	// Detect application types
	currentType, err := detectApplicationType(currentPath)
	if err != nil {
		return fmt.Errorf("failed to detect current app type: %w", err)
	}

	newType, err := detectApplicationType(newPath)
	if err != nil {
		return fmt.Errorf("failed to detect new app type: %w", err)
	}

	// Validate type compatibility
	if !areTypesCompatible(currentType, newType) {
		return fmt.Errorf("incompatible application types: current=%v (%s), new=%v (%s). Both must be either files or directories",
			currentType, typeToString(currentType), newType, typeToString(newType))
	}

	// Handle different application types
	switch currentType {
	case SingleFile:
		return fmt.Errorf("single file applications are not supported - use directory-based updates")
	case MacAppBundle:
		return fmt.Errorf("direct .app bundle arguments are not supported - use directory containing .app bundles")
	case MacAppBundleDirectory, MacDirectory, WindowsAppDirectory, LinuxAppDirectory, GenericDirectory:
		return atomicDirectoryReplace(currentPath, newPath)
	default:
		return fmt.Errorf("unsupported application type: %v", currentType)
	}
}

// atomicFileReplace performs atomic file replacement (original implementation)
func atomicFileReplace(currentPath, newPath string) error {
	log.Printf("Starting atomic file replacement: %s -> %s", newPath, currentPath)

	// Generate unique temporary filenames
	tempFile := generateTempFilename(currentPath, "tmp")
	newFile := generateTempFilename(currentPath, "new")

	// Step 1: Move current version to temp file (backup)
	log.Printf("Step 1: Backing up current version to %s", tempFile)
	if err := os.Rename(currentPath, tempFile); err != nil {
		return fmt.Errorf("failed to backup current version: %v", err)
	}

	// Step 2: Copy new version to intermediate file
	log.Printf("Step 2: Copying new version to %s", newFile)
	if err := copyFile(newPath, newFile); err != nil {
		// Rollback: restore from temp file
		log.Printf("Failed to copy new version, rolling back: %v", err)
		if rollbackErr := os.Rename(tempFile, currentPath); rollbackErr != nil {
			log.Printf("CRITICAL: Rollback failed: %v", rollbackErr)
		}
		return fmt.Errorf("failed to copy new version: %v", err)
	}

	// Step 3: Atomic move to final location
	log.Printf("Step 3: Moving to final location %s", currentPath)
	if err := os.Rename(newFile, currentPath); err != nil {
		// Rollback: restore from temp file
		log.Printf("Failed to move to final location, rolling back: %v", err)
		if rollbackErr := os.Rename(tempFile, currentPath); rollbackErr != nil {
			log.Printf("CRITICAL: Rollback failed: %v", rollbackErr)
		}
		// Clean up the intermediate file
		os.Remove(newFile)
		return fmt.Errorf("failed to move to final location: %v", err)
	}

	// Step 4: Clean up backup file
	log.Printf("Step 4: Cleaning up backup file %s", tempFile)
	if err := os.Remove(tempFile); err != nil {
		log.Printf("Warning: failed to remove backup file %s: %v", tempFile, err)
		// Don't return error here as the main operation succeeded
	}

	log.Printf("Atomic file replacement completed successfully")
	return nil
}

// atomicAppBundleDirectoryReplace performs atomic replacement for directories containing .app bundles
func atomicAppBundleDirectoryReplace(currentPath, newPath string) error {
	log.Printf("Starting atomic app bundle directory replacement: %s -> %s", newPath, currentPath)

	// Generate unique temporary subdirectory name inside current directory
	tempBackupSuffix := generateTempFilename("", "backup")
	tempBackupDir := filepath.Join(currentPath, tempBackupSuffix)

	// Step 1: Create temp backup directory inside current directory
	log.Printf("Step 1: Creating backup directory %s", tempBackupDir)
	if err := os.MkdirAll(tempBackupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %v", err)
	}

	// Step 2: Move all current files to backup directory, treating .app bundles as atomic files
	log.Printf("Step 2: Moving current files to backup")
	if err := moveAppBundleDirectoryContents(currentPath, tempBackupDir); err != nil {
		// Rollback: remove the backup directory we created
		log.Printf("Failed to move files to backup, cleaning up: %v", err)
		os.RemoveAll(tempBackupDir)
		return fmt.Errorf("failed to backup current files: %v", err)
	}

	// Step 3: Copy new files to current directory, treating .app bundles as atomic files
	log.Printf("Step 3: Copying new files to current directory")
	if err := copyAppBundleDirectoryTree(newPath, currentPath); err != nil {
		// Rollback: move files back from backup
		log.Printf("Failed to copy new files, rolling back: %v", err)
		if rollbackErr := restoreAppBundleDirectoryBackup(tempBackupDir, currentPath); rollbackErr != nil {
			log.Printf("CRITICAL: Rollback failed: %v", rollbackErr)
		}
		return fmt.Errorf("failed to copy new directory: %v", err)
	}

	// Step 4: Clean up backup directory
	log.Printf("Step 4: Cleaning up backup directory %s", tempBackupDir)
	if err := os.RemoveAll(tempBackupDir); err != nil {
		log.Printf("Warning: failed to remove backup directory %s: %v", tempBackupDir, err)
		// Don't return error here as the main operation succeeded
	}

	log.Printf("Atomic app bundle directory replacement completed successfully")
	return nil
}

// atomicDirectoryReplace performs atomic directory replacement with robust rollback capability
func atomicDirectoryReplace(currentPath, newPath string) error {
	log.Printf("Starting robust atomic directory replacement: %s -> %s", newPath, currentPath)

	// Check if this is a directory containing .app bundles
	currentType, err := detectApplicationType(currentPath)
	if err != nil {
		return fmt.Errorf("failed to detect current app type: %w", err)
	}

	if currentType == MacAppBundleDirectory {
		return atomicAppBundleDirectoryReplace(currentPath, newPath)
	}

	// Generate unique temporary subdirectory name inside current directory
	tempBackupSuffix := generateTempFilename("", "backup")
	tempBackupDir := filepath.Join(currentPath, tempBackupSuffix)

	// Step 1: Create temp backup directory inside current directory
	log.Printf("Step 1: Creating backup directory %s", tempBackupDir)
	if err := os.MkdirAll(tempBackupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %v", err)
	}

	// Step 2: Move all current files to backup directory
	log.Printf("Step 2: Moving current files to backup")
	if err := moveContentsToBackup(currentPath, tempBackupDir); err != nil {
		// Rollback: remove the backup directory we created
		log.Printf("Failed to move files to backup, cleaning up: %v", err)
		os.RemoveAll(tempBackupDir)
		return fmt.Errorf("failed to backup current files: %v", err)
	}

	// Step 3: Copy new files to current directory
	log.Printf("Step 3: Copying new files to current directory")
	if err := copyDirectoryTree(newPath, currentPath); err != nil {
		// Rollback: move files back from backup
		log.Printf("Failed to copy new files, rolling back: %v", err)
		if rollbackErr := restoreFromBackup(tempBackupDir, currentPath); rollbackErr != nil {
			log.Printf("CRITICAL: Rollback failed: %v", rollbackErr)
		}
		return fmt.Errorf("failed to copy new directory: %v", err)
	}

	// Step 4: Clean up backup directory
	log.Printf("Step 4: Cleaning up backup directory %s", tempBackupDir)
	if err := os.RemoveAll(tempBackupDir); err != nil {
		log.Printf("Warning: failed to remove backup directory %s: %v", tempBackupDir, err)
		// Don't return error here as the main operation succeeded
	}

	log.Printf("Robust atomic directory replacement completed successfully")
	return nil
}

// moveAppBundleDirectoryContents moves directory contents, treating .app bundles as atomic files
func moveAppBundleDirectoryContents(currentPath, backupDir string) error {
	entries, err := os.ReadDir(currentPath)
	if err != nil {
		return fmt.Errorf("failed to read current directory: %v", err)
	}

	// Get backup directory name to avoid moving it into itself
	backupName := filepath.Base(backupDir)

	// Move each entry to backup directory
	for _, entry := range entries {
		entryPath := filepath.Join(currentPath, entry.Name())

		// Skip the backup directory itself
		if entry.Name() == backupName {
			continue
		}

		backupPath := filepath.Join(backupDir, entry.Name())

		if entry.IsDir() && strings.HasSuffix(entry.Name(), ".app") {
			// Treat .app bundles as atomic files - move the entire bundle
			log.Printf("Moving .app bundle to backup: %s -> %s", entryPath, backupPath)
			if err := os.Rename(entryPath, backupPath); err != nil {
				return fmt.Errorf("failed to move .app bundle %s to backup: %v", entryPath, err)
			}
		} else if entry.IsDir() {
			// For regular directories, create directory in backup with original permissions
			srcInfo, err := os.Stat(entryPath)
			if err != nil {
				return fmt.Errorf("failed to stat directory %s: %v", entryPath, err)
			}

			if err := os.Mkdir(backupPath, srcInfo.Mode()); err != nil {
				return fmt.Errorf("failed to create backup directory %s: %v", backupPath, err)
			}

			// Recursively move contents
			if err := moveDirectoryContents(entryPath, backupPath); err != nil {
				return fmt.Errorf("failed to move directory contents: %v", err)
			}

			// Remove the original directory after moving contents
			if err := os.RemoveAll(entryPath); err != nil {
				return fmt.Errorf("failed to remove original directory %s: %v", entryPath, err)
			}
		} else {
			// Move file to backup
			if err := os.Rename(entryPath, backupPath); err != nil {
				return fmt.Errorf("failed to move file %s to backup: %v", entryPath, err)
			}
		}
	}

	return nil
}

// copyAppBundleSystem copies a .app bundle using Apple's ditto command
func copyAppBundleSystem(src, dst string) error {
	log.Printf("Using ditto to copy .app bundle: %s -> %s", src, dst)

	// Use Apple's ditto command which is recommended for .app bundles
	// ditto preserves all macOS-specific attributes, permissions, and metadata
	cmd := exec.Command("ditto", src, dst)
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ditto failed: %w", err)
	}

	log.Printf("ditto completed successfully")
	return nil
}

// copyFileWithPermissions copies a file with appropriate permissions for .app bundles
func copyFileWithPermissions(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %v", src, err)
	}
	defer sourceFile.Close()

	// Create destination directory if it doesn't exist
	destDir := filepath.Dir(dst)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory %s: %v", destDir, err)
	}

	// Create destination file with write permissions
	destinationFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %v", dst, err)
	}
	defer destinationFile.Close()

	_, err = io.Copy(destinationFile, sourceFile)
	if err != nil {
		return fmt.Errorf("failed to copy file content: %v", err)
	}

	// Sync to ensure all data is written
	err = destinationFile.Sync()
	if err != nil {
		return fmt.Errorf("failed to sync destination file: %v", err)
	}

	// Close file before changing permissions
	destinationFile.Close()

	// Set appropriate permissions for .app bundle files
	// Use 0644 (readable by all, writable by owner) to avoid permission issues
	if err := os.Chmod(dst, 0644); err != nil {
		log.Printf("Warning: failed to set permissions on %s: %v", dst, err)
		// Don't return error here as the copy succeeded
	}

	return nil
}

// copyAppBundle copies a .app bundle directory without creating destination first
func copyAppBundle(src, dst string) error {
	log.Printf("Copying .app bundle directory: %s -> %s", src, dst)

	// Get source directory info
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to stat source: %w", err)
	}

	// Create destination directory with same permissions
	if err := os.Mkdir(dst, srcInfo.Mode()); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Copy contents using WalkDir
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(dst, relPath)

		if d.IsDir() {
			// Skip the root directory (already created)
			if path != src {
				if err := os.MkdirAll(destPath, d.Type()); err != nil {
					return fmt.Errorf("failed to create directory %s: %w", destPath, err)
				}
			}
		} else {
			// Copy file
			if err := copyFile(path, destPath); err != nil {
				return fmt.Errorf("failed to copy file %s: %w", path, err)
			}
		}

		return nil
	})
}

// copyAppBundleDirectoryTree copies directory tree, treating .app bundles as atomic files
func copyAppBundleDirectoryTree(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to stat source: %w", err)
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("failed to read source directory: %w", err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() && strings.HasSuffix(entry.Name(), ".app") {
			// Treat .app bundles as atomic units using the correct macOS approach
			log.Printf("Atomic .app bundle replacement: %s -> %s", srcPath, dstPath)

			// Create temporary destination for new .app bundle
			tempDstPath := dstPath + ".new"
			os.RemoveAll(tempDstPath) // Clean up any previous failed attempt

			// Copy new .app bundle to temporary location using system cp command
			log.Printf("Copying .app bundle to temp location: %s", tempDstPath)
			if err := copyAppBundleSystem(srcPath, tempDstPath); err != nil {
				os.RemoveAll(tempDstPath) // Clean up on failure
				return fmt.Errorf("failed to copy .app bundle to temp location: %w", err)
			}

			// If destination exists, backup the old one
			if _, err := os.Stat(dstPath); err == nil {
				oldPath := dstPath + ".old"
				os.RemoveAll(oldPath) // Remove any previous backup
				log.Printf("Backing up existing .app bundle: %s -> %s", dstPath, oldPath)
				if err := os.Rename(dstPath, oldPath); err != nil {
					os.RemoveAll(tempDstPath) // Clean up temp on failure
					return fmt.Errorf("failed to backup existing .app bundle: %w", err)
				}
			}

			// Atomic move to final location
			log.Printf("Moving .app bundle to final location: %s -> %s", tempDstPath, dstPath)
			if err := os.Rename(tempDstPath, dstPath); err != nil {
				// Restore from backup on failure
				if _, err := os.Stat(dstPath + ".old"); err == nil {
					os.Rename(dstPath+".old", dstPath)
				}
				os.RemoveAll(tempDstPath)
				return fmt.Errorf("failed to move .app bundle to final location: %w", err)
			}

			log.Printf("Successfully replaced .app bundle")
		} else if entry.IsDir() {
			// For regular directories, recursively copy
			if err := copyDirectoryTree(srcPath, dstPath); err != nil {
				return fmt.Errorf("failed to copy directory %s: %w", srcPath, err)
			}
		} else {
			// Copy file
			if err := copyFile(srcPath, dstPath); err != nil {
				return fmt.Errorf("failed to copy file %s: %w", srcPath, err)
			}
		}
	}

	return nil
}

// restoreAppBundleDirectoryBackup restores files from backup, treating .app bundles as atomic files
func restoreAppBundleDirectoryBackup(backupDir, currentPath string) error {
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return fmt.Errorf("failed to read backup directory: %v", err)
	}

	// Restore each entry from backup to current path
	for _, entry := range entries {
		backupPath := filepath.Join(backupDir, entry.Name())
		originalPath := filepath.Join(currentPath, entry.Name())

		if entry.IsDir() && strings.HasSuffix(entry.Name(), ".app") {
			// Treat .app bundles as atomic units - use atomic replacement for restore too
			log.Printf("Restoring .app bundle: %s -> %s", backupPath, originalPath)

			// If destination exists, backup current version first
			if _, err := os.Stat(originalPath); err == nil {
				currentBackup := originalPath + ".current"
				os.RemoveAll(currentBackup)
				if err := os.Rename(originalPath, currentBackup); err != nil {
					return fmt.Errorf("failed to backup current .app bundle during restore: %v", err)
				}
				defer func() {
					if _, err := os.Stat(originalPath); os.IsNotExist(err) {
						// Restore succeeded, clean up current backup
						os.RemoveAll(currentBackup)
					} else {
						// Restore failed, restore from current backup
						os.Rename(currentBackup, originalPath)
					}
				}()
			}

			// Move from backup to original location
			if err := os.Rename(backupPath, originalPath); err != nil {
				return fmt.Errorf("failed to restore .app bundle %s: %v", backupPath, err)
			}
		} else if entry.IsDir() {
			// For regular directories, create it first
			if err := os.MkdirAll(originalPath, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %v", originalPath, err)
			}
			// Recursively restore directory contents
			if err := restoreDirectoryContents(backupPath, originalPath); err != nil {
				return fmt.Errorf("failed to restore directory contents: %v", err)
			}
		} else {
			// Move file back from backup
			if err := os.Rename(backupPath, originalPath); err != nil {
				return fmt.Errorf("failed to restore file %s: %v", backupPath, err)
			}
		}
	}

	return nil
}

// moveContentsToBackup moves all contents of currentPath to backupDir
func moveContentsToBackup(currentPath, backupDir string) error {
	// First, read the current directory contents
	entries, err := os.ReadDir(currentPath)
	if err != nil {
		return fmt.Errorf("failed to read current directory: %v", err)
	}

	// Get backup directory name to avoid moving it into itself
	backupName := filepath.Base(backupDir)

	// Move each entry to backup directory
	for _, entry := range entries {
		entryPath := filepath.Join(currentPath, entry.Name())

		// Skip the backup directory itself
		if entry.Name() == backupName {
			continue
		}

		backupPath := filepath.Join(backupDir, entry.Name())

		if entry.IsDir() {
			// Get original directory permissions
			srcInfo, err := os.Stat(entryPath)
			if err != nil {
				return fmt.Errorf("failed to stat directory %s: %v", entryPath, err)
			}

			// Create directory in backup with original permissions
			if err := os.Mkdir(backupPath, srcInfo.Mode()); err != nil {
				return fmt.Errorf("failed to create backup directory %s: %v", backupPath, err)
			}

			// For directories, we need to move contents recursively
			if err := moveDirectoryContents(entryPath, backupPath); err != nil {
				return fmt.Errorf("failed to move directory contents: %v", err)
			}

			// Remove the original directory after moving contents
			if err := os.RemoveAll(entryPath); err != nil {
				return fmt.Errorf("failed to remove original directory %s: %v", entryPath, err)
			}
		} else {
			// Move file to backup
			if err := os.Rename(entryPath, backupPath); err != nil {
				return fmt.Errorf("failed to move file %s to backup: %v", entryPath, err)
			}
		}
	}

	return nil
}

// moveDirectoryContents recursively moves directory contents
func moveDirectoryContents(srcDir, dstDir string) error {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(srcDir, entry.Name())
		dstPath := filepath.Join(dstDir, entry.Name())

		if entry.IsDir() {
			// Get original directory permissions
			srcInfo, err := os.Stat(srcPath)
			if err != nil {
				return err
			}

			// Create directory with original permissions
			if err := os.Mkdir(dstPath, srcInfo.Mode()); err != nil {
				return fmt.Errorf("failed to create directory %s: %v", dstPath, err)
			}

			// Recursively move contents
			if err := moveDirectoryContents(srcPath, dstPath); err != nil {
				return err
			}

			// Remove original directory after moving contents
			if err := os.RemoveAll(srcPath); err != nil {
				return fmt.Errorf("failed to remove original directory %s: %v", srcPath, err)
			}
		} else {
			// Move file
			if err := os.Rename(srcPath, dstPath); err != nil {
				return fmt.Errorf("failed to move file %s to %s: %v", srcPath, dstPath, err)
			}
		}
	}

	return nil
}

// restoreFromBackup restores files from backupDir to currentPath
func restoreFromBackup(backupDir, currentPath string) error {
	// Read backup directory contents
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return fmt.Errorf("failed to read backup directory: %v", err)
	}

	// Restore each entry from backup to current path
	for _, entry := range entries {
		backupPath := filepath.Join(backupDir, entry.Name())
		originalPath := filepath.Join(currentPath, entry.Name())

		if entry.IsDir() {
			// For directories, create it first
			if err := os.MkdirAll(originalPath, entry.Type()); err != nil {
				return fmt.Errorf("failed to create directory %s: %v", originalPath, err)
			}
			// Recursively restore directory contents
			if err := restoreDirectoryContents(backupPath, originalPath); err != nil {
				return fmt.Errorf("failed to restore directory contents: %v", err)
			}
		} else {
			// Move file back from backup
			if err := os.Rename(backupPath, originalPath); err != nil {
				return fmt.Errorf("failed to restore file %s: %v", backupPath, err)
			}
		}
	}

	return nil
}

// restoreDirectoryContents recursively restores directory contents
func restoreDirectoryContents(backupPath, originalPath string) error {
	entries, err := os.ReadDir(backupPath)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(backupPath, entry.Name())
		dstPath := filepath.Join(originalPath, entry.Name())

		if entry.IsDir() {
			// Get original permissions from backup
			srcInfo, err := os.Stat(srcPath)
			if err != nil {
				return err
			}

			// Create directory with original permissions
			if err := os.Mkdir(dstPath, srcInfo.Mode()); err != nil {
				return fmt.Errorf("failed to create directory %s: %v", dstPath, err)
			}

			// Recursively restore contents
			if err := restoreDirectoryContents(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			// Restore file
			if err := os.Rename(srcPath, dstPath); err != nil {
				return fmt.Errorf("failed to restore file %s to %s: %v", srcPath, dstPath, err)
			}
		}
	}

	return nil
}

// copyDirectoryTree recursively copies a directory tree
func copyDirectoryTree(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to stat source: %w", err)
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(dst, relPath)

		if d.IsDir() {
			return os.MkdirAll(destPath, d.Type())
		}

		return copyFile(path, destPath)
	})
}

// launchApplication launches the updated application with smart detection
func launchApplication(appPath, appName string) error {
	if appPath == "" {
		return fmt.Errorf("app path is empty")
	}

	absPath, err := filepath.Abs(appPath)
	if err != nil {
		return fmt.Errorf("failed to resolve app path: %w", err)
	}

	log.Printf("Launching application: %s", absPath)

	appType, err := detectApplicationType(absPath)
	if err != nil {
		return fmt.Errorf("failed to detect app type: %w", err)
	}

	switch appType {
	case SingleFile:
		return launchSingleFile(absPath)
	case MacAppBundle:
		return launchMacAppBundle(absPath)
	case MacAppBundleDirectory:
		return launchMacAppBundleDirectory(absPath, appName)
	case MacDirectory:
		return launchMacDirectory(absPath, appName)
	case WindowsAppDirectory:
		return launchWindowsApp(absPath, appName)
	case LinuxAppDirectory:
		return launchLinuxApp(absPath, appName)
	default:
		return fmt.Errorf("unsupported app type for launch: %v", appType)
	}
}

// launchSingleFile launches a single executable file
func launchSingleFile(appPath string) error {
	workDir := filepath.Dir(appPath)

	log.Printf("Launching single file: %s", appPath)

	cmd := exec.Command(appPath)
	cmd.Dir = workDir
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to launch single file: %w", err)
	}

	log.Printf("Single file launched with PID: %d", cmd.Process.Pid)
	return nil
}

// launchMacAppBundleDirectory launches the first .app bundle found in a directory
func launchMacAppBundleDirectory(appPath, appName string) error {
	log.Printf("Launching first .app bundle from directory: %s", appPath)

	// Find the first .app bundle in the directory
	entries, err := os.ReadDir(appPath)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	var firstAppBundle string
	for _, entry := range entries {
		if entry.IsDir() && strings.HasSuffix(entry.Name(), ".app") {
			firstAppBundle = filepath.Join(appPath, entry.Name())
			break
		}
	}

	if firstAppBundle == "" {
		return fmt.Errorf("no .app bundle found in directory: %s", appPath)
	}

	log.Printf("Found .app bundle: %s", firstAppBundle)
	return launchMacAppBundle(firstAppBundle)
}

// launchMacAppBundle launches a macOS .app bundle
func launchMacAppBundle(appPath string) error {
	workDir := filepath.Dir(appPath)

	log.Printf("Launching macOS app bundle: %s", appPath)

	// Use 'open' command for .app bundles
	cmd := exec.Command("open", appPath)
	cmd.Dir = workDir
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to launch macOS app bundle: %w", err)
	}

	log.Printf("macOS app bundle launched with PID: %d", cmd.Process.Pid)
	return nil
}

// launchMacDirectory launches a macOS directory with executables
func launchMacDirectory(appPath, appName string) error {
	workDir := filepath.Dir(appPath)

	// Find the executable to launch
	executable, err := findExecutableInDirectory(appPath, appName)
	if err != nil {
		return fmt.Errorf("failed to find executable: %w", err)
	}

	log.Printf("Launching macOS directory app: %s", executable)

	cmd := exec.Command(executable)
	cmd.Dir = workDir
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to launch macOS directory app: %w", err)
	}

	log.Printf("macOS directory app launched with PID: %d", cmd.Process.Pid)
	return nil
}

// launchWindowsApp launches a Windows application from a directory
func launchWindowsApp(appPath, appName string) error {
	workDir := filepath.Dir(appPath)

	// Find the executable to launch
	executable, err := findExecutableInDirectory(appPath, appName)
	if err != nil {
		return fmt.Errorf("failed to find executable: %w", err)
	}

	log.Printf("Launching Windows app: %s", executable)

	cmd := exec.Command(executable)
	cmd.Dir = workDir
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to launch Windows app: %w", err)
	}

	log.Printf("Windows app launched with PID: %d", cmd.Process.Pid)
	return nil
}

// launchLinuxApp launches a Linux application from a directory
func launchLinuxApp(appPath, appName string) error {
	workDir := filepath.Dir(appPath)

	// Find the executable to launch
	executable, err := findExecutableInDirectory(appPath, appName)
	if err != nil {
		return fmt.Errorf("failed to find executable: %w", err)
	}

	log.Printf("Launching Linux app: %s", executable)

	cmd := exec.Command(executable)
	cmd.Dir = workDir
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to launch Linux app: %w", err)
	}

	log.Printf("Linux app launched with PID: %d", cmd.Process.Pid)
	return nil
}

// printVersion prints the version information
func printVersion() {
	// fmt.Printf("atom-updater version %s\n", Version)
	// fmt.Printf("Built with %s on %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
	fmt.Printf("%s\n", Version)
}

// verifyChecksum verifies the SHA256 checksum of a file
func verifyChecksum(filePath, expectedChecksum string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file for checksum verification: %v", err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return fmt.Errorf("failed to read file for checksum: %v", err)
	}

	actualChecksum := fmt.Sprintf("%x", hash.Sum(nil))
	if actualChecksum != expectedChecksum {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedChecksum, actualChecksum)
	}

	log.Printf("Checksum verification passed for %s", filePath)
	return nil
}

// setupLogging configures logging to both console and file
func setupLogging() {
	// Get the directory where the executable is located
	execPath, err := os.Executable()
	if err != nil {
		log.Printf("Warning: Could not get executable path: %v", err)
		execPath = "atom-updater" // fallback
	}

	execDir := filepath.Dir(execPath)
	logFilePath := filepath.Join(execDir, "atom-updater.log")

	// Clear the log file at startup
	if err := os.WriteFile(logFilePath, []byte(""), 0644); err != nil {
		log.Printf("Warning: Could not clear log file %s: %v", logFilePath, err)
	}

	// Open log file for appending
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Printf("Warning: Could not open log file %s: %v", logFilePath, err)
		log.Printf("Continuing with console-only logging...")
		return
	}

	// Set up logging to both console and file
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.SetOutput(io.MultiWriter(os.Stderr, logFile))

	log.Printf("=== Atom-Updater Started ===")
	log.Printf("Log file: %s", logFilePath)
}

// getExecutableDir returns the directory containing the atom-updater executable
func getExecutableDir() string {
	execPath, err := os.Executable()
	if err != nil {
		return "." // fallback to current directory
	}
	return filepath.Dir(execPath)
}

func main() {
	// Setup logging to both console and file
	setupLogging()

	// Parse command line arguments
	config, err := parseArgs(os.Args)
	if err != nil {
		log.Fatal(err)
	}

	// Handle special commands
	if config == nil {
		return // Version or help was displayed
	}

	log.Printf("Starting update process:")
	log.Printf("  PID: %d", config.PID)
	log.Printf("  Current path: %s", config.CurrentPath)
	log.Printf("  New path: %s", config.NewPath)
	if config.AppName != "" {
		log.Printf("  App name: %s", config.AppName)
	}

	// Validate that both paths are directories (not files or .app bundles)
	currentInfo, err := os.Stat(config.CurrentPath)
	if os.IsNotExist(err) {
		log.Fatalf("Current application does not exist: %s", config.CurrentPath)
	}
	if !currentInfo.IsDir() {
		log.Fatalf("Current path must be a directory, not a file: %s", config.CurrentPath)
	}

	newInfo, err := os.Stat(config.NewPath)
	if os.IsNotExist(err) {
		log.Fatalf("New application does not exist: %s", config.NewPath)
	}
	if !newInfo.IsDir() {
		log.Fatalf("New path must be a directory, not a file: %s", config.NewPath)
	}

	// Additional validation: don't allow .app bundles as direct arguments
	if strings.HasSuffix(config.CurrentPath, ".app") {
		log.Fatalf("Current path cannot be a .app bundle, must be a directory: %s", config.CurrentPath)
	}
	if strings.HasSuffix(config.NewPath, ".app") {
		log.Fatalf("New path cannot be a .app bundle, must be a directory: %s", config.NewPath)
	}

	// Step 1: Wait for the target process to exit
	log.Printf("Waiting for process %d to exit...", config.PID)
	if err := waitForProcessExit(config.PID); err != nil {
		log.Printf("Warning: Failed to wait for process exit: %v", err)
		log.Printf("Continuing with update anyway...")
	}

	// Step 2: Perform atomic replacement
	if err := atomicReplace(config.CurrentPath, config.NewPath); err != nil {
		log.Fatalf("Atomic replacement failed: %v", err)
	}

	// Step 3: Launch the updated application
	if err := launchApplication(config.CurrentPath, config.AppName); err != nil {
		log.Printf("Warning: Failed to launch updated application: %v", err)
		// Don't exit here as the replacement was successful
	}

	log.Printf("Update process completed successfully")
}

// parseArgs parses command line arguments with support for the new app name parameter
func parseArgs(args []string) (*UpdateConfig, error) {
	if len(args) < 2 {
		showUsage()
		return nil, nil
	}

	switch args[1] {
	case "-v", "--version":
		printVersion()
		return nil, nil

	case "-h", "--help":
		showHelp()
		return nil, nil
	}

	// Parse update command arguments
	// Support both old format: <pid> <current_path> <new_path>
	// And new format: <pid> <current_path> <new_path> --app-name <name>

	var pid int
	var currentPath, newPath, appName string

	// Check if we have the app-name flag
	if len(args) >= 6 && args[4] == "--app-name" {
		// New format with app name
		var err error
		pid, err = strconv.Atoi(args[1])
		if err != nil {
			return nil, fmt.Errorf("invalid PID '%s': %v", args[1], err)
		}
		currentPath = args[2]
		newPath = args[3]
		appName = args[5]
	} else if len(args) == 4 {
		// Old format without app name
		var err error
		pid, err = strconv.Atoi(args[1])
		if err != nil {
			return nil, fmt.Errorf("invalid PID '%s': %v", args[1], err)
		}
		currentPath = args[2]
		newPath = args[3]
		appName = ""
	} else {
		return nil, fmt.Errorf("invalid arguments. Use '%s --help' for usage information", args[0])
	}

	// Resolve paths to absolute paths
	absCurrentPath, err := filepath.Abs(currentPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve current path '%s': %v", currentPath, err)
	}

	absNewPath, err := filepath.Abs(newPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve new path '%s': %v", newPath, err)
	}

	return &UpdateConfig{
		PID:         pid,
		CurrentPath: absCurrentPath,
		NewPath:     absNewPath,
		AppName:     appName,
	}, nil
}

// showUsage displays brief usage information
func showUsage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [options] <pid> <current_dir> <new_dir> [--app-name <name>]\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "Options:\n")
	fmt.Fprintf(os.Stderr, "  -v, --version    Show version information\n")
	fmt.Fprintf(os.Stderr, "  -h, --help       Show this help message\n")
	fmt.Fprintf(os.Stderr, "\nNote: Both current_dir and new_dir must be directories (not files or .app bundles)\n")
}

// showHelp displays detailed help information
func showHelp() {
	fmt.Fprintf(os.Stderr, "atom-updater %s - Directory-based application updater with atomic replacement\n\n", Version)
	fmt.Fprintf(os.Stderr, "Usage: %s [options] <pid> <current_dir> <new_dir> [--app-name <name>]\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "Usage: %s --version\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "\nOptions:\n")
	fmt.Fprintf(os.Stderr, "  -v, --version    Show version information\n")
	fmt.Fprintf(os.Stderr, "  -h, --help       Show this help message\n")
	fmt.Fprintf(os.Stderr, "\nParameters:\n")
	fmt.Fprintf(os.Stderr, "  <pid>            Process ID to wait for exit\n")
	fmt.Fprintf(os.Stderr, "  <current_dir>    Path to current application directory (must be directory)\n")
	fmt.Fprintf(os.Stderr, "  <new_dir>        Path to new application directory (must be directory)\n")
	fmt.Fprintf(os.Stderr, "  --app-name <name> Optional: Name of executable to launch (for directories)\n")
	fmt.Fprintf(os.Stderr, "\n⚠️  Restrictions:\n")
	fmt.Fprintf(os.Stderr, "  - Both current_dir and new_dir MUST be directories\n")
	fmt.Fprintf(os.Stderr, "  - Single files (like .exe) are NOT allowed\n")
	fmt.Fprintf(os.Stderr, "  - .app bundles are NOT allowed as direct arguments\n")
	fmt.Fprintf(os.Stderr, "\nExamples:\n")
	fmt.Fprintf(os.Stderr, "  # macOS directory containing .app bundles\n")
	fmt.Fprintf(os.Stderr, "  %s 12345 ./test/myapp ./test/updates/macapp\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "\n  # Windows directory with specific exe\n")
	fmt.Fprintf(os.Stderr, "  %s 12345 ./MyApp/ ./updates/MyApp/ --app-name app.exe\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "\nSupported application types:\n")
	fmt.Fprintf(os.Stderr, "  - macOS directories containing .app bundles ✨\n")
	fmt.Fprintf(os.Stderr, "  - macOS directories with executables\n")
	fmt.Fprintf(os.Stderr, "  - Windows directories with executables\n")
	fmt.Fprintf(os.Stderr, "  - Linux directories with executables\n")
}
