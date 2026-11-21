import assert from 'node:assert/strict';
import path from 'node:path';
import * as vscode from 'vscode';
import type { ServerDetails } from '../../src/server/info';
import { isReleaseVersion } from '../../src/server/version';

const SOURCE = 'pico-lsp';

export async function run(): Promise<void> {
  const pico8Workspace = requiredEnvironment('PICO_LSP_TEST_WORKSPACE');
  const plainWorkspace = requiredEnvironment('PICO_LSP_TEST_PLAIN_WORKSPACE');
  const extension = vscode.extensions.getExtension('asimmons91.pico-lsp');
  assert.ok(extension, 'PICO-8 extension was not installed in the test host');
  await extension.activate();

  const configuration = vscode.workspace.getConfiguration('pico8');
  assert.equal(configuration.inspect('typeChecking')?.defaultValue, 'basic');
  assert.equal(configuration.inspect('diagnostics.undefinedGlobal')?.defaultValue, 'warning');
  assert.equal(configuration.inspect('limits.codeLens')?.defaultValue, true);
  assert.equal(configuration.inspect('format.maxBlankLines')?.defaultValue, 2);
  assert.equal(configuration.inspect('autoDownload')?.defaultValue, true);
  assert.equal(configuration.inspect('serverPath')?.defaultValue, '');

  const cartUri = vscode.Uri.file(path.join(pico8Workspace, 'cart.p8'));
  const cart = await vscode.workspace.openTextDocument(cartUri);
  assert.equal(cart.languageId, 'pico8');
  await vscode.window.showTextDocument(cart, { preview: false });

  const cartDiagnostics = await waitForDiagnostics(cartUri, (diagnostics) =>
    diagnostics.some((diagnostic) => diagnostic.source === SOURCE)
  );
  const api = extension.exports as {
    getServerInfo?: () => ServerDetails;
    getStatusBar?: () => { text: string; visible: boolean };
  };
  await waitFor(
    () => {
      const status = api.getStatusBar?.();
      return status?.visible === true && status.text.includes('PICO-8');
    },
    'cart status bar'
  );

  const serverInfo = await waitForServerInfo(api);
  assert.equal(serverInfo.extensionVersion, String(extension.packageJSON.version));
  assert.equal(serverInfo.source, 'serverPath');
  assert.equal(serverInfo.running, true);
  const serverVersion = serverInfo.serverVersion;
  const serverPath = serverInfo.serverPath;
  assert.ok(serverPath, 'server path was not recorded');
  assert.ok(serverVersion, 'server version was not read from initializeResult');
  if (isReleaseVersion(serverVersion)) {
    assert.equal(serverInfo.versionStatus, 'match');
  } else {
    assert.equal(serverInfo.versionStatus, 'development');
  }
  assert.equal(serverInfo.platform, process.platform);
  assert.equal(serverInfo.arch, process.arch);
  assert.equal(serverInfo.workspaceFolders, 2);
  assert.equal(serverInfo.attachLua, true);

  const codeLenses = await vscode.commands.executeCommand<vscode.CodeLens[]>(
    'vscode.executeCodeLensProvider',
    cartUri
  );
  assert.ok(codeLenses.length > 0, 'cart budget code lens was not provided');
  assert.ok(
    cartDiagnostics.some(
      (diagnostic) => diagnostic.source === SOURCE && diagnostic.code === 'undefined-global'
    ),
    `missing undefined-global diagnostic: ${JSON.stringify(cartDiagnostics)}`
  );

  const completionOffset = cart.getText().indexOf('circfill');
  assert.ok(completionOffset >= 0, 'fixture has no circfill');
  const completions = await vscode.commands.executeCommand<
    vscode.CompletionList | vscode.CompletionItem[]
  >(
    'vscode.executeCompletionItemProvider',
    cartUri,
    cart.positionAt(completionOffset + 'circ'.length)
  );
  const labels = (Array.isArray(completions) ? completions : completions.items).map((item) =>
    typeof item.label === 'string' ? item.label : item.label.label
  );
  assert.ok(labels.includes('circfill'), `circfill missing from completions: ${labels.join(', ')}`);

  const hover = await vscode.commands.executeCommand<vscode.Hover[]>(
    'vscode.executeHoverProvider',
    cartUri,
    cart.positionAt(completionOffset + 1)
  );
  assert.ok(hover.length > 0, 'circfill hover was empty');

  const stringOffset = cart.getText().indexOf('local bad = "x"');
  assert.ok(stringOffset >= 0, 'fixture has no local bad string');
  const glyphPosition = cart.positionAt(stringOffset + 'local bad = "x'.length);
  const editor = await vscode.window.showTextDocument(cart, { preview: false });
  editor.selection = new vscode.Selection(glyphPosition, glyphPosition);
  await vscode.commands.executeCommand('pico8.client.insertGlyph', 'left');
  await waitFor(() => cart.getText().includes('"x⬅️"'), 'glyph insertion');

  const badOffset = cart.getText().indexOf('local bad');
  const badLineEnd = cart.getText().indexOf('\n', badOffset);
  const actions = await vscode.commands.executeCommand<vscode.CodeAction[]>(
    'vscode.executeCodeActionProvider',
    cartUri,
    new vscode.Range(cart.positionAt(badOffset), cart.positionAt(badLineEnd))
  );
  assert.ok(
    actions.some((action) => action.title.includes('Suppress')),
    `missing diagnostic suppression action: ${actions.map((action) => action.title).join(', ')}`
  );

  await vscode.commands.executeCommand('pico8.client.showCartInfo');
  await vscode.commands.executeCommand('pico8.client.showServerInfo');
  await vscode.commands.executeCommand('pico8.client.copyServerDiagnostics');
  const copiedDiagnostics = await vscode.env.clipboard.readText();
  for (const expected of [
    '### PICO-8 language server',
    serverInfo.extensionVersion,
    serverVersion,
    'custom path (pico8.serverPath)',
    serverPath,
    process.platform
  ]) {
    assert.ok(
      copiedDiagnostics.includes(expected),
      `copied diagnostics are missing ${expected}: ${copiedDiagnostics}`
    );
  }
  assert.ok(
    !copiedDiagnostics.includes('missing_global()'),
    'copied diagnostics contain cart source text'
  );

  const libUri = vscode.Uri.file(path.join(pico8Workspace, 'lib.lua'));
  const lib = await vscode.workspace.openTextDocument(libUri);
  await vscode.window.showTextDocument(lib, { preview: false });
  await waitForDiagnostics(libUri, (diagnostics) =>
    diagnostics.some((diagnostic) => diagnostic.source === SOURCE)
  );

  const plainUri = vscode.Uri.file(path.join(plainWorkspace, 'other.lua'));
  const plain = await vscode.workspace.openTextDocument(plainUri);
  await vscode.window.showTextDocument(plain, { preview: false });
  await delay(1500);
  assert.equal(
    vscode.languages
      .getDiagnostics(plainUri)
      .filter((diagnostic) => diagnostic.source === SOURCE).length,
    0,
    'pico-lsp attached to a Lua file outside PICO-8 workspaces'
  );

  await vscode.commands.executeCommand('pico8.client.restartServer');
  await waitForDiagnostics(cartUri, (diagnostics) =>
    diagnostics.some((diagnostic) => diagnostic.source === SOURCE)
  );
  const restartedInfo = await waitForServerInfo(api);
  assert.equal(restartedInfo.running, true);
  assert.equal(restartedInfo.source, 'serverPath');
  assert.equal(restartedInfo.serverPath, serverPath);
  assert.equal(restartedInfo.extensionVersion, serverInfo.extensionVersion);
  assert.equal(restartedInfo.serverVersion, serverVersion);
}

function requiredEnvironment(name: string): string {
  const value = process.env[name];
  if (!value) {
    throw new Error(`${name} is not set`);
  }
  return value;
}

async function waitForDiagnostics(
  uri: vscode.Uri,
  predicate: (diagnostics: vscode.Diagnostic[]) => boolean,
  timeout = 15_000
): Promise<vscode.Diagnostic[]> {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    const diagnostics = vscode.languages.getDiagnostics(uri);
    if (predicate(diagnostics)) {
      return diagnostics;
    }
    await delay(100);
  }
  throw new Error(`timed out waiting for diagnostics on ${uri.toString()}`);
}

async function waitForServerInfo(
  api: { getServerInfo?: () => ServerDetails },
  timeout = 15_000
): Promise<ServerDetails> {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    const info = api.getServerInfo?.();
    if (info?.running) {
      return info;
    }
    await delay(100);
  }
  throw new Error('timed out waiting for server info');
}

async function waitFor(
  predicate: () => boolean,
  description: string,
  timeout = 15_000
): Promise<void> {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    if (predicate()) {
      return;
    }
    await delay(100);
  }
  throw new Error(`timed out waiting for ${description}`);
}

function delay(milliseconds: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, milliseconds));
}
