import { mkdir, mkdtemp, readFile, writeFile } from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import AdmZip from 'adm-zip';
import * as tar from 'tar';
import { describe, expect, it } from 'vitest';
import { extractServerBinary, normalizeArchivePath } from '../../src/server/archive';
import type { ServerTarget } from '../../src/server/platforms';

const linuxTarget: ServerTarget = {
  platform: 'linux',
  arch: 'amd64',
  binary: 'pico-lsp',
  archive: 'tar.gz'
};

const windowsTarget: ServerTarget = {
  platform: 'windows',
  arch: 'amd64',
  binary: 'pico-lsp.exe',
  archive: 'zip'
};

describe('server archives', () => {
  it('extracts the Unix server from a tar archive', async () => {
    const directory = await mkdtemp(path.join(os.tmpdir(), 'pico-archive-'));
    const source = path.join(directory, 'source');
    await mkdir(source, { recursive: true });
    await writeFile(path.join(source, linuxTarget.binary), 'unix binary', { mode: 0o755 });
    const archive = path.join(directory, 'server.tar.gz');
    await tar.c({ gzip: true, file: archive, cwd: source }, [linuxTarget.binary]);

    const extracted = await extractServerBinary(
      archive,
      linuxTarget,
      path.join(directory, 'extract')
    );
    expect(await readFile(extracted, 'utf8')).toBe('unix binary');
  });

  it('extracts the Windows server from a zip archive', async () => {
    const directory = await mkdtemp(path.join(os.tmpdir(), 'pico-archive-'));
    const zip = new AdmZip();
    zip.addFile(windowsTarget.binary, Buffer.from('windows binary'));
    const archive = path.join(directory, 'server.zip');
    zip.writeZip(archive);

    const extracted = await extractServerBinary(
      archive,
      windowsTarget,
      path.join(directory, 'extract')
    );
    expect(await readFile(extracted, 'utf8')).toBe('windows binary');
  });

  it('rejects archives without the expected server', async () => {
    const directory = await mkdtemp(path.join(os.tmpdir(), 'pico-archive-'));
    const zip = new AdmZip();
    zip.addFile('README.md', Buffer.from('not a server'));
    const archive = path.join(directory, 'server.zip');
    zip.writeZip(archive);

    await expect(
      extractServerBinary(archive, windowsTarget, path.join(directory, 'extract'))
    ).rejects.toThrow('does not contain');
  });

  it('normalizes archive paths', () => {
    expect(normalizeArchivePath('.\\pico-lsp')).toBe('pico-lsp');
    expect(normalizeArchivePath('./pico-lsp')).toBe('pico-lsp');
  });
});
