import { execSync } from 'child_process';

export function checkPrereqs(): { ollama: boolean; git: boolean } {
  let ollama = false;
  let git = false;

  try {
    execSync('ollama --version', { stdio: 'ignore' });
    ollama = true;
  } catch {
    ollama = false;
  }

  try {
    execSync('git --version', { stdio: 'ignore' });
    git = true;
  } catch {
    git = false;
  }

  return { ollama, git };
}
