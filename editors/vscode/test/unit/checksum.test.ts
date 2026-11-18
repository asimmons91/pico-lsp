import { createHash } from 'node:crypto';
import { mkdtemp, writeFile } from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { describe, expect, it } from 'vitest';
import {
  parseChecksums,
  requireChecksum,
  sha256File,
  verifyFileChecksum
} from '../../src/server/checksum';

function digest(contents: string): string {
  return createHash('sha256').update(contents).digest('hex');
}

describe('checksums', () => {
  it('parses standard and escaped checksum lines', () => {
    const first = digest('first');
    const second = digest('second');
    const parsed = parseChecksums(
      `${first}  pico-lsp_1.3.0_linux_amd64.tar.gz\r\n${second} *pico-lsp_1.3.0_windows_amd64.zip\n`
    );
    expect(parsed.get('pico-lsp_1.3.0_linux_amd64.tar.gz')).toBe(first);
    expect(parsed.get('pico-lsp_1.3.0_windows_amd64.zip')).toBe(second);
  });

  it('requires an exact asset entry', () => {
    const checksum = digest('archive');
    expect(requireChecksum(`${checksum}  asset.tar.gz`, 'asset.tar.gz')).toBe(checksum);
    expect(() => requireChecksum(`${checksum}  other.tar.gz`, 'asset.tar.gz')).toThrow(
      'does not contain'
    );
    expect(() =>
      requireChecksum(`${checksum}  asset.tar.gz\n${checksum}  asset.tar.gz`, 'asset.tar.gz')
    ).toThrow('multiple entries');
  });

  it('verifies file digests', async () => {
    const directory = await mkdtemp(path.join(os.tmpdir(), 'pico-checksum-'));
    const file = path.join(directory, 'asset');
    await writeFile(file, 'contents');
    expect(await sha256File(file)).toBe(digest('contents'));
    await expect(verifyFileChecksum(file, digest('contents'))).resolves.toBeUndefined();
    await expect(verifyFileChecksum(file, digest('other'))).rejects.toThrow('checksum mismatch');
  });
});
