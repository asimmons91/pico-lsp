import { constants } from 'node:fs';
import { access, lstat } from 'node:fs/promises';
import path from 'node:path';
import type { CancellationToken } from 'vscode';
import { ensureServer, isUsableServerBinary } from './download';
import type { ServerSource } from './info';
import type { ServerTarget } from './platforms';
import { serverBinaryPath } from './paths';

export interface ResolvedServer {
  path: string;
  source: Exclude<ServerSource, 'unknown'>;
}

export interface ResolveServerOptions {
  version: string;
  target: ServerTarget;
  cacheRoot: string;
  serverPath: string;
  autoDownload: boolean;
  development: boolean;
  baseUrl?: string;
  proxy?: string;
  strictSSL?: boolean;
  token?: CancellationToken;
}

export async function resolveServer(options: ResolveServerOptions): Promise<ResolvedServer> {
  if (options.serverPath.trim()) {
    return {
      path: await validateServerPath(options.serverPath.trim()),
      source: 'serverPath'
    };
  }

  const cached = serverBinaryPath(options.cacheRoot, options.version, options.target);
  if (await isUsableServerBinary(cached)) {
    return { path: cached, source: 'cache' };
  }

  if (options.development) {
    throw new Error(
      'A development build of the PICO-8 extension requires pico8.serverPath; release server downloads are disabled.'
    );
  }
  if (!options.autoDownload) {
    throw new Error(
      'pico-lsp is not installed and pico8.autoDownload is disabled. Set pico8.serverPath or enable downloads.'
    );
  }

  const controller = new AbortController();
  const cancellation = options.token?.onCancellationRequested(() => controller.abort());
  try {
    return {
      path: await ensureServer({
        version: options.version,
        target: options.target,
        cacheRoot: options.cacheRoot,
        baseUrl: options.baseUrl,
        proxy: options.proxy,
        strictSSL: options.strictSSL,
        signal: controller.signal
      }),
      source: 'download'
    };
  } finally {
    cancellation?.dispose();
  }
}

async function validateServerPath(serverPath: string): Promise<string> {
  const resolved = path.resolve(serverPath);
  const stat = await lstat(resolved);
  if (!stat.isFile()) {
    throw new Error(`pico8.serverPath is not a regular file: ${resolved}`);
  }
  await access(resolved, process.platform === 'win32' ? constants.F_OK : constants.X_OK);
  return resolved;
}
