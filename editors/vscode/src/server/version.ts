export type ServerVersionStatus = 'match' | 'mismatch' | 'development' | 'unknown';

const releaseVersion = /^\d+\.\d+\.\d+$/;

// normalizeVersion trims surrounding whitespace and removes one leading v/V.
// The server reports v1.7.0 while the extension manifest uses 1.7.0, so both
// spellings must compare equal without adding semver range logic.
export function normalizeVersion(value: string): string {
  return value.trim().replace(/^[vV]/, '');
}

// isReleaseVersion accepts exactly X.Y.Z with non-negative decimal components.
// Pre-release suffixes, build metadata, dev, empty strings, and Go
// pseudo-versions are development versions.
export function isReleaseVersion(value: string): boolean {
  return releaseVersion.test(normalizeVersion(value));
}

export function classifyServerVersion(
  extensionVersion: string,
  serverVersion: string | undefined
): ServerVersionStatus {
  if (serverVersion === undefined) {
    return 'unknown';
  }
  const server = normalizeVersion(serverVersion);
  const extension = normalizeVersion(extensionVersion);
  // A pair is comparable only when both sides are exact release versions. A
  // development VSIX (for example 1.8.0-dev) must not warn against a release
  // server, and a development server must not warn against a release VSIX.
  if (!isReleaseVersion(server) || !isReleaseVersion(extension)) {
    return 'development';
  }
  return server === extension ? 'match' : 'mismatch';
}
