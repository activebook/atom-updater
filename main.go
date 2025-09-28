package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"time"
)

// Version is the current version of atom-updater
const Version = "v1.0.0"

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

// waitForProcessExit waits for the specified PID to exit
func waitForProcessExit(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process %d: %v", pid, err)
	}

	// Wait for process to exit
	state, err := process.Wait()
	if err != nil {
		return fmt.Errorf("error waiting for process %d: %v", pid, err)
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

	log.Printf("Atomic replacement completed successfully")
	return nil
}

// launchApplication launches the updated application
func launchApplication(appPath string) error {
	if appPath == "" {
		return fmt.Errorf("app path is empty")
	}
	absPath, err := filepath.Abs(appPath)
	if err != nil {
		return fmt.Errorf("failed to resolve app path: %w", err)
	}
	workDir := filepath.Dir(absPath)

	log.Printf("Launching application: %s", absPath)

	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		// For Windows: ensure no console window appears even for console-subsystem EXEs.
		cmd = exec.Command(absPath)
		cmd.Dir = workDir

		// Detach from the parent console and do not create a new one.
		// HideWindow suppresses any window associated with the process creation.
		// cmd.SysProcAttr = &syscall.SysProcAttr{
		// 	HideWindow:    true,
		// 	CreationFlags: wDetachedProcess | wCreateNoWindow | wCreateNewProcGrp,
		// }

		// Do NOT inherit stdio; attaching stdio can rebind to a console.
		cmd.Stdin = nil
		cmd.Stdout = nil
		cmd.Stderr = nil

	case "darwin":
		// On macOS, use open so app bundles (.app) launch properly.
		// For raw binaries, open will also work, but you can call the binary directly if needed.
		cmd = exec.Command("open", absPath)
		cmd.Dir = workDir

	default: // linux and others
		// On Linux, launching directly is fine. If opening a desktop file or URL, prefer xdg-open.
		cmd = exec.Command(absPath)
		cmd.Dir = workDir
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to launch application: %w", err)
	}

	log.Printf("Application launched with PID: %d", cmd.Process.Pid)
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

func main() {
	// Enable logging with timestamps
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <pid> <current_path> <new_path>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		fmt.Fprintf(os.Stderr, "  -v, --version    Show version information\n")
		fmt.Fprintf(os.Stderr, "  -h, --help       Show this help message\n")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "-v", "--version":
		printVersion()
		return

	case "-h", "--help":
		fmt.Fprintf(os.Stderr, "atom-updater %s - Application updater with atomic replacement\n\n", Version)
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <pid> <current_path> <new_path>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Usage: %s --version\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		fmt.Fprintf(os.Stderr, "  -v, --version    Show version information\n")
		fmt.Fprintf(os.Stderr, "  -h, --help       Show this help message\n")
		fmt.Fprintf(os.Stderr, "\nCommands:\n")
		fmt.Fprintf(os.Stderr, "  <pid> <current_path> <new_path>\n")
		fmt.Fprintf(os.Stderr, "    Wait for process <pid> to exit, then atomically replace\n")
		fmt.Fprintf(os.Stderr, "    <current_path> with <new_path>, and launch the updated application.\n")
		return
	}

	// Update command: atom-updater <pid> <current_path> <new_path>
	if len(os.Args) != 4 {
		fmt.Fprintf(os.Stderr, "Error: Update command requires exactly 3 arguments: <pid> <current_path> <new_path>\n")
		fmt.Fprintf(os.Stderr, "Use '%s --help' for more information\n", os.Args[0])
		os.Exit(1)
	}

	// Parse PID
	pid, err := strconv.Atoi(os.Args[1])
	if err != nil {
		log.Fatalf("Invalid PID '%s': %v", os.Args[1], err)
	}

	// Resolve paths to absolute paths to handle relative paths correctly
	currentPath, err := filepath.Abs(os.Args[2])
	if err != nil {
		log.Fatalf("Failed to resolve current path '%s': %v", os.Args[2], err)
	}

	newPath, err := filepath.Abs(os.Args[3])
	if err != nil {
		log.Fatalf("Failed to resolve new path '%s': %v", os.Args[3], err)
	}

	log.Printf("Starting update process:")
	log.Printf("  PID: %d", pid)
	log.Printf("  Current path: %s", currentPath)
	log.Printf("  New path: %s", newPath)

	// Validate that source files exist
	if _, err := os.Stat(currentPath); os.IsNotExist(err) {
		log.Fatalf("Current application does not exist: %s", currentPath)
	}

	if _, err := os.Stat(newPath); os.IsNotExist(err) {
		log.Fatalf("New application does not exist: %s", newPath)
	}

	// Step 1: Wait for the target process to exit
	log.Printf("Waiting for process %d to exit...", pid)
	if err := waitForProcessExit(pid); err != nil {
		log.Fatalf("Failed to wait for process exit: %v", err)
	}

	// Step 2: Perform atomic replacement
	if err := atomicReplace(currentPath, newPath); err != nil {
		log.Fatalf("Atomic replacement failed: %v", err)
	}

	// Step 3: Launch the updated application
	if err := launchApplication(currentPath); err != nil {
		log.Printf("Warning: Failed to launch updated application: %v", err)
		// Don't exit here as the replacement was successful
	}

	log.Printf("Update process completed successfully")
}
