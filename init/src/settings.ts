import * as fs from 'fs';
import * as path from 'path';

export interface HookEntry {
  type: string;
  command: string;
  timeout: number;
}

export interface MatcherGroup {
  matcher: string;
  hooks: HookEntry[];
}

export interface Settings {
  hooks?: {
    PostToolUse?: MatcherGroup[];
    [key: string]: any;
  };
  [key: string]: any;
}

export function patchSettings(projectRoot: string = process.cwd(), binaryPath: string = '$HOME/.local/bin/nano-guard'): string {
  const claudeDir = path.join(projectRoot, '.claude');
  if (!fs.existsSync(claudeDir)) {
    fs.mkdirSync(claudeDir, { recursive: true });
  }

  const settingsPath = path.join(claudeDir, 'settings.json');
  let settings: Settings = {};

  if (fs.existsSync(settingsPath)) {
    try {
      const content = fs.readFileSync(settingsPath, 'utf-8');
      settings = JSON.parse(content);
    } catch {
      settings = {};
    }
  }

  if (!settings.hooks) {
    settings.hooks = {};
  }
  if (!settings.hooks.PostToolUse) {
    settings.hooks.PostToolUse = [];
  }

  const matcherPattern = 'write_to_file|replace_file_content|multi_replace_file_content|Write|Edit';
  let existingGroup = settings.hooks.PostToolUse.find((g) => g.matcher === matcherPattern);

  const nanoGuardHook: HookEntry = {
    type: 'command',
    command: binaryPath,
    timeout: 30,
  };

  if (!existingGroup) {
    existingGroup = {
      matcher: matcherPattern,
      hooks: [nanoGuardHook],
    };
    settings.hooks.PostToolUse.push(existingGroup);
  } else {
    const alreadyInstalled = existingGroup.hooks.some((h) => h.command === binaryPath);
    if (!alreadyInstalled) {
      existingGroup.hooks.push(nanoGuardHook);
    }
  }

  fs.writeFileSync(settingsPath, JSON.stringify(settings, null, 2), 'utf-8');

  // Also setup Antigravity 2.0 workspace rules (.agents/AGENTS.md)
  const agentsDir = path.join(projectRoot, '.agents');
  if (!fs.existsSync(agentsDir)) {
    fs.mkdirSync(agentsDir, { recursive: true });
  }

  const agentsMdPath = path.join(agentsDir, 'AGENTS.md');
  const nanoGuardRule = `\n\n## Nano-Guard Post-Tool-Use Verification\nAfter executing any file modification tool (write_to_file, replace_file_content, multi_replace_file_content), you MUST verify the code change by running this terminal command:\n\`\`\`bash\necho '{"hook_event_name":"PostToolUse","tool_name":"replace_file_content","tool_input":{"TargetFile":"<edited_file_path>"},"cwd":"${projectRoot}"}' | ${binaryPath}\n\`\`\`\nIf the command outputs errors (Exit Code 2), you MUST fix the issues immediately.\n`;

  let existingAgentsContent = '';
  if (fs.existsSync(agentsMdPath)) {
    existingAgentsContent = fs.readFileSync(agentsMdPath, 'utf-8');
  }

  if (!existingAgentsContent.includes('Nano-Guard Post-Tool-Use Verification')) {
    fs.appendFileSync(agentsMdPath, nanoGuardRule, 'utf-8');
  }

  return settingsPath;
}
