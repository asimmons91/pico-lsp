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
  contributes?: {
    languages?: LanguageContribution[];
  };
}

const extensionRoot = path.resolve(__dirname, '../..');

describe('PICO-8 file icon', () => {
  it('declares an existing PNG for the pico8 language', () => {
    const manifest = JSON.parse(readFileSync(path.join(extensionRoot, 'package.json'), 'utf8')) as Manifest;
    const language = manifest.contributes?.languages?.find((entry) => entry.id === 'pico8');
    expect(language?.icon?.light).toBeTruthy();
    expect(language?.icon?.dark).toBeTruthy();

    for (const relativePath of [language?.icon?.light, language?.icon?.dark]) {
      expect(relativePath).toBeTruthy();
      const iconPath = path.join(extensionRoot, relativePath!);
      expect(existsSync(iconPath)).toBe(true);
      expect(readFileSync(iconPath).subarray(1, 4).toString('ascii')).toBe('PNG');
    }
  });
});
