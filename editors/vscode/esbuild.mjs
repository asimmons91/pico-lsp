import { context } from 'esbuild';

const watch = process.argv.includes('--watch');

const buildContext = await context({
  entryPoints: ['src/extension.ts'],
  bundle: true,
  platform: 'node',
  target: 'node20',
  format: 'cjs',
  outfile: 'dist/extension.js',
  external: ['vscode'],
  sourcemap: false,
  minify: false,
  logLevel: 'info'
});

if (watch) {
  await buildContext.watch();
  console.log('Watching for extension changes...');
} else {
  await buildContext.rebuild();
  await buildContext.dispose();
}
