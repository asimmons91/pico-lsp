export type PicoPlatform = 'linux' | 'darwin' | 'windows';
export type PicoArch = 'amd64' | 'arm64';

export interface ServerTarget {
  platform: PicoPlatform;
  arch: PicoArch;
  binary: string;
  archive: 'tar.gz' | 'zip';
}

export function targetFor(platform: NodeJS.Platform, arch: string): ServerTarget {
  const mappedPlatform = mapPlatform(platform);
  const mappedArch = mapArch(arch);
  return {
    platform: mappedPlatform,
    arch: mappedArch,
    binary: mappedPlatform === 'windows' ? 'pico-lsp.exe' : 'pico-lsp',
    archive: mappedPlatform === 'windows' ? 'zip' : 'tar.gz'
  };
}

function mapPlatform(platform: NodeJS.Platform): PicoPlatform {
  switch (platform) {
    case 'linux':
      return 'linux';
    case 'darwin':
      return 'darwin';
    case 'win32':
      return 'windows';
    default:
      throw new Error(`pico-lsp does not support platform ${platform}`);
  }
}

function mapArch(arch: string): PicoArch {
  switch (arch) {
    case 'x64':
      return 'amd64';
    case 'arm64':
      return 'arm64';
    default:
      throw new Error(`pico-lsp does not support architecture ${arch}`);
  }
}

export function assetName(version: string, target: ServerTarget): string {
  return `pico-lsp_${version}_${target.platform}_${target.arch}.${target.archive}`;
}

export function releaseBaseUrl(version: string): string {
  return `https://github.com/asimmons91/pico-lsp/releases/download/v${version}`;
}
