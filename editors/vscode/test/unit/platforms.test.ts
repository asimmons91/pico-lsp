import { describe, expect, it } from 'vitest';
import { assetName, releaseBaseUrl, targetFor } from '../../src/server/platforms';

describe('server platforms', () => {
  it.each([
    ['linux', 'x64', 'linux', 'amd64', 'pico-lsp', 'tar.gz'],
    ['linux', 'arm64', 'linux', 'arm64', 'pico-lsp', 'tar.gz'],
    ['darwin', 'x64', 'darwin', 'amd64', 'pico-lsp', 'tar.gz'],
    ['darwin', 'arm64', 'darwin', 'arm64', 'pico-lsp', 'tar.gz'],
    ['win32', 'x64', 'windows', 'amd64', 'pico-lsp.exe', 'zip'],
    ['win32', 'arm64', 'windows', 'arm64', 'pico-lsp.exe', 'zip']
  ] as const)(
    'maps %s/%s',
    (platform, arch, wantPlatform, wantArch, wantBinary, wantArchive) => {
      const target = targetFor(platform as NodeJS.Platform, arch);
      expect(target).toEqual({
        platform: wantPlatform,
        arch: wantArch,
        binary: wantBinary,
        archive: wantArchive
      });
    }
  );

  it('builds release asset names', () => {
    const target = targetFor('linux', 'x64');
    expect(assetName('1.3.0', target)).toBe('pico-lsp_1.3.0_linux_amd64.tar.gz');
    expect(releaseBaseUrl('1.3.0')).toBe(
      'https://github.com/asimmons91/pico-lsp/releases/download/v1.3.0'
    );
  });

  it('rejects unsupported platforms and architectures', () => {
    expect(() => targetFor('freebsd' as NodeJS.Platform, 'x64')).toThrow('platform');
    expect(() => targetFor('linux', 'ia32')).toThrow('architecture');
  });
});
