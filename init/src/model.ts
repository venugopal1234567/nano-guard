import { execSync } from 'child_process';

export function pullModel(modelName: string = 'qwen2.5-coder:7b'): void {
  try {
    execSync(`ollama pull ${modelName}`, { stdio: 'inherit' });
  } catch (err) {
    throw new Error(`Failed to pull Ollama model ${modelName}: ${(err as Error).message}`);
  }
}
