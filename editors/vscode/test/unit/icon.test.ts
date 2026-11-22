import { existsSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';

interface LanguageContribution {
  id: string;
  icon?: {
    light?: string;
    dark?: string;
  };
}

interface Manifest {
  icon?: string;
  contributes?: {
    languages?: LanguageContribution[];
  };
}

const extensionRoot = path.resolve(__dirname, '../..');

function expectPng(iconPath: string): Buffer {
  expect(existsSync(iconPath)).toBe(true);
  const data = readFileSync(iconPath);
  expect(data.subarray(1, 4).toString('ascii')).toBe('PNG');
  return data;
}

describe('Extension listing icon', () => {
  it('declares a 128x128 PNG listing icon', () => {
    const manifest = JSON.parse(readFileSync(path.join(extensionRoot, 'package.json'), 'utf8')) as Manifest;
    expect(manifest.icon).toBeTruthy();

    const data = expectPng(path.join(extensionRoot, manifest.icon!));
    expect(data.readUInt32BE(16)).toBe(128);
    expect(data.readUInt32BE(20)).toBe(128);
  });

  it('does not package the official PICO-8 logo', () => {
    expect(existsSync(path.join(extensionRoot, 'icons/pico8.png'))).toBe(false);
  });
});

describe('PICO-8 file icon', () => {
  it('declares an existing PNG for the pico8 language', () => {
    const manifest = JSON.parse(readFileSync(path.join(extensionRoot, 'package.json'), 'utf8')) as Manifest;
    const language = manifest.contributes?.languages?.find((entry) => entry.id === 'pico8');
    expect(language?.icon?.light).toBeTruthy();
    expect(language?.icon?.dark).toBeTruthy();

    for (const relativePath of [language?.icon?.light, language?.icon?.dark]) {
      expect(relativePath).toBeTruthy();
      expectPng(path.join(extensionRoot, relativePath!));
    }
  });
});
