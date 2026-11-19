import { describe, expect, it } from 'vitest';
import {
  cartInfoSeverity,
  formatCartInfo,
  formatCartTooltip,
  type CartInfo
} from '../../src/statusFormat';

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

describe('cart status tooltip', () => {
  it('renders cart and file paths through the workspace resolver', () => {
    const markdown = formatCartTooltip(
      info({
        rootUri: 'file:///workspace/cart.p8',
        files: [
          { uri: 'file:///workspace/lib.lua', tokens: 12, chars: 34, bytes: 56 },
          { uri: 'file:///workspace/objects/ship.lua', tokens: 78, chars: 90, bytes: 12 }
        ]
      }),
      (value) => value.replace('file:///workspace/', '')
    );

    expect(markdown).toContain('Cart: `cart.p8`');
    expect(markdown).toContain('| `lib.lua` | 12 | 34 |');
    expect(markdown).toContain('| `objects/ship.lua` | 78 | 90 |');
    expect(markdown).not.toContain('/workspace');
  });
});
