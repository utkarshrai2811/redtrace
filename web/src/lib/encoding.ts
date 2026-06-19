// Helpers for the base64-encoded raw HTTP bytes the backend sends.

export interface Decoded {
  /** False when the input was not valid base64. */
  ok: boolean;
  /** Decoded text (invalid UTF-8 bytes become U+FFFD). */
  text: string;
  /** True byte length of the decoded payload. */
  bytes: number;
}

/** Decode base64-encoded raw HTTP bytes for display, reporting the byte count. */
export function decode(b64: string): Decoded {
  try {
    const binary = atob(b64);
    const bytes = Uint8Array.from(binary, (c) => c.charCodeAt(0));
    return { ok: true, text: new TextDecoder().decode(bytes), bytes: bytes.length };
  } catch {
    return { ok: false, text: '', bytes: 0 };
  }
}

/** Decode base64 to a UTF-8 string (empty on failure). */
export function decodeBase64(b64: string): string {
  return decode(b64).text;
}

/** Encode an edited UTF-8 string back into base64 for sending to the backend. */
export function encodeBase64(text: string): string {
  const bytes = new TextEncoder().encode(text);
  let binary = '';
  for (const byte of bytes) {
    binary += String.fromCharCode(byte);
  }
  return btoa(binary);
}
