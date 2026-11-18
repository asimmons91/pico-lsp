import * as vscode from 'vscode';
import type { LanguageClient } from 'vscode-languageclient/node';
import { cartInfoSeverity, formatCartInfo, type CartInfo } from './statusFormat';
import { isPico8Document } from './workspace';

interface StatusBarDependencies {
  client: () => LanguageClient | undefined;
  pico8Folders: () => vscode.WorkspaceFolder[];
  attachLua: () => boolean;
  log: (message: string, error?: unknown) => void;
}

export class CartStatusBar implements vscode.Disposable {
  private readonly item: vscode.StatusBarItem;
  private readonly disposables: vscode.Disposable[] = [];
  private timer: NodeJS.Timeout | undefined;
  private request: vscode.CancellationTokenSource | undefined;
  private isVisible = false;

  constructor(private readonly dependencies: StatusBarDependencies) {
    this.item = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Right, 100);
    this.item.name = 'PICO-8 Cart Budget';
    this.item.command = 'pico8.client.showCartInfo';
    this.disposables.push(
      this.item,
      vscode.window.onDidChangeActiveTextEditor(() => this.schedule(0)),
      vscode.workspace.onDidChangeTextDocument((event) => {
        if (event.document === vscode.window.activeTextEditor?.document) {
          this.schedule(350);
        }
      }),
      vscode.workspace.onDidSaveTextDocument(() => this.schedule(100)),
      vscode.workspace.onDidChangeConfiguration((event) => {
        if (event.affectsConfiguration('pico8.attachLua')) {
          this.schedule(0);
        }
      })
    );
    this.schedule(0);
  }

  updateNow(): void {
    this.schedule(0);
  }

  text(): string {
    return this.item.text;
  }

  visible(): boolean {
    return this.isVisible;
  }

  dispose(): void {
    if (this.timer) {
      clearTimeout(this.timer);
    }
    this.request?.cancel();
    this.request?.dispose();
    for (const disposable of this.disposables) {
      disposable.dispose();
    }
  }

  private schedule(delay: number): void {
    if (this.timer) {
      clearTimeout(this.timer);
    }
    this.timer = setTimeout(() => {
      this.timer = undefined;
      void this.refresh();
    }, delay);
  }

  private async refresh(): Promise<void> {
    const editor = vscode.window.activeTextEditor;
    if (
      !editor ||
      !isPico8Document(editor.document, this.dependencies.pico8Folders(), this.dependencies.attachLua())
    ) {
      this.hide();
      return;
    }
    const client = this.dependencies.client();
    if (!client) {
      this.hide();
      return;
    }

    this.request?.cancel();
    this.request?.dispose();
    this.request = new vscode.CancellationTokenSource();
    try {
      const info = await client.sendRequest<CartInfo>(
        'pico8/cartInfo',
        {
          textDocument: { uri: editor.document.uri.toString() }
        },
        this.request.token
      );
      if (editor !== vscode.window.activeTextEditor) {
        return;
      }
      this.show(info);
    } catch (error) {
      this.hide();
      if (!(error instanceof vscode.CancellationError)) {
        this.dependencies.log('unable to refresh cart info', error);
      }
    }
  }

  private show(info: CartInfo): void {
    this.item.text = formatCartInfo(info);
    this.item.tooltip = tooltip(info);
    const severity = cartInfoSeverity(info);
    this.item.backgroundColor =
      severity === 'error'
        ? new vscode.ThemeColor('statusBarItem.errorBackground')
        : severity === 'warning'
          ? new vscode.ThemeColor('statusBarItem.warningBackground')
          : undefined;
    this.item.show();
    this.isVisible = true;
  }

  private hide(): void {
    this.item.hide();
    this.item.backgroundColor = undefined;
    this.isVisible = false;
  }
}

function tooltip(info: CartInfo): vscode.MarkdownString {
  const markdown = new vscode.MarkdownString();
  markdown.appendMarkdown(`**${formatCartInfo(info)}**\n\n`);
  if (info.rootUri) {
    markdown.appendMarkdown(`Cart: \`${vscode.Uri.parse(info.rootUri).fsPath}\`\n\n`);
  }
  const files = info.files ?? [];
  if (files.length > 0) {
    markdown.appendMarkdown('| File | Tokens | Chars |\n|---|---:|---:|\n');
    for (const file of files) {
      markdown.appendMarkdown(
        `| \`${vscode.Uri.parse(file.uri).fsPath}\` | ${file.tokens} | ${file.chars} |\n`
      );
    }
  }
  return markdown;
}
