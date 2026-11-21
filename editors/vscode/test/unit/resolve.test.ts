import { createHash } from 'node:crypto';
import { createServer } from 'node:http';
import { chmod, mkdir, mkdtemp, readFile, writeFile } from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import * as tar from 'tar';
import { afterEach, describe, expect, it } from 'vitest';
import { resolveServer } from '../../src/server/resolve';
import { serverBinaryPath } from '../../src/server/paths';
import type { ServerTarget } from '../../src/server/platforms';

const target: ServerTarget = {
  platform: 'linux',
  arch: 'amd64',
  binary: 'pico-lsp',
  archive: 'tar.gz'
};

const version = '1.3.0';
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

async function serveDownload(contents: Buffer): Promise<string> {
  const checksum = createHash('sha256').update(contents).digest('hex');
  const server = createServer((request, response) => {
    if (request.url === `/${asset}`) {
      response.end(contents);
      return;
    }
    if (request.url === '/checksums.txt') {
      response.end(`${checksum}  ${asset}\n`);
      return;
    }
    response.statusCode = 404;
    response.end('not found');
  });
  servers.push(server);
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  const address = server.address();
  if (address === null || typeof address === 'string') {
    throw new Error('test server has no port');
  }
  return `http://127.0.0.1:${address.port}`;
}

describe('server resolution', () => {
  it('uses an explicit server path and disables downloads', async () => {
    const directory = await mkdtemp(path.join(os.tmpdir(), 'pico-resolve-'));
    const binary = path.join(directory, 'pico-lsp');
    await writeFile(binary, 'server', { mode: 0o755 });
    await chmod(binary, 0o755);

    await expect(
      resolveServer({
        version,
        target,
        cacheRoot: path.join(directory, 'cache'),
        serverPath: binary,
        autoDownload: false,
        development: true
      })
    ).resolves.toEqual({ path: binary, source: 'serverPath' });
  });

  it('reuses a cached binary', async () => {
    const directory = await mkdtemp(path.join(os.tmpdir(), 'pico-resolve-'));
    const cacheRoot = path.join(directory, 'cache');
    const cached = serverBinaryPath(cacheRoot, version, target);
    await mkdir(path.dirname(cached), { recursive: true });
    await writeFile(cached, 'server', { mode: 0o755 });
    await chmod(cached, 0o755);

    await expect(
      resolveServer({
        version,
        target,
        cacheRoot,
        serverPath: '',
        autoDownload: false,
        development: false
      })
    ).resolves.toEqual({ path: cached, source: 'cache' });
  });

  it('reports a downloaded binary', async () => {
    const directory = await mkdtemp(path.join(os.tmpdir(), 'pico-resolve-'));
    const contents = await archiveFixture(directory);
    const baseUrl = await serveDownload(contents);
    const cacheRoot = path.join(directory, 'cache');

    const resolved = await resolveServer({
      version,
      target,
      cacheRoot,
      serverPath: '',
      autoDownload: true,
      development: false,
      baseUrl
    });

    expect(resolved).toEqual({
      path: serverBinaryPath(cacheRoot, version, target),
      source: 'download'
    });
    expect(await readFile(resolved.path, 'utf8')).toBe('server binary');
  });

  it('requires an explicit path in development builds', async () => {
    const directory = await mkdtemp(path.join(os.tmpdir(), 'pico-resolve-'));
    await expect(
      resolveServer({
        version,
        target,
        cacheRoot: path.join(directory, 'cache'),
        serverPath: '',
        autoDownload: true,
        development: true
      })
    ).rejects.toThrow('requires pico8.serverPath');
  });

  it('reports disabled downloads when nothing is cached', async () => {
    const directory = await mkdtemp(path.join(os.tmpdir(), 'pico-resolve-'));
    await expect(
      resolveServer({
        version,
        target,
        cacheRoot: path.join(directory, 'cache'),
        serverPath: '',
        autoDownload: false,
        development: false
      })
    ).rejects.toThrow('autoDownload is disabled');
  });
});
