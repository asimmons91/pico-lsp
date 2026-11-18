import { cp, mkdtemp, writeFile } from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { runTests } from '@vscode/test-electron';

async function main(): Promise<void> {
  const extensionRoot = path.resolve(__dirname, '..', '..');
  const server = process.env.PICO_LSP_TEST_SERVER;

  if (!server) {
    console.error('PICO_LSP_TEST_SERVER must point at a built pico-lsp binary');
    process.exitCode = 1;
    return;
  }

  const workspaceRoot = await mkdtemp(path.join(os.tmpdir(), 'pico-vscode-test-'));
  const pico8Workspace = path.join(workspaceRoot, 'pico8');
  const plainWorkspace = path.join(workspaceRoot, 'plain');
  await cp(path.join(extensionRoot, 'test', 'fixtures', 'pico8'), pico8Workspace, {
    recursive: true
  });
  await cp(path.join(extensionRoot, 'test', 'fixtures', 'plain'), plainWorkspace, {
    recursive: true
  });
  const workspaceFile = path.join(workspaceRoot, 'pico.code-workspace');
  await writeFile(
    workspaceFile,
    JSON.stringify(
      {
        folders: [{ path: 'pico8' }, { path: 'plain' }],
        settings: {
          pico8: {
            serverPath: server,
            autoDownload: false,
            attachLua: true
          }
        }
      },
      null,
      2
    )
  );

  try {
    await runTests({
      version: '1.95.0',
      extensionDevelopmentPath: extensionRoot,
      extensionTestsPath: path.join(extensionRoot, 'out', 'integration', 'index.js'),
      launchArgs: ['--disable-extensions', '--no-sandbox', workspaceFile],
      extensionTestsEnv: {
        PICO_LSP_TEST_WORKSPACE: pico8Workspace,
        PICO_LSP_TEST_PLAIN_WORKSPACE: plainWorkspace
      }
    });
  } finally {
    console.log(`Integration workspace: ${workspaceRoot}`);
  }
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
