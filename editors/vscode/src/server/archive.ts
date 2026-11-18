import { chmod, lstat, mkdir, rm, writeFile } from 'node:fs/promises';
import path from 'node:path';
import AdmZip from 'adm-zip';
import * as tar from 'tar';
import type { ServerTarget } from './platforms';

export async function extractServerBinary(
  archivePath: string,
  target: ServerTarget,
  destinationDirectory: string
): Promise<string> {
  await mkdir(destinationDirectory, { recursive: true });
  const destination = path.join(destinationDirectory, target.binary);

  if (target.archive === 'tar.gz') {
    let found = false;
    await tar.t({
      file: archivePath,
      onentry: (entry) => {
        const entryPath = normalizeArchivePath(entry.path);
        if (entryPath === target.binary) {
          found = true;
        }
        if (isUnsafePath(entry.path)) {
          throw new Error(`unsafe path in server archive: ${entry.path}`);
        }
      }
    });
    if (!found) {
      throw new Error(`server archive does not contain ${target.binary}`);
    }
    await tar.x({
      file: archivePath,
      cwd: destinationDirectory,
      strict: true,
      preservePaths: false,
      filter: (entryPath) => normalizeArchivePath(entryPath) === target.binary
    });
  } else {
    const zip = new AdmZip(archivePath);
    const entry = zip
      .getEntries()
      .find((candidate) => normalizeArchivePath(candidate.entryName) === target.binary);
    if (!entry || entry.isDirectory) {
      throw new Error(`server archive does not contain ${target.binary}`);
    }
    const data = zip.readFile(entry);
    if (!data) {
      throw new Error(`unable to read ${target.binary} from server archive`);
    }
    await writeFile(destination, data, { mode: 0o755 });
  }

  const stat = await lstat(destination);
  if (!stat.isFile()) {
    await rm(destination, { force: true });
    throw new Error(`server archive entry ${target.binary} is not a regular file`);
  }
  if (process.platform !== 'win32') {
    await chmod(destination, 0o755);
  }
  return destination;
}

export function normalizeArchivePath(entryPath: string): string {
  return entryPath.replaceAll('\\', '/').replace(/^\.\//, '');
}

function isUnsafePath(entryPath: string): boolean {
  const normalized = normalizeArchivePath(entryPath);
  return normalized.startsWith('/') || normalized.split('/').includes('..');
}
