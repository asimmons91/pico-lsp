import { createHash } from 'node:crypto';
import { createReadStream } from 'node:fs';

export function parseChecksums(text: string): Map<string, string> {
  const checksums = new Map<string, string>();
  for (const rawLine of text.split(/\r?\n/)) {
    const line = rawLine.trim();
    if (!line) {
      continue;
    }
    const match = /^([a-fA-F0-9]{64})\s+\*?(.+)$/.exec(line);
    if (!match) {
      continue;
    }
    checksums.set(match[2].trim(), match[1].toLowerCase());
  }
  return checksums;
}

export function requireChecksum(text: string, asset: string): string {
  let checksum: string | undefined;
  let matches = 0;
  for (const rawLine of text.split(/\r?\n/)) {
    const match = /^([a-fA-F0-9]{64})\s+\*?(.+)$/.exec(rawLine.trim());
    if (!match || match[2].trim() !== asset) {
      continue;
    }
    matches++;
    checksum = match[1].toLowerCase();
  }
  if (matches === 0 || !checksum) {
    throw new Error(`checksums.txt does not contain ${asset}`);
  }
  if (matches > 1) {
    throw new Error(`checksums.txt contains multiple entries for ${asset}`);
  }
  return checksum;
}

export async function sha256File(file: string): Promise<string> {
  const hash = createHash('sha256');
  await new Promise<void>((resolve, reject) => {
    const stream = createReadStream(file);
    stream.on('data', (chunk) => hash.update(chunk));
    stream.on('error', reject);
    stream.on('end', resolve);
  });
  return hash.digest('hex');
}

export async function verifyFileChecksum(file: string, expected: string): Promise<void> {
  const actual = await sha256File(file);
  if (actual.toLowerCase() !== expected.toLowerCase()) {
    throw new Error(`checksum mismatch for ${file}: expected ${expected}, got ${actual}`);
  }
}
