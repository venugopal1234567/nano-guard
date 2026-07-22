import { patchSettings } from '../settings';
import * as fs from 'fs';
import * as path from 'path';
import * as os from 'os';

describe('patchSettings', () => {
  let tempDir: string;

  beforeEach(() => {
    tempDir = fs.mkdtempSync(path.join(os.tmpdir(), 'nano-guard-test-'));
  });

  afterEach(() => {
    fs.rmSync(tempDir, { recursive: true, force: true });
  });

  it('should create .claude/settings.json if it does not exist', () => {
    const settingsPath = patchSettings(tempDir, '/usr/local/bin/nano-guard');
    expect(fs.existsSync(settingsPath)).toBe(true);

    const content = JSON.parse(fs.readFileSync(settingsPath, 'utf-8'));
    expect(content.hooks?.PostToolUse).toHaveLength(1);
    expect(content.hooks.PostToolUse[0].hooks[0].command).toBe('/usr/local/bin/nano-guard');
  });

  it('should preserve existing settings when patching', () => {
    const claudeDir = path.join(tempDir, '.claude');
    fs.mkdirSync(claudeDir, { recursive: true });
    const settingsPath = path.join(claudeDir, 'settings.json');

    const initialSettings = {
      otherKey: 'otherValue',
      hooks: {
        PreToolUse: [],
      },
    };
    fs.writeFileSync(settingsPath, JSON.stringify(initialSettings, null, 2));

    patchSettings(tempDir, '/usr/local/bin/nano-guard');

    const updated = JSON.parse(fs.readFileSync(settingsPath, 'utf-8'));
    expect(updated.otherKey).toBe('otherValue');
    expect(updated.hooks.PreToolUse).toBeDefined();
    expect(updated.hooks.PostToolUse).toHaveLength(1);
  });
});
