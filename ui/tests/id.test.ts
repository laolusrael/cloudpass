import { describe, it, expect, vi, afterEach } from 'vitest';
import { generateIdempotencyKey } from '../src/lib/utils/id';

const UUID_V4_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;
// Mirrors api/internal/handlers/jobs.go idempotencyKeyRegex.
const BACKEND_RE = /^[A-Za-z0-9_-]{1,128}$/;

describe('generateIdempotencyKey', () => {
	afterEach(() => {
		vi.unstubAllGlobals();
	});

	it('uses native randomUUID when available', () => {
		const key = generateIdempotencyKey();
		expect(key).toMatch(UUID_V4_RE);
		expect(key).toMatch(BACKEND_RE);
	});

	it('falls back to getRandomValues when randomUUID is missing (insecure HTTP)', () => {
		vi.stubGlobal('crypto', {
			getRandomValues: (arr: Uint8Array) => {
				for (let i = 0; i < arr.length; i++) {
					arr[i] = (i * 17 + 3) % 256;
				}
				return arr;
			}
		});

		const key = generateIdempotencyKey();
		expect(key).toMatch(UUID_V4_RE);
		expect(key).toMatch(BACKEND_RE);
	});

	it('falls back to Math.random when crypto is missing', () => {
		vi.stubGlobal('crypto', undefined);

		const key = generateIdempotencyKey();
		expect(key).toMatch(UUID_V4_RE);
		expect(key).toMatch(BACKEND_RE);
	});

	it('generates unique keys on the fallback path', () => {
		vi.stubGlobal('crypto', undefined);

		const keys = new Set(Array.from({ length: 100 }, () => generateIdempotencyKey()));
		expect(keys.size).toBe(100);
	});
});
