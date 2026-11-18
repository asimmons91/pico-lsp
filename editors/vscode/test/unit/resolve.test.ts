import { chmod, mkdtemp, writeFile } from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { describe, expect, it } from 'vitest';
import { resolveServer } from '../../src/server/resolve';
import type { ServerTarget } from '../../src/server/platforms';

const target: ServerTarget = {
  platform: 'linux',
  arch: 'amd64',
  binary: 'pico-lsp',
  archive: 'tar.gz'
};

describe('server resolution', () => {
  it('uses an explicit server path and disables downloads', async () => {
    const directory = await mkdtemp(path.join(os.tmpdir(), 'pico-resolve-'));
    const binary = path.join(directory, 'pico-lsp');
    await writeFile(binary, 'server', { mode: 0o755 });
    await chmod(binary, 0o755);

    await expect(
      resolveServer({
        version: '1.3.0',
        target,
        cacheRoot: path.join(directory, 'cache'),
        serverPath: binary,
        autoDownload: false,
        development: true
      })
    ).resolves.toBe(binary);
  });

  it('requires an explicit path in development builds', async () => {
    const directory = await mkdtemp(path.join(os.tmpdir(), 'pico-resolve-'));
    await expect(
      resolveServer({
        version: '1.3.0',
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
        version: '1.3.0',
        target,
        cacheRoot: path.join(directory, 'cache'),
        serverPath: '',
        autoDownload: false,
        development: false
      })
    ).rejects.toThrow('autoDownload is disabled');
  });
});
