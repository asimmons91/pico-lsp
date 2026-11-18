import path from 'node:path';
import * as vscode from 'vscode';
import type { DocumentSelector } from 'vscode-languageclient/node';

const PICO8_FIND_GLOB = '**/*.p8';
const PICO8_EXCLUDE_GLOB = '**/{.git,node_modules,dist,out}/**';

export async function findPico8WorkspaceFolders(): Promise<vscode.WorkspaceFolder[]> {
  const folders = vscode.workspace.workspaceFolders ?? [];
  const result: vscode.WorkspaceFolder[] = [];
  for (const folder of folders) {
    const matches = await vscode.workspace.findFiles(
      new vscode.RelativePattern(folder, PICO8_FIND_GLOB),
      PICO8_EXCLUDE_GLOB,
      1
    );
    if (matches.length > 0) {
      result.push(folder);
    }
  }
  return result;
}

export function buildDocumentSelector(
  pico8Folders: vscode.WorkspaceFolder[],
  attachLua: boolean
): DocumentSelector {
  const selector: DocumentSelector = [
    { scheme: 'file', language: 'pico8', pattern: '**/*.p8' }
  ];
  if (attachLua) {
    for (const folder of pico8Folders) {
      selector.push({
        scheme: 'file',
        language: 'lua',
        pattern: {
          baseUri: { uri: folder.uri.toString(), name: folder.name },
          pattern: '**/*.lua'
        }
      });
    }
  }
  return selector;
}

export function isPico8Document(
  document: vscode.TextDocument,
  pico8Folders: vscode.WorkspaceFolder[],
  attachLua: boolean
): boolean {
  if (document.uri.scheme !== 'file') {
    return false;
  }
  if (document.languageId === 'pico8' || document.uri.path.toLowerCase().endsWith('.p8')) {
    return true;
  }
  if (!attachLua || document.languageId !== 'lua') {
    return false;
  }
  return pico8Folders.some((folder) => isWithin(folder.uri.fsPath, document.uri.fsPath));
}

function isWithin(folder: string, file: string): boolean {
  const relative = path.relative(folder, file);
  return relative !== '' && !relative.startsWith('..') && !path.isAbsolute(relative);
}

export class Pico8WorkspaceMonitor implements vscode.Disposable {
  private folders: vscode.WorkspaceFolder[] = [];
  private readonly changed = new vscode.EventEmitter<vscode.WorkspaceFolder[]>();
  private readonly watcher: vscode.FileSystemWatcher;
  private refreshTimer: NodeJS.Timeout | undefined;

  readonly onDidChange = this.changed.event;

  constructor() {
    this.watcher = vscode.workspace.createFileSystemWatcher('**/*.p8');
    this.watcher.onDidCreate(() => this.scheduleRefresh());
    this.watcher.onDidDelete(() => this.scheduleRefresh());
  }

  current(): vscode.WorkspaceFolder[] {
    return this.folders;
  }

  async refresh(): Promise<vscode.WorkspaceFolder[]> {
    const next = await findPico8WorkspaceFolders();
    const previous = new Set(this.folders.map((folder) => folder.uri.toString()));
    const current = new Set(next.map((folder) => folder.uri.toString()));
    if (previous.size !== current.size || [...previous].some((uri) => !current.has(uri))) {
      this.folders = next;
      this.changed.fire(next);
    }
    return next;
  }

  dispose(): void {
    if (this.refreshTimer) {
      clearTimeout(this.refreshTimer);
    }
    this.watcher.dispose();
    this.changed.dispose();
  }

  private scheduleRefresh(): void {
    if (this.refreshTimer) {
      clearTimeout(this.refreshTimer);
    }
    this.refreshTimer = setTimeout(() => {
      this.refreshTimer = undefined;
      void this.refresh();
    }, 250);
  }
}
