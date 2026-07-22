import { execSync } from 'child_process';
import * as fs from 'fs';
import * as path from 'path';
import * as os from 'os';
import * as https from 'https';

export async function installBinary(projectRoot: string): Promise<string> {
  const localBin = path.join(os.homedir(), '.local', 'bin');
  if (!fs.existsSync(localBin)) {
    fs.mkdirSync(localBin, { recursive: true });
  }

  const targetPath = path.join(localBin, 'nano-guard');

  // Attempt to download pre-built release binary if available
  const version = 'v0.1.0';
  const platform = os.platform(); // 'linux', 'darwin', 'win32'
  const arch = os.arch(); // 'x64', 'arm64'
  const downloadUrl = `https://github.com/venugopal1234567/nano-guard/releases/download/${version}/nano-guard-${platform}-${arch}`;

  const downloaded = await downloadFile(downloadUrl, targetPath).catch(() => false);
  if (downloaded) {
    fs.chmodSync(targetPath, 0o755);
    return targetPath;
  }

  // Fallback: build Go binary from local source
  try {
    execSync(`go build -o "${targetPath}" ./cmd/nano-guard`, {
      cwd: projectRoot,
      stdio: 'inherit',
    });
    fs.chmodSync(targetPath, 0o755);
  } catch (err) {
    throw new Error(`Failed to build nano-guard Go binary: ${(err as Error).message}`);
  }

  return targetPath;
}

function downloadFile(url: string, dest: string): Promise<boolean> {
  return new Promise((resolve, reject) => {
    const file = fs.createWriteStream(dest);
    https.get(url, (response) => {
      if (response.statusCode === 200) {
        response.pipe(file);
        file.on('finish', () => {
          file.close();
          resolve(true);
        });
      } else {
        file.close();
        fs.unlink(dest, () => {});
        reject(false);
      }
    }).on('error', (err) => {
      fs.unlink(dest, () => {});
      reject(err);
    });
  });
}
