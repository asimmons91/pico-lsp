import { readFileSync } from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';

interface GlyphItemSchema {
  required?: string[];
  oneOf?: Array<{ required?: string[] }>;
  properties?: Record<string, unknown>;
}

interface ConfigurationProperty {
  type?: string;
  enum?: unknown[];
  default?: unknown;
  minimum?: number;
  scope?: string;
  description?: string;
  markdownDescription?: string;
  properties?: Record<string, ConfigurationProperty>;
  additionalProperties?: boolean;
  items?: GlyphItemSchema;
}

interface ConfigurationBlock {
  title?: string;
  properties?: Record<string, ConfigurationProperty>;
}

interface Manifest {
  contributes?: {
    configuration?: ConfigurationBlock | ConfigurationBlock[];
  };
}

const extensionRoot = path.resolve(__dirname, '../..');
const manifest = JSON.parse(readFileSync(path.join(extensionRoot, 'package.json'), 'utf8')) as Manifest;

const blocks = Array.isArray(manifest.contributes?.configuration)
  ? manifest.contributes.configuration
  : manifest.contributes?.configuration
    ? [manifest.contributes.configuration]
    : [];
const properties: Record<string, ConfigurationProperty> = Object.assign(
  {},
  ...blocks.map((block) => block.properties ?? {})
);

function flatten(
  prefix: string,
  property: ConfigurationProperty,
  out: Map<string, ConfigurationProperty>
): void {
  if (property.properties && Object.keys(property.properties).length > 0) {
    for (const [name, child] of Object.entries(property.properties)) {
      flatten(`${prefix}.${name}`, child, out);
    }
    return;
  }
  out.set(prefix, property);
}

function leafProperties(): Map<string, ConfigurationProperty> {
  const leaves = new Map<string, ConfigurationProperty>();
  for (const [name, property] of Object.entries(properties)) {
    flatten(name, property, leaves);
  }
  return leaves;
}

describe('PICO-8 settings schema', () => {
  it('contributes every server setting with a description', () => {
    for (const [name, property] of Object.entries(properties)) {
      expect(property.description ?? property.markdownDescription, name).toBeTruthy();
    }
    for (const [name, property] of leafProperties()) {
      expect(property.description ?? property.markdownDescription, name).toBeTruthy();
    }
  });

  it('declares the nested server settings', () => {
    const expected: Record<string, string[]> = {
      'pico8.limits': ['codeLens', 'diagnostics'],
      'pico8.format': ['maxBlankLines'],
      'pico8.diagnostics': [
        'mode',
        'undefinedGlobal',
        'unusedLocal',
        'unreachableCode',
        'duplicateTableField',
        'callbackPrecedence',
        'callbackParams',
        'noStdlib',
        'shadowedLocal',
        'emptyBlock',
        'globalInFunction',
        'includeOutsideWorkspace',
        'includeUnusualType',
        'includeDuplicate'
      ],
      'pico8.lint': ['apiRanges'],
      'pico8.refactors': ['enabled']
    };

    for (const [name, children] of Object.entries(expected)) {
      const property = properties[name];
      expect(property, name).toBeDefined();
      expect(property?.type, name).toBe('object');
      expect(property?.additionalProperties, name).toBe(false);
      for (const child of children) {
        expect(property?.properties?.[child], `${name}.${child}`).toBeDefined();
      }
    }
  });

  it('keeps the glyph item schema', () => {
    const glyphs = properties['pico8.glyphs'];
    expect(glyphs?.type).toBe('array');
    expect(glyphs?.items?.required).toEqual(['name']);

    const oneOf = glyphs?.items?.oneOf ?? [];
    expect(oneOf).toHaveLength(3);
    expect(new Set(oneOf.map((entry) => entry.required?.[0]))).toEqual(
      new Set(['byte', 'glyph', 'text'])
    );
    expect(glyphs?.items?.properties).toHaveProperty('context');
    expect(glyphs?.items?.properties).toHaveProperty('aliases');
    expect(glyphs?.items?.properties).toHaveProperty('description');
  });

  it('declares scopes for every setting', () => {
    for (const [name, property] of Object.entries(properties)) {
      expect(property.scope, name).toBeTruthy();
    }
    expect(properties['pico8.serverPath']?.scope).toBe('machine');
    for (const name of [
      'pico8.autoDownload',
      'pico8.attachLua',
      'pico8.trace.server',
      'pico8.glyphs'
    ]) {
      expect(properties[name]?.scope, name).toBe('window');
    }
  });
});
