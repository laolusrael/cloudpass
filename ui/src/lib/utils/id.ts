function bytesToUuid(bytes: Uint8Array): string {
	const hex: string[] = [];
	for (let i = 0; i < 16; i++) {
		hex.push(bytes[i].toString(16).padStart(2, '0'));
	}
	return (
		hex.slice(0, 4).join('') +
		'-' +
		hex.slice(4, 6).join('') +
		'-' +
		hex.slice(6, 8).join('') +
		'-' +
		hex.slice(8, 10).join('') +
		'-' +
		hex.slice(10, 16).join('')
	);
}

/**
 * Generate a v4-shaped idempotency key.
 *
 * `crypto.randomUUID()` only exists in Secure Contexts (HTTPS, localhost).
 * Over plain `http://<lan-ip>` it is `undefined` and throws
 * "crypto.randomUUID is not a function". Fall back to `getRandomValues`,
 * then to `Math.random`, so async job actions never hard-crash.
 * Output always matches /^[0-9a-f-]{36}$/ (backend: [A-Za-z0-9_-]{1,128}).
 */
export function generateIdempotencyKey(): string {
	try {
		const c = globalThis.crypto as Crypto | undefined;
		if (c?.randomUUID) {
			return c.randomUUID();
		}
		if (c?.getRandomValues) {
			const bytes = c.getRandomValues(new Uint8Array(16));
			bytes[6] = (bytes[6] & 0x0f) | 0x40;
			bytes[8] = (bytes[8] & 0x3f) | 0x80;
			return bytesToUuid(bytes);
		}
	} catch {
		// Fall through to Math.random below.
	}

	const bytes = new Uint8Array(16);
	for (let i = 0; i < 16; i++) {
		bytes[i] = Math.floor(Math.random() * 256);
	}
	bytes[6] = (bytes[6] & 0x0f) | 0x40;
	bytes[8] = (bytes[8] & 0x3f) | 0x80;
	return bytesToUuid(bytes);
}
