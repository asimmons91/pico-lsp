import * as vscode from 'vscode';
import {
  LanguageClient,
  State,
  type LanguageClientOptions,
  type ServerOptions
} from 'vscode-languageclient/node';
import { Trace } from 'vscode-jsonrpc';
import { extensionSettings, httpSettings, pico8InitializationOptions } from './config';
import { targetFor } from './server/platforms';
import { clearServerCache, isUsableServerBinary } from './server/download';
import { resolveServer } from './server/resolve';
import { serverBinaryPath } from './server/paths';
import type { CartStatusBar } from './statusBar';
import { buildDocumentSelector, isPico8Document, type Pico8WorkspaceMonitor } from './workspace';

const CLIENT_ID = 'pico-lsp';
const CLIENT_NAME = 'PICO-8 Language Server';

export class PicoClient implements vscode.Disposable {
  private client: LanguageClient | undefined;
  private running: LanguageClient | undefined;
  private operation = Promise.resolve();
  private disposed = false;

  constructor(
    private readonly context: vscode.ExtensionContext,
    private readonly output: vscode.LogOutputChannel,
    private readonly trace: vscode.LogOutputChannel,
    private readonly monitor: Pico8WorkspaceMonitor,
    private readonly statusBar: CartStatusBar
  ) {}

  current(): LanguageClient | undefined {
    return this.client;
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

    let serverPath: string;
    try {
      serverPath = await this.resolveWithProgress(settings, target, cacheRoot, version);
    } catch (error) {
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

    const folders = this.monitor.current();
    const documentSelector = buildDocumentSelector(folders, settings.attachLua);
    const serverOptions: ServerOptions = {
      command: serverPath,
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
      this.output.info(`pico-lsp ${version} started from ${serverPath}`);
      this.statusBar.updateNow();
    } catch (error) {
      this.client = undefined;
      this.running = undefined;
      this.output.error(`unable to start pico-lsp: ${errorMessage(error)}`);
      void vscode.window.showErrorMessage(`Unable to start pico-lsp: ${errorMessage(error)}`);
    }
  }

  private async resolveWithProgress(
    settings: ReturnType<typeof extensionSettings>,
    target: ReturnType<typeof targetFor>,
    cacheRoot: string,
    version: string
  ): Promise<string> {
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

  private async stopNow(): Promise<void> {
    const client = this.client;
    this.client = undefined;
    this.running = undefined;
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
