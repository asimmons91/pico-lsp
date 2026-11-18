import { execFileSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import path from 'node:path';

const manifest = JSON.parse(readFileSync('package.json', 'utf8'));
const version = manifest.version;
const vsce = path.resolve('node_modules', '.bin', process.platform === 'win32' ? 'vsce.cmd' : 'vsce');
const output = path.resolve('..', '..', `pico-lsp-vscode-${version}.vsix`);

execFileSync(vsce, ['package', '--no-dependencies', '--out', output], {
  stdio: 'inherit',
  shell: process.platform === 'win32'
});
