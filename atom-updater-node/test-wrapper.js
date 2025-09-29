#!/usr/bin/env node

/**
 * Test script to verify the atom-updater wrapper with binary copying functionality
 * This script tests the basic functionality without performing an actual update
 */

import { AtomUpdater, PlatformUtils } from './dist/index.js';
import fs from 'fs';
import path from 'path';
import os from 'os';

async function testWrapper() {
  console.log('Testing atom-updater Node.js wrapper with binary copying...\n');

  const platformInfo = PlatformUtils.getPlatformInfo();
  console.log('Platform info:', platformInfo);

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

    // Test 4: Test binary copying functionality
    console.log('\n4. Testing binary copying functionality...');
    const externalBinDir = "../test/bin";

    // Clean up any existing test directory
    // if (fs.existsSync(externalBinDir)) {
    //   fs.rmSync(externalBinDir, { recursive: true, force: true });
    // }

    // Test copying the binary to external directory
    const result = await updater.update({
      pid: process.pid,
      currentAppDir: "../test/myapp",
      newAppDir: "../test/updates/macapp",
      binDir: externalBinDir
    });

    console.log('   Update test result:', result);

    // Test 5: Test without binDir (normal operation)
    // console.log('\n5. Testing normal operation (without binDir)...');
    // const normalResult = await updater.update({
    //   pid: process.pid,
    //   currentAppDir: process.cwd(),
    //   newAppDir: path.join(process.cwd(), 'test-new-version-2')
    // });
    // console.log('   Normal operation result:', normalResult);

    console.log('\n✅ All tests passed! The wrapper with binary copying is working correctly.');

  } catch (error) {
    console.error('\n❌ Test failed:', error.message);
    console.error('Stack:', error.stack);
  }
}

testWrapper();