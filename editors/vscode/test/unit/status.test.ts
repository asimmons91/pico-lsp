import { describe, expect, it } from 'vitest';
import { cartInfoSeverity, formatCartInfo, type CartInfo } from '../../src/statusFormat';

function info(overrides: Partial<CartInfo> = {}): CartInfo {
  return {
    tokens: 4210,
    tokenLimit: 8192,
    chars: 27433,
    charLimit: 65535,
    compressed: 12000,
    compressedLimit: 15616,
    compressedWarning: 14054,
    compressedStatus: 'ok',
    ...overrides
  };
}

describe('cart status formatting', () => {
  it('formats all budgets compactly', () => {
    expect(formatCartInfo(info())).toBe('PICO-8 4210/8192 tok · 27433/65535 chr · 11.7/15.3 KB');
  });

  it('shows compressed n/a when unavailable', () => {
    expect(formatCartInfo(info({ compressedStatus: 'unavailable' }))).toContain('compressed n/a');
  });

  it('uses warning and error thresholds', () => {
    expect(cartInfoSeverity(info())).toBe('ok');
    expect(cartInfoSeverity(info({ tokens: 7373 }))).toBe('warning');
    expect(cartInfoSeverity(info({ chars: 58982 }))).toBe('warning');
    expect(cartInfoSeverity(info({ compressedStatus: 'warning' }))).toBe('warning');
    expect(cartInfoSeverity(info({ tokens: 8193 }))).toBe('error');
    expect(cartInfoSeverity(info({ compressedStatus: 'error' }))).toBe('error');
  });
});
