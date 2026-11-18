import path from 'node:path';
import type { ServerTarget } from './platforms';

export function serverCacheDirectory(
  cacheRoot: string,
  version: string,
  target: ServerTarget
): string {
  return path.join(cacheRoot, 'servers', `v${version}`, `${target.platform}-${target.arch}`);
}

export function serverBinaryPath(cacheRoot: string, version: string, target: ServerTarget): string {
  return path.join(serverCacheDirectory(cacheRoot, version, target), target.binary);
}

export function downloadDirectory(cacheRoot: string): string {
  return path.join(cacheRoot, 'downloads');
}
