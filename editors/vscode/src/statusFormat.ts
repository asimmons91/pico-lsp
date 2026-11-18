export interface CartInfo {
  rootUri?: string;
  tokens: number;
  tokenLimit: number;
  chars: number;
  charLimit: number;
  compressed?: number;
  compressedLimit?: number;
  compressedWarning?: number;
  compressedStatus?: 'ok' | 'warning' | 'error' | 'unavailable';
  files?: Array<{ uri: string; tokens: number; chars: number; bytes: number }>;
  includes?: Array<{ path: string; uri: string; tokens: number; chars: number; bytes: number }>;
}

export type CartSeverity = 'ok' | 'warning' | 'error';

export function formatCartInfo(info: CartInfo): string {
  const compressed = compressedLabel(info);
  return `PICO-8 ${info.tokens}/${info.tokenLimit} tok · ${info.chars}/${info.charLimit} chr · ${compressed}`;
}

export function cartInfoSeverity(info: CartInfo): CartSeverity {
  if (
    info.tokens > info.tokenLimit ||
    info.chars > info.charLimit ||
    info.compressedStatus === 'error'
  ) {
    return 'error';
  }
  if (
    info.tokens >= info.tokenLimit * 0.9 ||
    info.chars >= info.charLimit * 0.9 ||
    info.compressedStatus === 'warning'
  ) {
    return 'warning';
  }
  return 'ok';
}

function compressedLabel(info: CartInfo): string {
  if (info.compressedStatus === 'unavailable' || info.compressed === undefined) {
    return 'compressed n/a';
  }
  if (info.compressedLimit === undefined) {
    return `${formatKB(info.compressed)} KB`;
  }
  return `${formatKB(info.compressed)}/${formatKB(info.compressedLimit)} KB`;
}

function formatKB(bytes: number): string {
  return (bytes / 1024).toFixed(1);
}
