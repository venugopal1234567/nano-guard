#!/usr/bin/env node
import { checkPrereqs } from './prereqs.js';
import { pullModel } from './model.js';
import { installBinary } from './binary.js';
import { writeConfig } from './config.js';
import { patchSettings } from './settings.js';
import * as path from 'path';

async function runInit() {
  console.log('🛡️  Nano-Guard Init\n');

  // 1. Prerequisites check
  console.log('  Checking prerequisites...');
  const prereqs = checkPrereqs();
  if (!prereqs.ollama) {
    console.error('❌ Ollama is not installed or not in PATH.');
    console.error('Please install Ollama from https://ollama.com before continuing.');
    process.exit(1);
  }
  console.log('  ✅ Ollama found');
  if (!prereqs.git) {
    console.warn('  ⚠️ git is not found in PATH (will fallback to direct file evaluation)');
  } else {
    console.log('  ✅ git found');
  }

  const modelName = 'qwen2.5-coder:7b';

  // 2. Pull local LLM model
  console.log(`\n  Pulling model: ${modelName}`);
  pullModel(modelName);
  console.log('  ✅ Model ready');

  // 3. Build / Install binary
  console.log('\n  Installing nano-guard binary...');
  const projectRoot = path.resolve(__dirname, '../..');
  const binaryPath = await installBinary(projectRoot);
  console.log(`  ✅ Installed at ${binaryPath}`);

  // 4. Write nano-guard.config.json
  console.log('\n  Writing config...');
  const configPath = writeConfig(process.cwd());
  console.log('  ✅ nano-guard.config.json created');

  // 5. Patch .claude/settings.json
  console.log('\n  Patching .claude/settings.json...');
  const settingsPath = patchSettings(process.cwd(), binaryPath);
  console.log('  ✅ PostToolUse hook registered');

  console.log('\n──────────────────────────────────────');
  console.log('  Nano-Guard is active. Every file edit');
  console.log('  will now be verified locally by');
  console.log(`  ${modelName} before your agent`);
  console.log('  continues. Zero cloud tokens used.');
  console.log('──────────────────────────────────────');
}

runInit().catch((err) => {
  console.error('\n❌ Initialization failed:', err.message);
  process.exit(1);
});
