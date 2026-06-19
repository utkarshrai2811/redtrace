// Helpers for the base64-encoded raw HTTP bytes the backend sends.

/** Decode base64-encoded raw HTTP bytes into a UTF-8 string for display. */
export function decodeBase64(b64: string): string {
  try {
    const binary = atob(b64);
    const bytes = Uint8Array.from(binary, (c) => c.charCodeAt(0));
    return new TextDecoder().decode(bytes);
  } catch {
    return '';
  }
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
