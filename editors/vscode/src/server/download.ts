import { constants, createWriteStream } from 'node:fs';
import { access, chmod, lstat, mkdir, mkdtemp, rename, rm } from 'node:fs/promises';
import path from 'node:path';
import { Readable } from 'node:stream';
import { pipeline } from 'node:stream/promises';
import { Agent, ProxyAgent, fetch, type Dispatcher } from 'undici';
import { extractServerBinary } from './archive';
import { requireChecksum, verifyFileChecksum } from './checksum';
import { assetName, releaseBaseUrl, type ServerTarget } from './platforms';
import { downloadDirectory, serverBinaryPath, serverCacheDirectory } from './paths';

export interface DownloadServerOptions {
  version: string;
  target: ServerTarget;
  cacheRoot: string;
  baseUrl?: string;
  proxy?: string;
  strictSSL?: boolean;
  signal?: AbortSignal;
}

const activeDownloads = new Map<string, Promise<string>>();

export async function ensureServer(options: DownloadServerOptions): Promise<string> {
  const cached = serverBinaryPath(options.cacheRoot, options.version, options.target);
  if (await isUsableServerBinary(cached)) {
    return cached;
  }

  const key = `${options.cacheRoot}:${options.version}:${options.target.platform}:${options.target.arch}`;
  const existing = activeDownloads.get(key);
  if (existing) {
    return existing;
  }

  const download = downloadServer(options).finally(() => {
    activeDownloads.delete(key);
  });
  activeDownloads.set(key, download);
  return download;
}

async function downloadServer(options: DownloadServerOptions): Promise<string> {
  const baseUrl = (options.baseUrl ?? releaseBaseUrl(options.version)).replace(/\/$/, '');
  const asset = assetName(options.version, options.target);
  const probe = makeDispatcher(options);

  try {
    const checksums = await fetchText(`${baseUrl}/checksums.txt`, probe, options.signal);
    const expected = requireChecksum(checksums, asset);

    const downloads = downloadDirectory(options.cacheRoot);
    await mkdir(downloads, { recursive: true });
    const temporaryDirectory = await mkdtemp(path.join(downloads, 'pico-lsp-'));
    const archivePath = path.join(temporaryDirectory, asset);

    try {
      await downloadFile(`${baseUrl}/${asset}`, archivePath, probe, options.signal);
      await verifyFileChecksum(archivePath, expected);

      const extractedDirectory = path.join(temporaryDirectory, 'extract');
      const extracted = await extractServerBinary(archivePath, options.target, extractedDirectory);
      const finalDirectory = serverCacheDirectory(
        options.cacheRoot,
        options.version,
        options.target
      );
      const finalPath = serverBinaryPath(options.cacheRoot, options.version, options.target);

      await mkdir(finalDirectory, { recursive: true });
      await rm(finalPath, { force: true });
      await rename(extracted, finalPath);
      if (process.platform !== 'win32') {
        await chmod(finalPath, 0o755);
      }
      return finalPath;
    } finally {
      await rm(temporaryDirectory, { recursive: true, force: true });
    }
  } finally {
    await closeDispatcher(probe);
  }
}

function makeDispatcher(options: DownloadServerOptions): Dispatcher {
  if (options.proxy) {
    return new ProxyAgent({
      uri: options.proxy,
      requestTls: { rejectUnauthorized: options.strictSSL !== false }
    });
  }
  if (options.strictSSL === false) {
    return new Agent({ connect: { rejectUnauthorized: false } });
  }
  return new Agent();
}

async function closeDispatcher(dispatcher: Dispatcher): Promise<void> {
  if ('close' in dispatcher && typeof dispatcher.close === 'function') {
    await dispatcher.close();
  }
}

async function fetchText(
  url: string,
  dispatcher: Dispatcher,
  signal?: AbortSignal
): Promise<string> {
  const response = await fetch(url, { dispatcher, signal });
  if (!response.ok) {
    throw new Error(`unable to download ${url}: HTTP ${response.status}`);
  }
  return response.text();
}

async function downloadFile(
  url: string,
  destination: string,
  dispatcher: Dispatcher,
  signal?: AbortSignal
): Promise<void> {
  const response = await fetch(url, { dispatcher, signal });
  if (!response.ok || !response.body) {
    throw new Error(`unable to download ${url}: HTTP ${response.status}`);
  }
  await pipeline(Readable.fromWeb(response.body as never), createWriteStream(destination));
}

export async function isUsableServerBinary(file: string): Promise<boolean> {
  try {
    const stat = await lstat(file);
    if (!stat.isFile()) {
      return false;
    }
    await access(file, process.platform === 'win32' ? constants.F_OK : constants.X_OK);
    return true;
  } catch {
    return false;
  }
}

export async function clearServerCache(cacheRoot: string, version: string): Promise<void> {
  await rm(path.join(cacheRoot, 'servers', `v${version}`), { recursive: true, force: true });
}
