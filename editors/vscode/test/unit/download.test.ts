import { createHash } from 'node:crypto';
import { createServer } from 'node:http';
import { mkdir, mkdtemp, readFile, writeFile } from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import * as tar from 'tar';
import { afterEach, describe, expect, it } from 'vitest';
import { clearServerCache, ensureServer } from '../../src/server/download';
import { serverBinaryPath } from '../../src/server/paths';
import type { ServerTarget } from '../../src/server/platforms';

const version = '1.3.0';
const target: ServerTarget = {
  platform: 'linux',
  arch: 'amd64',
  binary: 'pico-lsp',
  archive: 'tar.gz'
};
const asset = `pico-lsp_${version}_${target.platform}_${target.arch}.tar.gz`;

const servers: ReturnType<typeof createServer>[] = [];

afterEach(async () => {
  await Promise.all(
    servers.splice(0).map(
      (server) =>
        new Promise<void>((resolve) => {
          server.close(() => resolve());
          server.closeAllConnections();
        })
    )
  );
});

async function archiveFixture(directory: string): Promise<Buffer> {
  const source = path.join(directory, 'source');
  await mkdir(source, { recursive: true });
  await writeFile(path.join(source, target.binary), 'server binary', { mode: 0o755 });
  const archive = path.join(directory, asset);
  await tar.c({ gzip: true, file: archive, cwd: source }, [target.binary]);
  return readFile(archive);
}

function checksumsFor(contents: Buffer): string {
  const digest = createHash('sha256').update(contents).digest('hex');
  return `${digest}  ${asset}\n`;
}

async function serve(files: Record<string, Buffer | string>): Promise<string> {
  const server = createServer((request, response) => {
    const body = files[request.url ?? ''];
    if (body === undefined) {
      response.statusCode = 404;
      response.end('not found');
      return;
    }
    response.end(body);
  });
  servers.push(server);
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  const address = server.address();
  if (address === null || typeof address === 'string') {
    throw new Error('test server has no port');
  }
  return `http://127.0.0.1:${address.port}`;
}

describe('server download', () => {
  it('downloads, verifies, caches, and reuses the server', async () => {
    const directory = await mkdtemp(path.join(os.tmpdir(), 'pico-download-'));
    const contents = await archiveFixture(directory);
    let assetRequests = 0;
    const server = createServer((request, response) => {
      if (request.url === `/${asset}`) {
        assetRequests++;
        response.end(contents);
        return;
      }
      if (request.url === '/checksums.txt') {
        response.end(checksumsFor(contents));
        return;
      }
      response.statusCode = 404;
      response.end();
    });
    servers.push(server);
    await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
    const address = server.address();
    if (address === null || typeof address === 'string') {
      throw new Error('test server has no port');
    }
    const baseUrl = `http://127.0.0.1:${address.port}`;
    const cacheRoot = path.join(directory, 'cache');

    const first = await ensureServer({ version, target, cacheRoot, baseUrl });
    expect(await readFile(first, 'utf8')).toBe('server binary');
    expect(first).toBe(serverBinaryPath(cacheRoot, version, target));
    expect(assetRequests).toBe(1);

    const second = await ensureServer({ version, target, cacheRoot, baseUrl });
    expect(second).toBe(first);
    expect(assetRequests).toBe(1);
  });

  it('rejects checksum mismatches', async () => {
    const directory = await mkdtemp(path.join(os.tmpdir(), 'pico-download-'));
    const contents = await archiveFixture(directory);
    const baseUrl = await serve({
      '/checksums.txt': `${'0'.repeat(64)}  ${asset}\n`,
      [`/${asset}`]: contents
    });

    await expect(
      ensureServer({ version, target, cacheRoot: path.join(directory, 'cache'), baseUrl })
    ).rejects.toThrow('checksum mismatch');
  });

  it('rejects missing checksum entries and assets', async () => {
    const directory = await mkdtemp(path.join(os.tmpdir(), 'pico-download-'));
    const contents = await archiveFixture(directory);
    const missingChecksum = await serve({
      '/checksums.txt': `${'0'.repeat(64)}  other.tar.gz\n`,
      [`/${asset}`]: contents
    });
    await expect(
      ensureServer({ version, target, cacheRoot: path.join(directory, 'cache'), baseUrl: missingChecksum })
    ).rejects.toThrow('does not contain');

    const missingAsset = await serve({
      '/checksums.txt': checksumsFor(contents)
    });
    await expect(
      ensureServer({ version, target, cacheRoot: path.join(directory, 'cache'), baseUrl: missingAsset })
    ).rejects.toThrow('HTTP 404');
  });

  it('clears the versioned cache', async () => {
    const directory = await mkdtemp(path.join(os.tmpdir(), 'pico-download-'));
    const contents = await archiveFixture(directory);
    const baseUrl = await serve({
      '/checksums.txt': checksumsFor(contents),
      [`/${asset}`]: contents
    });
    const cacheRoot = path.join(directory, 'cache');
    const binary = await ensureServer({ version, target, cacheRoot, baseUrl });
    await clearServerCache(cacheRoot, version);
    await expect(readFile(binary)).rejects.toThrow();
  });
});
