import * as vscode from 'vscode';
import {
  LanguageClient,
  State,
  type LanguageClientOptions,
  type ServerOptions
} from 'vscode-languageclient/node';
import { Trace } from 'vscode-jsonrpc';
import { extensionSettings, httpSettings, pico8InitializationOptions } from './config';
import {
  formatServerDiagnostics,
  formatServerInfo,
  serverVersionWarning,
  type ServerDetails,
  type ServerSource
} from './server/info';
import { targetFor } from './server/platforms';
import { clearServerCache, isUsableServerBinary } from './server/download';
import { resolveServer, type ResolvedServer } from './server/resolve';
import { serverBinaryPath } from './server/paths';
import { classifyServerVersion } from './server/version';
import type { CartStatusBar } from './statusBar';
import { buildDocumentSelector, isPico8Document, type Pico8WorkspaceMonitor } from './workspace';

const CLIENT_ID = 'pico-lsp';
const CLIENT_NAME = 'PICO-8 Language Server';

interface P8GlyphEntry {
  name: string;
  text: string;
  byte: number;
  aliases?: string[];
  category: string;
  description?: string;
  context: 'both' | 'string';
}

interface P8GlyphsResult {
  glyphs: P8GlyphEntry[];
}

interface LspTextEdit {
  range: {
    start: { line: number; character: number };
    end: { line: number; character: number };
  };
  newText: string;
}

interface LspWorkspaceEdit {
  changes?: Record<string, LspTextEdit[]>;
}

interface GlyphPickItem extends vscode.QuickPickItem {
  glyph: P8GlyphEntry;
}

async function applyLspWorkspaceEdit(edit: LspWorkspaceEdit): Promise<void> {
  const workspaceEdit = new vscode.WorkspaceEdit();
  for (const [uriText, edits] of Object.entries(edit.changes ?? {})) {
    const documentUri = vscode.Uri.parse(uriText);
    for (const item of edits) {
      workspaceEdit.replace(
        documentUri,
        new vscode.Range(
          item.range.start.line,
          item.range.start.character,
          item.range.end.line,
          item.range.end.character
        ),
        item.newText
      );
    }
  }
  if (!(await vscode.workspace.applyEdit(workspaceEdit))) {
    throw new Error('VS Code refused the glyph edit.');
  }
}

export class PicoClient implements vscode.Disposable {
  private client: LanguageClient | undefined;
  private running: LanguageClient | undefined;
  private operation = Promise.resolve();
  private disposed = false;
  private serverDetails: ServerDetails;

  constructor(
    private readonly context: vscode.ExtensionContext,
    private readonly output: vscode.LogOutputChannel,
    private readonly trace: vscode.LogOutputChannel,
    private readonly monitor: Pico8WorkspaceMonitor,
    private readonly statusBar: CartStatusBar
  ) {
    this.serverDetails = {
      extensionVersion: String(context.extension.packageJSON.version),
      versionStatus: 'unknown',
      source: 'unknown',
      platform: process.platform,
      arch: process.arch,
      cacheRoot: context.globalStorageUri.fsPath,
      workspaceFolders: monitor.current().length,
      attachLua: extensionSettings().attachLua,
      running: false
    };
  }

  current(): LanguageClient | undefined {
    return this.client;
  }

  getServerInfo(): ServerDetails {
    return {
      ...this.serverDetails,
      extensionVersion: this.extensionVersion(),
      cacheRoot: this.context.globalStorageUri.fsPath,
      workspaceFolders: this.monitor.current().length,
      attachLua: extensionSettings().attachLua
    };
  }

  start(): Promise<void> {
    return this.enqueue(() => this.startNow());
  }

  restart(): Promise<void> {
    return this.enqueue(async () => {
      await this.stopNow();
      await this.startNow();
    });
  }

  dispose(): void {
    void this.shutdown();
  }

  async shutdown(): Promise<void> {
    this.disposed = true;
    await this.stopNow();
  }

  async showCartInfo(): Promise<void> {
    const client = this.client;
    if (!client) {
      void vscode.window.showErrorMessage('PICO-8 language server is not running.');
      return;
    }
    const editor = vscode.window.activeTextEditor;
    if (!editor || !isPico8Document(editor.document, this.monitor.current(), extensionSettings().attachLua)) {
      void vscode.window.showInformationMessage('Open a PICO-8 cart or included Lua file first.');
      return;
    }
    try {
      await client.sendRequest('workspace/executeCommand', {
        command: 'pico8.showCartInfo',
        arguments: [editor.document.uri.toString()]
      });
    } catch (error) {
      void vscode.window.showErrorMessage(`Unable to show PICO-8 cart info: ${errorMessage(error)}`);
    }
  }

  async insertGlyph(selection?: string | { name?: string; byte?: number }): Promise<void> {
    const client = this.client;
    if (!client) {
      void vscode.window.showErrorMessage('PICO-8 language server is not running.');
      return;
    }
    const editor = vscode.window.activeTextEditor;
    if (!editor || !isPico8Document(editor.document, this.monitor.current(), extensionSettings().attachLua)) {
      void vscode.window.showInformationMessage('Open a PICO-8 cart or included Lua file first.');
      return;
    }

    let name = typeof selection === 'string' ? selection : selection?.name;
    const byte = typeof selection === 'object' && selection ? selection.byte : undefined;
    const position = editor.selection.active;
    const textDocument = { uri: editor.document.uri.toString() };

    try {
      if (!name && byte === undefined) {
        const result = await client.sendRequest<P8GlyphsResult>('pico8/glyphs', {
          textDocument,
          position
        });
        const items: GlyphPickItem[] = result.glyphs.map((glyph) => ({
          label: `${glyph.text}  ${glyph.name}`,
          description: `P8SCII 0x${glyph.byte.toString(16).padStart(2, '0')}`,
          detail: glyph.description,
          glyph
        }));
        const picked = await vscode.window.showQuickPick(items, {
          placeHolder: 'Select a P8SCII glyph',
          matchOnDescription: true,
          matchOnDetail: true
        });
        if (!picked) {
          return;
        }
        name = picked.glyph.name;
      }

      const argument: Record<string, unknown> = { textDocument, position };
      if (name) {
        argument.name = name;
      } else if (byte !== undefined) {
        argument.byte = byte;
      }
      const result = await client.sendRequest<{ applied?: boolean; edit?: LspWorkspaceEdit } | null>(
        'workspace/executeCommand',
        {
          command: 'pico8.insertGlyph',
          arguments: [argument]
        }
      );
      if (result && result.applied === false && result.edit) {
        await applyLspWorkspaceEdit(result.edit);
      }
    } catch (error) {
      void vscode.window.showErrorMessage(`Unable to insert P8SCII glyph: ${errorMessage(error)}`);
    }
  }

  async clearCache(): Promise<void> {
    const answer = await vscode.window.showWarningMessage(
      `Delete downloaded pico-lsp ${this.extensionVersion()} servers?`,
      { modal: true },
      'Delete'
    );
    if (answer !== 'Delete') {
      return;
    }
    try {
      await clearServerCache(this.context.globalStorageUri.fsPath, this.extensionVersion());
      void vscode.window.showInformationMessage(
        'Downloaded PICO-8 language servers removed. Restart the server to use pico8.serverPath or download a fresh copy when auto-download is enabled.'
      );
    } catch (error) {
      void vscode.window.showErrorMessage(`Unable to clear server cache: ${errorMessage(error)}`);
    }
  }

  showOutput(): void {
    this.output.show(true);
  }

  showServerInfo(): void {
    try {
      this.output.appendLine('');
      this.output.appendLine(formatServerInfo(this.getServerInfo()));
      this.output.show(true);
    } catch (error) {
      void this.showActionableError(
        `Unable to show PICO-8 server info: ${errorMessage(error)}`
      );
    }
  }

  async copyServerDiagnostics(): Promise<void> {
    try {
      await vscode.env.clipboard.writeText(formatServerDiagnostics(this.getServerInfo()));
      void vscode.window.showInformationMessage(
        'PICO-8 server diagnostics copied to the clipboard.'
      );
    } catch (error) {
      await this.showActionableError(
        `Unable to copy PICO-8 server diagnostics: ${errorMessage(error)}`
      );
    }
  }

  refreshTrace(setting: 'off' | 'messages' | 'verbose'): void {
    if (this.client) {
      void this.applyTrace(this.client, setting).catch((error) => {
        this.output.warn(`unable to set LSP trace level: ${errorMessage(error)}`);
      });
    }
  }

  private enqueue(operation: () => Promise<void>): Promise<void> {
    const next = this.operation.then(operation, operation);
    this.operation = next.catch(() => undefined);
    return next;
  }

  private async startNow(): Promise<void> {
    if (this.disposed) {
      return;
    }
    const settings = extensionSettings();
    const target = targetFor(process.platform, process.arch);
    const cacheRoot = this.context.globalStorageUri.fsPath;
    const version = this.extensionVersion();
    const folders = this.monitor.current();

    // Clear stale run details but keep the most recent error until a start
    // succeeds, so Server Info remains useful if this attempt fails early.
    this.serverDetails = {
      ...this.serverDetails,
      extensionVersion: version,
      serverVersion: undefined,
      versionStatus: 'unknown',
      source: 'unknown',
      serverPath: undefined,
      platform: process.platform,
      arch: process.arch,
      cacheRoot,
      workspaceFolders: folders.length,
      attachLua: settings.attachLua,
      running: false
    };

    let resolved: ResolvedServer;
    try {
      resolved = await this.resolveWithProgress(settings, target, cacheRoot, version);
    } catch (error) {
      const attempted = this.attemptedSource(settings, target, cacheRoot, version);
      this.serverDetails = {
        ...this.serverDetails,
        source: attempted.source,
        serverPath: attempted.serverPath,
        lastError: errorMessage(error)
      };
      this.output.error(`unable to start pico-lsp: ${errorMessage(error)}`);
      const choice = await vscode.window.showErrorMessage(
        `Unable to start pico-lsp: ${errorMessage(error)}`,
        'Retry',
        'Open Settings',
        'Show Log'
      );
      if (choice === 'Retry') {
        // The current operation must finish before the queued restart can run.
        void this.restart();
      } else if (choice === 'Open Settings') {
        await vscode.commands.executeCommand('workbench.action.openSettings', 'pico8.serverPath');
      } else if (choice === 'Show Log') {
        this.output.show(true);
      }
      return;
    }
    if (this.disposed) {
      return;
    }
    this.serverDetails = {
      ...this.serverDetails,
      source: resolved.source,
      serverPath: resolved.path
    };

    const documentSelector = buildDocumentSelector(folders, settings.attachLua);
    const serverOptions: ServerOptions = {
      command: resolved.path,
      args: [],
      options: {
        cwd: folders.length === 1 ? folders[0].uri.fsPath : undefined
      }
    };
    const clientOptions: LanguageClientOptions = {
      documentSelector,
      workspaceFolder: folders.length === 1 ? folders[0] : undefined,
      initializationOptions: pico8InitializationOptions(),
      outputChannel: this.output,
      traceOutputChannel: this.trace,
      synchronize: { configurationSection: 'pico8' }
    };
    const client = new LanguageClient(CLIENT_ID, CLIENT_NAME, serverOptions, clientOptions);
    this.client = client;
    client.onDidChangeState((event) => {
      if (event.newState === State.Stopped && this.running === client && !this.disposed) {
        this.output.warn('pico-lsp stopped unexpectedly.');
        this.running = undefined;
        this.serverDetails = { ...this.serverDetails, running: false };
        this.statusBar.updateNow();
        void vscode.window
          .showWarningMessage('pico-lsp stopped unexpectedly.', 'Show Log', 'Restart')
          .then((choice) => {
            if (choice === 'Show Log') {
              this.output.show(true);
            } else if (choice === 'Restart') {
              void this.restart();
            }
          });
      }
    });

    try {
      await client.start();
      if (this.disposed) {
        await this.stopNow();
        return;
      }
      this.running = client;
      try {
        await this.applyTrace(client, settings.traceServer);
      } catch (error) {
        this.output.warn(`unable to set LSP trace level: ${errorMessage(error)}`);
      }
      const reportedVersion: unknown = client.initializeResult?.serverInfo?.version;
      const serverVersion = typeof reportedVersion === 'string' ? reportedVersion : undefined;
      const versionStatus = classifyServerVersion(version, serverVersion);
      this.serverDetails = {
        ...this.serverDetails,
        serverVersion,
        versionStatus,
        running: true,
        lastError: undefined
      };
      this.output.info(
        `pico-lsp ${serverVersion ?? 'unknown version'} started from ${resolved.path} (${versionStatus})`
      );
      this.statusBar.updateNow();
      void this.warnAboutVersion();
    } catch (error) {
      this.client = undefined;
      this.running = undefined;
      this.serverDetails = {
        ...this.serverDetails,
        serverVersion: undefined,
        versionStatus: 'unknown',
        running: false,
        lastError: errorMessage(error)
      };
      this.output.error(`unable to start pico-lsp: ${errorMessage(error)}`);
      void vscode.window.showErrorMessage(`Unable to start pico-lsp: ${errorMessage(error)}`);
    }
  }

  private async resolveWithProgress(
    settings: ReturnType<typeof extensionSettings>,
    target: ReturnType<typeof targetFor>,
    cacheRoot: string,
    version: string
  ): Promise<ResolvedServer> {
    const cached = serverBinaryPath(cacheRoot, version, target);
    if (settings.serverPath || (await isUsableServerBinary(cached))) {
      return resolveServer({
        version,
        target,
        cacheRoot,
        serverPath: settings.serverPath,
        autoDownload: settings.autoDownload,
        development: this.context.extensionMode === vscode.ExtensionMode.Development
      });
    }

    const http = httpSettings();
    return vscode.window.withProgress(
      {
        location: vscode.ProgressLocation.Notification,
        title: `Downloading pico-lsp ${version}`,
        cancellable: true
      },
      async (_progress, token) =>
        resolveServer({
          version,
          target,
          cacheRoot,
          serverPath: settings.serverPath,
          autoDownload: settings.autoDownload,
          development: this.context.extensionMode === vscode.ExtensionMode.Development,
          proxy: http.proxy,
          strictSSL: http.strictSSL,
          token
        })
    );
  }

  private async applyTrace(
    client: LanguageClient,
    setting: 'off' | 'messages' | 'verbose'
  ): Promise<void> {
    const trace = setting === 'verbose' ? Trace.Verbose : setting === 'messages' ? Trace.Messages : Trace.Off;
    await client.setTrace(trace);
  }

  private attemptedSource(
    settings: ReturnType<typeof extensionSettings>,
    target: ReturnType<typeof targetFor>,
    cacheRoot: string,
    version: string
  ): { source: ServerSource; serverPath?: string } {
    if (settings.serverPath.trim()) {
      return { source: 'serverPath', serverPath: settings.serverPath.trim() };
    }
    if (settings.autoDownload) {
      return {
        source: 'download',
        serverPath: serverBinaryPath(cacheRoot, version, target)
      };
    }
    return { source: 'unknown' };
  }

  private async warnAboutVersion(): Promise<void> {
    const warning = serverVersionWarning(this.getServerInfo());
    if (!warning) {
      return;
    }
    const choice = await vscode.window.showWarningMessage(warning.message, ...warning.actions);
    if (choice === 'Show Server Info') {
      this.showServerInfo();
    } else if (choice === 'Open Settings') {
      await vscode.commands.executeCommand('workbench.action.openSettings', 'pico8.serverPath');
    } else if (choice === 'Show Log') {
      this.showOutput();
    } else if (choice === 'Clear Server Cache') {
      await this.clearCache();
    }
  }

  private async showActionableError(message: string): Promise<void> {
    const choice = await vscode.window.showErrorMessage(message, 'Show Log');
    if (choice === 'Show Log') {
      this.showOutput();
    }
  }

  private async stopNow(): Promise<void> {
    const client = this.client;
    this.client = undefined;
    this.running = undefined;
    this.serverDetails = { ...this.serverDetails, running: false };
    if (!client) {
      return;
    }
    try {
      await client.stop();
      client.dispose();
    } catch (error) {
      this.output.warn(`error stopping pico-lsp: ${errorMessage(error)}`);
    }
  }

  private extensionVersion(): string {
    return String(this.context.extension.packageJSON.version);
  }
}

export function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}
