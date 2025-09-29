# Build Scripts

This directory contains automation scripts for the atom-updater project.

## release.sh

The main release script that automates the entire release process:

```bash
# Perform a full release (build, tag, publish)
./build/release.sh

# Dry run to see what would happen
./build/release.sh --dry-run

# Cleanup a failed release
./build/release.sh --cleanup
```

**What it does:**
1. Pre-flight checks (dependencies, git status, branch)
2. Extract version from source code
3. Generate changelog from git commits
4. Create and push git tag
5. Run GoReleaser to build and publish release
6. **Automatically update atom-updater-node binaries** from build artifacts

## update-node-binaries.sh

Standalone script to update the Node.js wrapper binaries from GoReleaser artifacts:

```bash
# Update binaries after manual GoReleaser build
./build/update-node-binaries.sh
```

**What it does:**
1. Scans `dist/` directory for tar.gz/zip archives
2. Extracts them to `atom-updater-node/bin/{platform}/{arch}/`
3. Sets executable permissions on binaries
4. Cleans up archive files
5. Provides summary of installed binaries

**Archive naming pattern:**
- `atom-updater_Darwin_x86_64.tar.gz` → `bin/darwin/x64/atom-updater`
- `atom-updater_Linux_arm64.tar.gz` → `bin/linux/arm64/atom-updater`
- `atom-updater_Windows_i386.zip` → `bin/win32/ia32/atom-updater.exe`

## Integration

The `release.sh` script automatically calls `update-node-binaries.sh` after a successful GoReleaser build, so the Node.js wrapper binaries are always kept in sync with the Go releases.

If you run GoReleaser manually, you can call the binary update script separately:

```bash
goreleaser release --clean
./build/update-node-binaries.sh