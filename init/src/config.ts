import * as fs from 'fs';
import * as path from 'path';

export const defaultConfigTemplate = {
  $schema: 'https://nano-guard.dev/schema/config.json',
  model: 'qwen2.5-coder:7b',
  ollama_host: 'http://localhost:11434',
  timeout_seconds: 30,
  max_diff_lines: 200,
  fail_open: true,
  rules: {
    unhandled_errors: true,
    debug_logs: true,
    type_safety: true,
    placeholder_stubs: true,
  },
  ignore_paths: [
    '**/*.test.ts',
    '**/*.spec.go',
    '**/vendor/**',
    '**/node_modules/**',
  ],
};

export function writeConfig(targetDir: string = process.cwd()): string {
  const configPath = path.join(targetDir, 'nano-guard.config.json');
  if (!fs.existsSync(configPath)) {
    fs.writeFileSync(configPath, JSON.stringify(defaultConfigTemplate, null, 2), 'utf-8');
  }
  return configPath;
}
