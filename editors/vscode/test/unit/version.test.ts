import { describe, expect, it } from 'vitest';
import {
  classifyServerVersion,
  isReleaseVersion,
  normalizeVersion
} from '../../src/server/version';

describe('server version normalization', () => {
  it('trims whitespace and strips one leading v', () => {
    expect(normalizeVersion('v1.7.0')).toBe('1.7.0');
    expect(normalizeVersion('V1.7.0')).toBe('1.7.0');
    expect(normalizeVersion('  v1.7.0  ')).toBe('1.7.0');
    expect(normalizeVersion('vv1.7.0')).toBe('v1.7.0');
    expect(normalizeVersion('')).toBe('');
  });

  it('accepts only exact release versions', () => {
    expect(isReleaseVersion('1.7.0')).toBe(true);
    expect(isReleaseVersion('v1.7.0')).toBe(true);
    expect(isReleaseVersion('V1.7.0')).toBe(true);
    expect(isReleaseVersion(' 1.7.0 ')).toBe(true);
    expect(isReleaseVersion('1.7.0-rc.1')).toBe(false);
    expect(isReleaseVersion('1.7.0+build')).toBe(false);
    expect(isReleaseVersion('1.7')).toBe(false);
    expect(isReleaseVersion('1.7.0.0')).toBe(false);
    expect(isReleaseVersion('dev')).toBe(false);
    expect(isReleaseVersion('v0.0.0-20261121123456-abcdefabcdef')).toBe(false);
    expect(isReleaseVersion('')).toBe(false);
    expect(isReleaseVersion('v')).toBe(false);
  });
});

describe('server version classification', () => {
  it('matches release versions across the v-prefix difference', () => {
    expect(classifyServerVersion('1.7.0', 'v1.7.0')).toBe('match');
    expect(classifyServerVersion('1.7.0', 'V1.7.0')).toBe('match');
    expect(classifyServerVersion('1.7.0 ', ' v1.7.0 ')).toBe('match');
  });

  it('reports differing release versions as a mismatch', () => {
    expect(classifyServerVersion('1.8.0', 'v1.7.0')).toBe('mismatch');
    expect(classifyServerVersion('1.7.0', '1.7.1')).toBe('mismatch');
  });

  it('treats non-release server versions as development', () => {
    expect(classifyServerVersion('1.7.0', '1.7.0-rc.1')).toBe('development');
    expect(classifyServerVersion('1.7.0', '1.7.0+build')).toBe('development');
    expect(classifyServerVersion('1.7.0', 'dev')).toBe('development');
    expect(classifyServerVersion('1.7.0', '')).toBe('development');
    expect(classifyServerVersion('1.7.0', 'v0.0.0-20261121123456-abcdefabcdef')).toBe(
      'development'
    );
  });

  it('treats a non-release extension version as development', () => {
    expect(classifyServerVersion('1.8.0-dev', 'v1.7.0')).toBe('development');
    expect(classifyServerVersion('dev', 'v1.7.0')).toBe('development');
  });

  it('reports a missing server version as unknown', () => {
    expect(classifyServerVersion('1.7.0', undefined)).toBe('unknown');
    expect(classifyServerVersion('1.8.0-dev', undefined)).toBe('unknown');
  });
});
