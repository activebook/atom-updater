#!/usr/bin/env node

/**
 * Simple test script to verify the atom-updater wrapper works
 * This script tests the basic functionality without performing an actual update
 */

import { AtomUpdater, PlatformUtils } from './dist/index.js';

async function testWrapper() {
  console.log('Testing atom-updater Node.js wrapper...\n');

  // Debug: Check bundled binary path
  const platformInfo = PlatformUtils.getPlatformInfo();
  console.log('Platform info:', platformInfo);

  // Check if bundled binary exists
  const bundledPath = `/Users/mac/Github/atom-updater/atom-updater-node/bin/${platformInfo.platform}/${platformInfo.arch}/${platformInfo.executableName}`;
  console.log('Expected bundled binary path:', bundledPath);

  try {
    const fs = await import('fs');
    console.log('Bundled binary exists:', fs.existsSync(bundledPath));
  } catch (error) {
    console.log('Error checking bundled binary:', error.message);
  }

  const updater = new AtomUpdater({
    verbose: true,
    logger: console.log
  });

  try {
    // Test 1: Check if executable is available
    console.log('1. Checking if atom-updater is available...');
    const isAvailable = await updater.isAvailable();
    console.log(`   Available: ${isAvailable}`);

    if (!isAvailable) {
      console.error('❌ atom-updater executable not found!');
      console.log('\nMake sure the atom-updater executable is in your PATH or current directory.');
      return;
    }

    // Test 2: Get version
    console.log('\n2. Getting version...');
    const version = await updater.getVersion();
    console.log(`   Version: ${version}`);

    // Test 3: Get executable path
    console.log('\n3. Getting executable path...');
    const execPath = updater.getExecutablePath();
    console.log(`   Executable path: ${execPath}`);

    console.log('\n✅ All tests passed! The wrapper is working correctly.');

  } catch (error) {
    console.error('\n❌ Test failed:', error.message);
    console.error('Stack:', error.stack);
  }
}

testWrapper();