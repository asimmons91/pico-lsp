import { describe, expect, it } from 'vitest';
import {
  formatServerDiagnostics,
  formatServerInfo,
  serverVersionWarning,
  type ServerDetails
} from '../../src/server/info';

function details(overrides: Partial<ServerDetails> = {}): ServerDetails {
  return {
    extensionVersion: '1.7.0',
    serverVersion: 'v1.7.0',
    versionStatus: 'match',
    source: 'serverPath',
    serverPath: '/usr/local/bin/pico-lsp',
    platform: 'linux',
    arch: 'amd64',
    cacheRoot: '/home/user/.config/Code/User/globalStorage/asimmons91.pico-lsp',
    workspaceFolders: 2,
    attachLua: true,
    running: true,
    ...overrides
  };
}

describe('server info formatting', () => {
  it('formats a running custom-path server', () => {
    expect(formatServerInfo(details())).toBe(
      [
        'PICO-8 language server',
        '',
        'Extension version: 1.7.0',
        'Server: v1.7.0',
        'Version status: match',
        'Source: custom path (pico8.serverPath)',
        'Server path: /usr/local/bin/pico-lsp',
        'Platform: linux amd64',
        'Cache root: /home/user/.config/Code/User/globalStorage/asimmons91.pico-lsp',
        'Workspace folders: 2',
        'Lua attachment: enabled',
        'Running: yes'
      ].join('\n')
    );
  });

  it('formats cached and downloaded sources', () => {
    expect(formatServerInfo(details({ source: 'cache' }))).toContain('Source: cached download');
    expect(formatServerInfo(details({ source: 'download' }))).toContain(
      'Source: downloaded this session'
    );
    expect(formatServerInfo(details({ source: 'unknown' }))).toContain('Source: unknown');
  });

  it('shows not running, unknown status, and the last error', () => {
    const failed = details({
      running: false,
      serverVersion: undefined,
      versionStatus: 'unknown',
      source: 'download',
      serverPath: undefined,
      lastError: 'unable to download pico-lsp: HTTP 404'
    });
    const text = formatServerInfo(failed);
    expect(text).toContain('Server: not running');
    expect(text).toContain('Version status: unknown');
    expect(text).toContain('Server path: not resolved');
    expect(text).toContain('Running: no');
    expect(text).toContain('Last error: unable to download pico-lsp: HTTP 404');
    expect(formatServerDiagnostics(failed)).toContain(
      '- Last error: `unable to download pico-lsp: HTTP 404`'
    );
  });

  it('shows a missing version without failing startup', () => {
    const text = formatServerInfo(details({ serverVersion: undefined, versionStatus: 'unknown' }));
    expect(text).toContain('Server: version not reported');
    expect(text).toContain('Running: yes');
  });

  it('labels development builds without treating them as a mismatch', () => {
    const development = details({ serverVersion: 'dev', versionStatus: 'development' });
    expect(formatServerInfo(development)).toContain('Server: dev');
    expect(formatServerInfo(development)).toContain('Version status: development build');
    expect(formatServerDiagnostics(development)).toContain(
      '- Version status: `development build`'
    );
  });

  it('formats copyable Markdown diagnostics', () => {
    const text = formatServerDiagnostics(details());
    for (const expected of [
      '### PICO-8 language server',
      '- Extension version: `1.7.0`',
      '- Server: `v1.7.0`',
      '- Version status: `match`',
      '- Source: `custom path (pico8.serverPath)`',
      '- Server path: `/usr/local/bin/pico-lsp`',
      '- Platform: `linux amd64`',
      '- Cache root: `/home/user/.config/Code/User/globalStorage/asimmons91.pico-lsp`',
      '- Workspace folders: `2`',
      '- Lua attachment: `enabled`',
      '- Running: `yes`'
    ]) {
      expect(text).toContain(expected);
    }
  });

  it('never copies source text or cart contents', () => {
    const text = formatServerDiagnostics({
      ...details(),
      sourceText: 'local secret = "cart contents"',
      cartContents: 'function _draw() cls() end'
    } as ServerDetails);
    expect(text).not.toContain('local secret');
    expect(text).not.toContain('cart contents');
    expect(text).not.toContain('_draw');
  });
});

describe('server version warnings', () => {
  it('stays silent for matches, development builds, and stopped servers', () => {
    expect(serverVersionWarning(details())).toBeUndefined();
    expect(serverVersionWarning(details({ versionStatus: 'development' }))).toBeUndefined();
    expect(
      serverVersionWarning(details({ running: false, versionStatus: 'mismatch' }))
    ).toBeUndefined();
  });

  it('warns once about a server with no version', () => {
    const warning = serverVersionWarning(
      details({ serverVersion: undefined, versionStatus: 'unknown' })
    );
    expect(warning?.message).toContain('did not report its version');
    expect(warning?.actions).toEqual(['Show Server Info', 'Show Log']);
  });

  it('offers settings for a custom-path mismatch', () => {
    const warning = serverVersionWarning(
      details({ serverVersion: 'v1.6.0', versionStatus: 'mismatch', source: 'serverPath' })
    );
    expect(warning?.message).toBe(
      'pico-lsp v1.6.0 does not match the PICO-8 extension 1.7.0.'
    );
    expect(warning?.actions).toEqual(['Show Server Info', 'Open Settings', 'Show Log']);
  });

  it('offers cache clearing for cached and downloaded mismatches', () => {
    for (const source of ['cache', 'download'] as const) {
      const warning = serverVersionWarning(
        details({ serverVersion: 'v1.6.0', versionStatus: 'mismatch', source })
      );
      expect(warning?.message).toContain(
        'pico-lsp v1.6.0 does not match the PICO-8 extension 1.7.0.'
      );
      expect(warning?.message).toContain('cached server may be stale');
      expect(warning?.actions).toEqual([
        'Clear Server Cache',
        'Show Server Info',
        'Show Log'
      ]);
    }
  });
});
