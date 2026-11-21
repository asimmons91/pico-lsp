import * as vscode from 'vscode';
import { PicoClient } from './client';
import { extensionSettings } from './config';
import { CartStatusBar } from './statusBar';
import { Pico8WorkspaceMonitor } from './workspace';

let extensionClient: PicoClient | undefined;

export async function activate(context: vscode.ExtensionContext): Promise<unknown> {
  if (vscode.env.remoteName) {
    void vscode.window.showErrorMessage(
      `PICO-8 language support does not support remote extension hosts (${vscode.env.remoteName}). Open the folder on the local machine.`
    );
    return undefined;
  }

  const output = vscode.window.createOutputChannel('PICO-8 Language Server', { log: true });
  const trace = vscode.window.createOutputChannel('PICO-8 Language Server Trace', { log: true });
  const monitor = new Pico8WorkspaceMonitor();
  await monitor.refresh();

  const holder: { client: PicoClient | undefined } = { client: undefined };
  const statusBar = new CartStatusBar({
    client: () => holder.client?.current(),
    pico8Folders: () => monitor.current(),
    attachLua: () => extensionSettings().attachLua,
    log: (message, error) => {
      output.warn(`${message}: ${error instanceof Error ? error.message : String(error ?? '')}`);
    }
  });
  const client = new PicoClient(context, output, trace, monitor, statusBar);
  holder.client = client;
  extensionClient = client;

  context.subscriptions.push(
    output,
    trace,
    monitor,
    statusBar,
    client,
    vscode.commands.registerCommand('pico8.client.showCartInfo', () => client.showCartInfo()),
    vscode.commands.registerCommand(
      'pico8.client.insertGlyph',
      (selection?: string | { name?: string; byte?: number }) => client.insertGlyph(selection)
    ),
    vscode.commands.registerCommand('pico8.client.restartServer', () => client.restart()),
    vscode.commands.registerCommand('pico8.client.showOutput', () => client.showOutput()),
    vscode.commands.registerCommand('pico8.client.showServerInfo', () => client.showServerInfo()),
    vscode.commands.registerCommand('pico8.client.copyServerDiagnostics', () =>
      client.copyServerDiagnostics()
    ),
    vscode.commands.registerCommand('pico8.client.clearServerCache', () => client.clearCache()),
    monitor.onDidChange(() => {
      void client.restart();
    }),
    vscode.workspace.onDidChangeWorkspaceFolders(() => {
      void monitor.refresh();
    }),
    vscode.workspace.onDidChangeConfiguration((event) => {
      if (
        event.affectsConfiguration('pico8.serverPath') ||
        event.affectsConfiguration('pico8.autoDownload') ||
        event.affectsConfiguration('pico8.attachLua')
      ) {
        void client.restart();
      } else if (event.affectsConfiguration('pico8.trace.server')) {
        client.refreshTrace(extensionSettings().traceServer);
      }
    })
  );

  await client.start();
  return {
    getClient: () => client.current(),
    getServerInfo: () => client.getServerInfo(),
    getStatusBar: () => ({ text: statusBar.text(), visible: statusBar.visible() })
  };
}

export async function deactivate(): Promise<void> {
  await extensionClient?.shutdown();
  extensionClient = undefined;
}
