import * as vscode from 'vscode';

export interface ExtensionSettings {
  serverPath: string;
  autoDownload: boolean;
  attachLua: boolean;
  traceServer: 'off' | 'messages' | 'verbose';
}

export interface HttpSettings {
  proxy: string;
  strictSSL: boolean;
}

export function extensionSettings(): ExtensionSettings {
  const config = vscode.workspace.getConfiguration('pico8');
  return {
    serverPath: config.get<string>('serverPath', ''),
    autoDownload: config.get<boolean>('autoDownload', true),
    attachLua: config.get<boolean>('attachLua', true),
    traceServer: config.get<'off' | 'messages' | 'verbose'>('trace.server', 'off')
  };
}

export function httpSettings(): HttpSettings {
  const config = vscode.workspace.getConfiguration('http');
  return {
    proxy: config.get<string | null>('proxy', null) ?? '',
    strictSSL: config.get<boolean>('proxyStrictSSL', true)
  };
}

export function pico8InitializationOptions(): Record<string, unknown> {
  const config = vscode.workspace.getConfiguration('pico8');
  return {
    pico8: {
      apiVersion: config.get('apiVersion'),
      typeChecking: config.get('typeChecking'),
      inlayHints: config.get('inlayHints'),
      maxProblems: config.get('maxProblems'),
      limits: config.get('limits'),
      format: config.get('format'),
      diagnostics: config.get('diagnostics'),
      lint: config.get('lint'),
      refactors: config.get('refactors'),
      glyphs: config.get('glyphs')
    }
  };
}
