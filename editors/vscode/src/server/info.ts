import type { ServerVersionStatus } from './version';

export type ServerSource = 'serverPath' | 'cache' | 'download' | 'unknown';

export interface ServerDetails {
  extensionVersion: string;
  serverVersion?: string;
  versionStatus: ServerVersionStatus;
  source: ServerSource;
  serverPath?: string;
  platform: string;
  arch: string;
  cacheRoot: string;
  workspaceFolders: number;
  attachLua: boolean;
  running: boolean;
  lastError?: string;
}

export type ServerWarningAction =
  | 'Show Server Info'
  | 'Open Settings'
  | 'Show Log'
  | 'Clear Server Cache';

export interface ServerWarning {
  message: string;
  actions: ServerWarningAction[];
}

function serverLabel(details: ServerDetails): string {
  if (!details.running) {
    return 'not running';
  }
  return details.serverVersion ?? 'version not reported';
}

function sourceLabel(details: ServerDetails): string {
  switch (details.source) {
    case 'serverPath':
      return 'custom path (pico8.serverPath)';
    case 'cache':
      return 'cached download';
    case 'download':
      return 'downloaded this session';
    default:
      return 'unknown';
  }
}

function luaAttachLabel(details: ServerDetails): string {
  return details.attachLua ? 'enabled' : 'disabled';
}

function versionStatusLabel(status: ServerVersionStatus): string {
  return status === 'development' ? 'development build' : status;
}

// formatServerInfo renders the output-channel block shown by
// pico8.client.showServerInfo. It is metadata only: no document text, cart
// contents, or environment variables.
export function formatServerInfo(details: ServerDetails): string {
  const lines = [
    'PICO-8 language server',
    '',
    `Extension version: ${details.extensionVersion}`,
    `Server: ${serverLabel(details)}`,
    `Version status: ${versionStatusLabel(details.versionStatus)}`,
    `Source: ${sourceLabel(details)}`,
    `Server path: ${details.serverPath ?? 'not resolved'}`,
    `Platform: ${details.platform} ${details.arch}`,
    `Cache root: ${details.cacheRoot}`,
    `Workspace folders: ${details.workspaceFolders}`,
    `Lua attachment: ${luaAttachLabel(details)}`,
    `Running: ${details.running ? 'yes' : 'no'}`
  ];
  if (!details.running && details.serverVersion) {
    lines.push(`Last server version: ${details.serverVersion}`);
  }
  if (details.lastError) {
    lines.push(`Last error: ${details.lastError}`);
  }
  return lines.join('\n');
}

// formatServerDiagnostics renders the same metadata as Markdown for
// pico8.client.copyServerDiagnostics.
export function formatServerDiagnostics(details: ServerDetails): string {
  const lines = [
    '### PICO-8 language server',
    '',
    `- Extension version: \`${details.extensionVersion}\``,
    `- Server: \`${serverLabel(details)}\``,
    `- Version status: \`${versionStatusLabel(details.versionStatus)}\``,
    `- Source: \`${sourceLabel(details)}\``,
    `- Server path: \`${details.serverPath ?? 'not resolved'}\``,
    `- Platform: \`${details.platform} ${details.arch}\``,
    `- Cache root: \`${details.cacheRoot}\``,
    `- Workspace folders: \`${details.workspaceFolders}\``,
    `- Lua attachment: \`${luaAttachLabel(details)}\``,
    `- Running: \`${details.running ? 'yes' : 'no'}\``
  ];
  if (!details.running && details.serverVersion) {
    lines.push(`- Last server version: \`${details.serverVersion}\``);
  }
  if (details.lastError) {
    lines.push(`- Last error: \`${details.lastError}\``);
  }
  return lines.join('\n');
}

// serverVersionWarning returns the one warning to show after a successful
// start, or undefined when the pair matches or is a development build.
export function serverVersionWarning(details: ServerDetails): ServerWarning | undefined {
  if (!details.running) {
    return undefined;
  }
  switch (details.versionStatus) {
    case 'match':
    case 'development':
      return undefined;
    case 'unknown':
      return {
        message: 'pico-lsp did not report its version.',
        actions: ['Show Server Info', 'Show Log']
      };
    case 'mismatch': {
      const message = `pico-lsp ${details.serverVersion} does not match the PICO-8 extension ${details.extensionVersion}.`;
      if (details.source === 'serverPath') {
        return {
          message,
          actions: ['Show Server Info', 'Open Settings', 'Show Log']
        };
      }
      return {
        message: `${message} The cached server may be stale; clear the cache and restart to download a matching server.`,
        actions: ['Clear Server Cache', 'Show Server Info', 'Show Log']
      };
    }
  }
}
