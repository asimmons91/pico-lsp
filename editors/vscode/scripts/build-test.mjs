import { build } from 'esbuild';

await build({
  entryPoints: ['test/integration/index.ts'],
  bundle: true,
  platform: 'node',
  target: 'node20',
  format: 'cjs',
  outfile: 'out/integration/index.js',
  external: ['vscode'],
  sourcemap: false,
  minify: false,
  logLevel: 'info'
});
